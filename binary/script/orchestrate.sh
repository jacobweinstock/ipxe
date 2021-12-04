#!/bin/bash

set -oxe pipefail

# check for changes to iPXE files
function changes_detected() {
    local file="$1"

    result=$(sha512sum -c "${file}")
    if [ $? -eq 0 ]; then
        # No changes detected
        exit 0
    else
        # Changes detected
        echo 1
    fi
}

# remove old iPXE files
function clean_iPXE() {
    # remove existing iPXE binaries
    echo "Removing existing iPXE binaries"
    make binary/clean
    if [ $? -ne 0 ]; then
        echo "Failed to remove iPXE binaries" 1>&2
        exit 1
    fi
}

# build iPXE binaries
function build_iPXE() {
    # build iPXE
    echo "Building iPXE"
    make binary
    if [ $? -ne 0 ]; then
        echo "Failed to build iPXE" 1>&2
        exit 1
    fi
}

# update checksums file
function create_checksums() {
    local location="$1"

    find . -type f \( -name '*.h' \
    -o -name 'snp.efi' \
    -o -name 'ipxe.efi' \
    -o -name 'undionly.kpxe' \
    -o -name 'embed.ipxe' \
    -o -name 'ipxe.commit' \
    \) -exec sha512sum {} + > "${location}"
}

# create a new branch
function create_branch() {
    local branch="$1"

    # create a new branch
    git checkout -b "${branch}"
    if [ $? -ne 0 ]; then
        echo "Failed to create branch ${branch}" 1>&2
        exit 1
    fi
}

# commit changes to git
function commit_changes() {
    local files="$1"
    local message="$2"

    # commit changes
    echo "Committing changes"
    git add "${file}"
    git commit -sm "Update iPXE"
    if [ $? -ne 0 ]; then
        echo "Failed to commit changes" 1>&2
        exit 1
    fi
}

# push changes to origin
function push_changes() {
    local branch="$1"

    # push changes
    echo "Pushing changes"
    git push origin "${branch}"
    if [ $? -ne 0 ]; then
        echo "Failed to push changes" 1>&2
        exit 1
    fi
}

# create Github Pull Request
function create_pull_request() {
    local branch="$1"
    local message="$2"

    # create pull request
    echo "Creating pull request"
    
    if [ $? -ne 0 ]; then
        echo "Failed to create pull request" 1>&2
        exit 1
    fi
}

function main() {
    local sha_file="$1"
    
    changes=$(changes_detected "${sha_file}")
    if [ $? -eq 0 ]; then
        echo "No changes detected"
        exit 0
    fi
    create_branch "update_iPXE_$(date +"%Y_%m_%d_%H_%M_%S")"
    clean_iPXE
    build_iPXE
    create_checksums "${sha_file}"
    commit_changes "${sha_file},snp.efi,ipxe.efi,undionly.kpxe" "Update iPXE binaries"
    push_changes "main"
    create_pull_request "main" "Update iPXE binaries"
}

main