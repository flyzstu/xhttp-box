#!/usr/bin/env bash

set -e -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/common.sh"

echo "Uninstalling xhttp-box..."

if systemctl is-active --quiet xhttp-box 2>/dev/null; then
    echo "Stopping xhttp-box service..."
    sudo systemctl stop xhttp-box
fi

if systemctl is-enabled --quiet xhttp-box 2>/dev/null; then
    echo "Disabling xhttp-box service..."
    sudo systemctl disable xhttp-box
fi

echo "Removing files..."
sudo rm -rf "$INSTALL_DATA_PATH"
sudo rm -rf "$INSTALL_BIN_PATH/$BINARY_NAME"
sudo rm -rf "$INSTALL_CONFIG_PATH"
sudo rm -rf "$SYSTEMD_SERVICE_PATH/xhttp-box.service"

echo "Reloading systemd..."
sudo systemctl daemon-reload

echo ""
echo "Uninstallation complete!"
