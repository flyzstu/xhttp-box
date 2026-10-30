#!/usr/bin/env bash

set -e -o pipefail

sudo systemctl enable xhttp-box
sudo systemctl start xhttp-box
sudo journalctl -u xhttp-box --output cat -f
