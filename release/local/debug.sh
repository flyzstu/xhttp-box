#!/usr/bin/env bash

set -e -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/common.sh"

setup_environment

echo "Updating xhttp-box from git repository..."
cd "$PROJECT_DIR"
git fetch
git reset FETCH_HEAD --hard
git clean -fdx

BUILD_TAGS=$(get_build_tags "debug")

build_xhttp_box "$BUILD_TAGS"

stop_service
install_binary
start_service

echo ""
echo "Following service logs (Ctrl+C to exit)..."
sudo journalctl -u xhttp-box --output cat -f
