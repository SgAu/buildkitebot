#!/usr/bin/env bash

# shellcheck source=.buildkite/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

queue='pavedroad-prod:cicd'

cat << EOF
steps:
- label: ":shell: Shellcheck"
  command: make shellcheck
  plugins:
  - docker-compose#v2.6.0:
      run: code-quality
  agents:
    queue: "${queue}"

- label: ":sparkles: Lint"
  command: make lint
  plugins:
  - docker-compose#v2.6.0:
      run: code-quality
  agents:
    queue: "${queue}"

- label: ":golang: Unit tests"
  command: make test
  plugins:
  - docker-compose#v2.6.0:
      run: base
  agents:
    queue: "${queue}"

- label: ":mag: Snyk"
  command: make snyk
  plugins:
  - seek-oss/aws-sm#v0.0.5:
      env:
        SNYK_TOKEN: /snyk/keys/service-accounts/another-paved-road
  - docker-compose#v2.6.0:
      run: "snyk"
  agents:
    queue: ${queue}

- wait
- label: ":docker: Build and upload Orgbot"
  agents:
    queue: "${queue}"
  branches: master
  plugins:
  - seek-jobs/gantry#v1.0.6: # Publish image used as Gantry service
      command: build
      config: templates/resources.yaml
      values:
      - templates/values.yaml
      working-dir: .
  - seek-jobs/gantry#v1.0.6: # Publish image used as for CLI
      command: build
      config: templates/resources.yaml
      values:
      - templates/values.yaml
      - templates/values-latest.yaml
      working-dir: .

- wait
- label: ":rocket: Deploy production"
  agents:
    queue: "${queue}"
  branches: master
  concurrency: 1
  concurrency_group: orgbot/deploy
  plugins:
  - seek-jobs/gantry#v1.0.6:
      command: apply
      config: templates/resources.yaml
      values:
      - templates/values.yaml
      working-dir: .
      environment: paved-road-prod
EOF
