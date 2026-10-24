#!/usr/bin/env bash
#MISE description="Lint the OpenAPI description of the Inoreader API"
set -euo pipefail

function main() {
  cd "$(git rev-parse --show-toplevel)"
  vacuum lint --ruleset api/ruleset.yaml --fail-severity warn --details --no-banner api/openapi.yaml
}

main "$@"
