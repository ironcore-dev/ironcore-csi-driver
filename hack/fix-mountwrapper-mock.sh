#!/usr/bin/env bash
set -euo pipefail

FILE="pkg/utils/mount/mock_mountutils_unix.go"

# Fixes "MountWrapper Type cannot implement 'MountWrapper' as it has a non-exported method and is defined in a different package"
# See https://github.com/kubernetes/mount-utils/commit/a20fcfb15a701977d086330b47b7efad51eb608e for context.

# Standard POSIX stream editing (works perfectly on ALL sed versions)
sed '/type MockMountWrapper struct {/a \
	mount.Interface
' "$FILE" > "$FILE.tmp" && mv "$FILE.tmp" "$FILE"

