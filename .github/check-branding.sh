#!/usr/bin/env bash

set -euo pipefail

forbidden_content='ENTRYPOINT \["sing-box"\]|Use:[[:space:]]+"sing-box"|"sing-box version |/usr/(local/)?bin/sing-box|/etc/sing-box|/var/lib/sing-box|sing-box\.sagernet\.org|maintainer[:=]"?nekohasekai|--name sing-box|name:sing-box|origin:sing-box|dist/sing-box|name="sing-box-'
search_paths=(
  Dockerfile
  Dockerfile.binary
  .fpm_openwrt
  .fpm_pacman
  .fpm_systemd
  .github
  cmd/sing-box
  release
)

if matches=$(git grep -n -E "$forbidden_content" -- "${search_paths[@]}" ':(exclude).github/check-branding.sh'); then
  echo "Forbidden user-facing upstream branding remains:" >&2
  echo "$matches" >&2
  exit 1
fi

forbidden_files='(^|/)(sing-box(\.service|@\.service|\.initd|\.confd|\.postinst|\.rules|\.sysusers|-split-dns\.xml)|sing-box\.(bash|fish|zsh))$'
if matches=$(git ls-files | grep -E "$forbidden_files"); then
  echo "Forbidden user-facing upstream-branded files remain:" >&2
  echo "$matches" >&2
  exit 1
fi

echo "xhttp-box branding checks passed."
