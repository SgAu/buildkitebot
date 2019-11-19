#!/usr/bin/env bash

# shellcheck source=.buildkite/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

cat << EOF
steps:
- label: ":mag: Check for changes"
  command: .buildkite/steps/diff.sh
  agents:
    queue: pavedroad-prod:org
EOF
