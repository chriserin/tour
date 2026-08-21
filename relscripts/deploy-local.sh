#!/bin/bash
set -e

VERSION_FILE=cmd/version.go
INSTALL_PATH=/usr/local/bin/tour

if [ "${1}" = "dev" ]; then
    VERSION="dev-$(date +%Y%m%d%H%M%S)"
    echo "Building tour ${VERSION} (local dev build)..."

    # Temporarily patch the version const for this build only, then
    # restore the file no matter how the script exits — this is a
    # local, throwaway build, not a release, so nothing here should
    # end up committed. (cmd/version.go's `version` is a hardcoded
    # const, not a var, so this can't be done via -ldflags -X; see
    # relscripts/build.sh and implementation/CLI.md's Version
    # reporting section.)
    #
    # -i.bak, not bare -i: GNU sed accepts -i with no argument, but
    # BSD/macOS sed requires an explicit (possibly empty) suffix —
    # -i.bak works identically on both, and doubles as our own backup.
    trap '[ -f "${VERSION_FILE}.bak" ] && mv "${VERSION_FILE}.bak" "$VERSION_FILE"' EXIT
    sed -i.bak "s/const version = \".*\"/const version = \"${VERSION}\"/" "$VERSION_FILE"
    go build -o tour .
else
    echo "Building tour..."
    go build -o tour .
fi

echo "Installing to ${INSTALL_PATH}..."
# Requires write access to /usr/local/bin — rerun with sudo if this fails.
cp tour "$INSTALL_PATH"
rm tour

echo "✓ Installed $("$INSTALL_PATH" version)"
