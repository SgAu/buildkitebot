#!/usr/bin/env bash
# shellcheck disable=SC2034

# Provides common variables and functionality required by scripts in .buildkite directory

set -eou pipefail

#
# Run yq inside a Docker container
#
yq() {
  docker run -v "${PWD}:/workdir" mikefarah/yq yq "$@"
}

#
# Whether the current branch is master
#
is_master() {
  [[ "${branch}" == 'master' ]]
}

# Ensure that the CWD of all scripts is the repo root
root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
pushd "${root_dir}" > /dev/null

# Common variables
build_dir='target'
version="$(cat VERSION)"
commit_sha="$(cut -c1-7 <<< "${BUILDKITE_COMMIT:-$(git rev-parse HEAD)}")"
build_number="${BUILDKITE_BUILD_NUMBER:-0}"
build_prefix="orgbot/${version}+${build_number}.sha.${commit_sha}"
branch="${BUILDKITE_BRANCH:-"$(git rev-parse --abbrev-ref HEAD)"}"
org_repo_uri='git@github.com:SEEK-Jobs/org.git'
region='ap-southeast-2'
ecr_account_id='849781104100'
ecr_registry="${ecr_account_id}.dkr.ecr.${region}.amazonaws.com"
build_bucket="$(yq r "templates/values.yaml" buildBucket)"
org_bucket="$(yq r "templates/values.yaml" orgBucket)"

# Ensure build directory exists
mkdir -p "${build_dir}"

#
# Login to ECR
#
docker_login() {
  aws ecr get-authorization-token \
    --registry-ids "${ecr_account_id}" \
    --region "${region}" \
    --query 'authorizationData[].authorizationToken' \
    --output text \
    | base64 --decode \
    | cut -d: -f2 \
    | docker login -u AWS --password-stdin "https://${ecr_registry}" >&2
}

#
# Run orgctl as a Docker container, ensuring the latest image is run
#
orgctl() {
  docker_login
  docker rmi -f "${ecr_registry}/orgbot:latest" >&2 2> /dev/null
  docker run \
    -v "${HOME}/.aws:/.aws" \
    -v "${PWD}:/src" \
    -w /src \
    "${ecr_registry}/orgbot:latest" orgctl "${@}"
}
