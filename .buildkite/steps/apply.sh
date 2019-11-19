#!/usr/bin/env bash

# shellcheck source=.buildkite/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../lib/common.sh"

usage() {
  cat << EOF
Usage: $(basename "${0}") BRANCH COMMIT_SHA

Executes the appropriate deployment process for the specified branch and commit
of the org repository. If the commit is for the master branch then we use Orgbot
to apply the changes, otherwise we use Orgbot to perform a dry-run.

EOF
  exit 1
}

(($# != 2)) && usage

org_branch="${1}"
org_commit="${2}"

# Clone the org repository and checkout the specified commit
git clone "${org_repo_uri}" "${build_dir}"/org
cd "${build_dir}"/org
git checkout "${org_commit}"

# Perform a dry-run if we're not applying master
args=()
if [[ "${org_branch}" != 'master' ]]; then
  args+=('--dry-run')
fi

# Run the Orgbot against each org directory
for org_dir in config/*; do
  # Variable expansion expression below is a workaround for < Bash 4.4
  # where an empty array is treated as an unset variable
  orgctl org apply --dir "${org_dir}" "${args[@]+"${args[@]}"}"

  # If we just successfully applied master, upload the merged org to S3 for external consumption
  if [[ "${org_branch}" == 'master' ]]; then
    org_name="$(basename "${org_dir}")"
    orgctl org merge --dir "${org_dir}" --format=json \
      | aws s3 cp - "s3://${org_bucket}/${org_name}/org.json"
  fi
done
