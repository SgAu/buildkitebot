#!/usr/bin/env bash

# shellcheck source=.buildkite/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../lib/common.sh"

# Directory that we'll clone the org repository into
org_repo_dir="${build_dir}/org"
rm -rf "${org_repo_dir}"

# Clone the org repository
git clone "${org_repo_uri}" "${org_repo_dir}"

diff_found='false'

# Use Orgbot to perform a diff against each org directory
for org_dir in "${org_repo_dir}"/config/*; do
  org_name="$(yq r "${org_dir}/org.yaml" 'name')"
  echo >&2
  echo >&2 "Performing dry-run apply on ${org_name} to check for differences"
  result="$(orgctl org apply --dir="${org_dir}" --dry-run --format=json)"
  echo >&2 "Result: ${result}"

  # Check the JSON result body for any changes made
  for delta in $(jq -r '. | values[]' <<< "${result}"); do
    if ((delta != 0)); then
      diff_found=true
    fi
  done
done

# Exit with an error if differences were found
if [[ "${diff_found}" == 'true' ]]; then
  echo >&2
  echo >&2 "Differences found. See above for details."
  exit 1
fi
