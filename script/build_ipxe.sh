#!/usr/bin/env bash

set -eux

function download_ipxe_repo() {
    local sha_or_tag="$1"
    if [ ! -f "ipxe-${sha_or_tag}.tar.gz" ]; then
        echo "downloading"
        curl -fLo ipxe-"${sha_or_tag}".tar.gz https://github.com/ipxe/ipxe/archive/${sha_or_tag}.tar.gz
    else
        echo "already downloaded"
    fi
    
}

function extract_ipxe_repo() {
    local archive_name="$1"
    local archive_dir="$2"

    if [ ! -d "$archive_dir" ]; then
        echo "extracting"
        mkdir -p "${archive_dir}"
        tar -xzf "${archive_name}" -C "${archive_dir}" --strip-components 1
    else
        echo "already extracted"
    fi
}

function build_ipxe() {
    local ipxe_dir="$1"
    local ipxe_bin="$2"
    local run_in_docker="$3"
    local env_opts="$4"
    local embed_path="$5"

    if [ "${run_in_docker}" = true ]; then
        if [ ! -f "${ipxe_dir}/src/${ipxe_bin}" ]; then
            echo "running in docker"
            docker run -it --rm -v ${PWD}:/code -w /code nixos/nix nix-shell script/shell.nix --run "${env_opts} make -C ${ipxe_dir}/src EMBED=${embed_path} ${ipxe_bin}"
        fi
    else
        echo "running locally"
        nix-shell script/shell.nix --run "${env_opts} make -C ${ipxe_dir}/src EMBED=${embed_path} ${ipxe_bin}"
    fi
}

function mv_embed_into_build() {
    local embed_path="$1"
    local ipxe_dir="$2"

    cp -a "${embed_path}" "${ipxe_dir}"/src/embed.ipxe
}

function make_local_empty() {
    local ipxe_dir="$1" 

    rm -rf "${ipxe_dir}"/src/config/local/*
}

function copy_common_files() {
    local ipxe_dir="$1" 
    cp -a script/ipxe-customizations/colour.h "${ipxe_dir}"/src/config/local/
    cp -a script/ipxe-customizations/common.h "${ipxe_dir}"/src/config/local/
    cp -a script/ipxe-customizations/console.h "${ipxe_dir}"/src/config/local/
}

function copy_custom_files() {
    local ipxe_dir="$1"
    local ipxe_bin="$2"

    case "${ipxe_bin}" in
    bin/undionly.kpxe)
    	cp script/ipxe-customizations/general.undionly.h "${ipxe_dir}"/src/config/local/general.h
    	;;
    bin/ipxe.lkrn)
    	cp script/ipxe-customizations/general.undionly.h "${ipxe_dir}"/src/config/local/general.h
    	;;
    bin-x86_64-efi/ipxe.efi)
    	cp script/ipxe-customizations/general.efi.h "${ipxe_dir}"/src/config/local/general.h
        cp script/ipxe-customizations/isa.h "${ipxe_dir}"/src/config/local/isa.h
    	;;
    bin-arm64-efi/snp.efi)
    	cp script/ipxe-customizations/general.efi.h "${ipxe_dir}"/src/config/local/general.h
    	;;
    *) echo "unknown binary: ${ipxe_bin}" >&2 && exit 1 ;;
    esac
}

function customize_aarch_build() {
    local ipxe_dir="$1"
    # http://lists.ipxe.org/pipermail/ipxe-devel/2018-August/006254.html
    sed -i.bak '/^WORKAROUND_CFLAGS/ s|^|#|' "${ipxe_dir}"/src/arch/arm64/Makefile
}

function customize() {
    local ipxe_dir="$1"
    local ipxe_bin="$2"

    make_local_empty "${ipxe_dir}"
    copy_common_files "${ipxe_dir}"
    copy_custom_files "${ipxe_dir}" "${ipxe_bin}"
    customize_aarch_build "${ipxe_dir}"
}

function main() {
    local bin_path="$(echo $1 | xargs)"
    local ipxe_sha_or_tag="$(echo $2 | xargs)"
    local ipxe_build_in_docker="$(echo $3 | xargs)"
    local final_path="$(echo $4 | xargs)"
    local env_opts="$(echo $5 | xargs)"
    local embed_path="$(echo $6 | xargs)"

    download_ipxe_repo "${ipxe_sha_or_tag}"
    extract_ipxe_repo "ipxe-${ipxe_sha_or_tag}.tar.gz" "upstream-${ipxe_sha_or_tag}"
    mv_embed_into_build "${embed_path}" "upstream-${ipxe_sha_or_tag}"
    customize "upstream-${ipxe_sha_or_tag}" "${bin_path}"
    build_ipxe "upstream-${ipxe_sha_or_tag}" "${bin_path}" "${ipxe_build_in_docker}" "${env_opts}" "embed.ipxe"
    cp -a "upstream-${ipxe_sha_or_tag}/src/${bin_path}" "${final_path}"
}

main "$1" "$2" "$3" "$4" "${5:-''}" "${6:-script/embed.ipxe}" 
