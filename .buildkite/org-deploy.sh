#!/usr/bin/env bash

# shellcheck source=.buildkite/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

if [[ "${BUILDKITE_TRIGGERED_FROM_BUILD_PIPELINE_SLUG:-}" != 'org' ]]; then
  cat << EOF
steps:
- label: ":no_entry_sign: This pipeline should not be invoked directly"
  command: exit 1
EOF
  exit
fi

label=':rocket: Apply org changes'
if [[ "${DEPLOY_BRANCH}" != 'master' ]]; then
  label=':microscope: Test org changes'
fi

cat << EOF
steps:
- label: "${label}"
  command: .buildkite/steps/apply.sh "${DEPLOY_BRANCH}" "${DEPLOY_COMMIT}"
  agents:
    queue: pavedroad-prod:org
EOF
