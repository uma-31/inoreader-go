#!/usr/bin/env bash
#MISE description="Get a refresh token for the live tests and write it to .env"
#USAGE arg "[args]" var=#true help="Flags for the command, such as --port; see -h"
set -euo pipefail

function main() {
  cd "$(git rev-parse --show-toplevel)"
  go run ./live/authorize "$@"
}

main "$@"
