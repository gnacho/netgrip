#!/bin/sh
# sign-apk.sh - Signs an OpenWrt .apk package and generates a signed APKINDEX.
#
# Usage:
#   ./deploy/openwrt/sign-apk.sh <apk-file> <private-key> [output-dir]
#
# Requires: apk-tools (3.x), abuild (for abuild-sign)
#
# This script:
# 1. Unpacks the .apk (PKGINFO + control.tar.gz + data.tar.gz)
# 2. Signs control.tar.gz with the private key
# 3. Repacks the signed .apk
# 4. Generates APKINDEX.tar.gz from all .apks in output-dir
# 5. Signs APKINDEX.tar.gz with the private key

set -eu

apk_file="$1"
privkey="$2"
out_dir="${3:-$(dirname "$apk_file")}"

if [ -z "$apk_file" ] || [ -z "$privkey" ]; then
  echo "Usage: $0 <apk-file> <private-key> [output-dir]" >&2
  exit 1
fi

if [ ! -f "$apk_file" ]; then
  echo "Error: APK file not found: $apk_file" >&2
  exit 1
fi

if [ ! -f "$privkey" ]; then
  echo "Error: Private key not found: $privkey" >&2
  exit 1
fi

mkdir -p "$out_dir"
tmp_dir=$(mktemp -d)
trap "rm -rf $tmp_dir" EXIT

echo "==> Unpacking $apk_file"
cd "$tmp_dir"

# Extract the three parts of the .apk
# .PKGINFO is a plain file at the start
# control.tar.gz and data.tar.gz follow
tar xf "$apk_file" 2>/dev/null || {
  echo "Warning: tar extraction failed, trying abuild-tar" >&2
  abuild-tar -xf "$apk_file" 2>/dev/null || {
    echo "Error: Cannot unpack APK" >&2
    exit 1
  }
}

echo "==> Signing control.tar.gz"
if [ -f control.tar.gz ]; then
  abuild-sign -k "$privkey" control.tar.gz
elif [ -f control.tar ]; then
  gzip control.tar
  abuild-sign -k "$privkey" control.tar.gz
else
  echo "Error: No control.tar.gz found in APK" >&2
  exit 1
fi

echo "==> Repacking signed APK"
signed_name="$(basename "$apk_file")"
tar cf "$out_dir/$signed_name" .PKGINFO control.tar.gz data.tar.gz 2>/dev/null || \
  tar cf "$out_dir/$signed_name" .PKGINFO control.tar.gz data.tar.gz

echo "==> Generating APKINDEX"
cd "$out_dir"
apk index -o APKINDEX.tar.gz --description "NetGrip $(date +%Y-%m-%d)" *.apk
abuild-sign -k "$privkey" APKINDEX.tar.gz

echo "==> Done"
echo "Signed APK: $out_dir/$signed_name"
echo "Signed index: $out_dir/APKINDEX.tar.gz"
