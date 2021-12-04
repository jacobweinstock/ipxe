#!/bin/bash

set -oxe pipefail

# check for the GITHUB_TOKEN environment variable
function check_github_token() {
  if [ -z "${GITHUB_TOKEN}" ]; then
    echo "GITHUB_TOKEN is not set"
    exit 1
  fi
}

# check for changes to iPXE files
function changes_detected() {
    local file="${1:-sha512sum.txt}"

    result=$(sha512sum -c "${file}")
    if [ $? -eq 0 ]; then
        # No changes detected
        echo 0
    else
        # Changes detected
        echo 1
    fi
}

# remove old iPXE files
function clean_iPXE() {
    # remove existing iPXE binaries
    echo "Removing existing iPXE binaries"
    (cd $(git rev-parse --show-toplevel); make binary/clean)
    if [ $? -ne 0 ]; then
        echo "Failed to remove iPXE binaries" 1>&2
        exit 1
    fi
}

# build iPXE binaries
function build_iPXE() {
    # build iPXE
    echo "Building iPXE"
    (cd $(git rev-parse --show-toplevel); make binary)
    if [ $? -ne 0 ]; then
        echo "Failed to build iPXE" 1>&2
        exit 1
    fi
}

# update checksums file
function create_checksums() {
    local location="${1:-sha512sum.txt}"

    find . -type f \( -name '*.h' \
    -o -name 'snp.efi' \
    -o -name 'ipxe.efi' \
    -o -name 'undionly.kpxe' \
    -o -name 'embed.ipxe' \
    -o -name 'ipxe.commit' \
    \) -exec sha512sum {} + > "${location}"
}

# configure git client
function configure_git() {
    local email="${1:-github-actions[bot]@users.noreply.github.com}"
    local name="${2:-github-actions[bot]}"

    git config --local user.email "${email}"
    git config --local user.name "${name}"
}

# create a new branch
function create_branch() {
    local branch="${1:-update_iPXE_$(date +"%Y_%m_%d_%H_%M_%S")}"

    # create a new branch
    git checkout -b "${branch}"
    if [ $? -ne 0 ]; then
        echo "Failed to create branch ${branch}" 1>&2
        exit 1
    fi
    push_changes "${branch}"
    if [ $? -ne 0 ]; then
        echo "Failed to push branch ${branch}" 1>&2
        exit 1
    fi
}

# commit changes to git
function commit_changes() {
    local files="${1:-script/sha512sum.txt snp.efi ipxe.efi undionly.kpxe}"
    local message="${2:-Update iPXE}"

    # commit changes
    echo "Committing changes"
    git add ${files}
    git commit -sm "${message}"
    if [ $? -ne 0 ]; then
        echo "Failed to commit changes" 1>&2
        exit 1
    fi
}

# push changes to origin
function push_changes() {
    local branch="${1}"

    REPOSITORY="jacobweinstock/ipxe"
    # push changes
    echo "Pushing changes"
    git push https://${GITHUB_ACTOR}:${GITHUB_TOKEN}@github.com/${REPOSITORY}.git HEAD:"${branch}"
    if [ $? -ne 0 ]; then
        echo "Failed to push changes" 1>&2
        exit 1
    fi
}

# create Github Pull Request
function create_pull_request() {
    local branch="$1"
    local message="${1:-Update iPXE}"

    # create pull request
    echo "Creating pull request"
    $(git rev-parse --show-toplevel)/binary/script/gh pr create --base main --body "updating iPXE binaries" --title "update iPXE binaries" --head "${branch}"
    if [ $? -ne 0 ]; then
        echo "Failed to create pull request" 1>&2
        exit 1
    fi
}

# clean_up undoes any changes made by the script
function clean_up() {
    git config --local --unset user.email
    git config --local --unset user.name
}

function main() {
    local sha_file="$1"

    check_github_token
    changes=$(changes_detected "${sha_file}")
    if [ ${changes} == "0" ]; then
        echo "No changes detected"
        exit 0
    fi
    branch="update_iPXE_$(date +"%Y_%m_%d_%H_%M_%S")"
    create_branch "update_iPXE_$(date +"%Y_%m_%d_%H_%M_%S")"
    clean_iPXE
    build_iPXE
    create_checksums "${sha_file}"
    configure_git
    commit_changes "script/sha512sum.txt snp.efi ipxe.efi undionly.kpxe" "Update iPXE binaries"
    push_changes "${branch}"
    create_pull_request "${branch}" "Update iPXE binaries"
    clean_up
}

main "${1:-./script/sha512sum.txt}"
