# shellcheck shell=bash

# The modules of the repository: the client, and the E2E tests with the mock
# server, which keep their dependencies out of the client's go.mod.
function ls_modules() {
  echo .
  echo e2e
}
