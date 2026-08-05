#!/usr/bin/env bash
set -euo pipefail

FILE="pkg/utils/mount/mock_mountutils_unix.go"

# Fixes "MountWrapper Type cannot implement 'MountWrapper' as it has a non-exported method and is defined in a different package"
# See https://github.com/kubernetes/mount-utils/commit/a20fcfb15a701977d086330b47b7efad51eb608e for context.

if [[ "$(uname)" == "Darwin" ]]; then
  # macOS sed requires an empty string ('') right after -i.
  sed -i '' '/type MockMountWrapper struct {/a\
	mount.Interface
' "$FILE"
else
  # Linux sed works with the -i flag as is.
  sed -i '/type MockMountWrapper struct {/a \\tmount.Interface' "$FILE"
fi