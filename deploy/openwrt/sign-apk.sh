#!/bin/sh
# sign-apk.sh - Generates and signs a signed APKINDEX for OpenWrt .apk feeds.
#
# Usage:
#   ./deploy/openwrt/sign-apk.sh [output-dir] [private-key]
#
# Requires: apk-tools (3.x), abuild-sign
#
# For apk v3 (OpenWrt 25.12+) packages are pre-built by the SDK and do not need
# per-package repacking/signature. The feed only needs a signed APKINDEX.tar.gz.
# This script:
#   1. Generates APKINDEX.tar.gz from all *.apk in output-dir
#   2. Signs APKINDEX.tar.gz with the private key
#   3. Extracts the matching public key into output-dir

set -eu

out_dir="${1:-feed}"
privkey="${2:-~/.abuild/netgrip-signing-key.rsa}"

if [ ! -d "$out_dir" ]; then
  echo "Error: output directory not found: $out_dir" >&2
  exit 1
fi

if [ ! -f "$privkey" ]; then
  echo "Error: Private key not found: $privkey" >&2
  exit 1
fi

cd "$out_dir"

echo "==> Extracting public key"
openssl rsa -in "$privkey" -pubout -out netgrip.rsa.pub 2>/dev/null

echo "==> Generating APKINDEX"
# The release apks are signed by the build SDK keys, which this container
# does not trust; index with --allow-untrusted and let the trust live in
# the APKINDEX signature below. Fail closed: a feed without a signed index
# is a broken feed. This used to warn and exit 0, which published apk
# files nobody could install (#296).
apk index --allow-untrusted -o APKINDEX.tar.gz --description "NetGrip $(date +%Y-%m-%d)" *.apk

echo "==> Signing APKINDEX"
abuild-sign -k "$privkey" APKINDEX.tar.gz

echo "==> Done"
echo "Signed index: $out_dir/APKINDEX.tar.gz"
