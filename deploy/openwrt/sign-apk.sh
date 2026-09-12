#!/bin/sh
# sign-apk.sh - Builds the OpenWrt 25 .apk feed index and signs it.
#
# Usage:
#   sign-apk.sh [output-dir] [usign-secret-key] [public-key]
#
# Requires: apk-tools 3 built for OpenWrt (the SDK host build; the Alpine
# apk-tools speaks a different package format) and usign, both under
# staging_dir/host/bin of an extracted OpenWrt SDK.
#
# Produces in output-dir:
#   packages.adb      - repository index (v3), the file repositories.d
#                       entries must point at
#   packages.adb.asc  - usign detached signature of the index
#   <public-key copy> - the matching public key, for /etc/apk/keys
#
# The release apks are signed by the SDK build keys, which this run does
# not trust; the index is built with --allow-untrusted and the trust lives
# in the usign signature of packages.adb.
#
# NOTE (#296): OpenWrt 25.12 routers currently only verify indexes signed
# with the official OpenWrt build-system PGP key, so third-party adb feeds
# are not verifiable on-device yet. The structure published here is the
# correct feed layout; routers can still install the apks directly with
# `apk add --allow-untrusted`, which is what netgrip-heal-register does.

set -eu

out_dir="${1:-feed}"
privkey="${2:?usage: sign-apk.sh <output-dir> <usign-secret-key> <public-key>}"
pubkey="${3:?usage: sign-apk.sh <output-dir> <usign-secret-key> <public-key>}"

if [ ! -d "$out_dir" ]; then
  echo "Error: output directory not found: $out_dir" >&2
  exit 1
fi

if [ ! -f "$privkey" ]; then
  echo "Error: usign secret key not found: $privkey" >&2
  exit 1
fi

if [ ! -f "$pubkey" ]; then
  echo "Error: public key not found: $pubkey" >&2
  exit 1
fi

# Resolve inputs to absolute paths before changing directory.
out_dir="$(cd "$(dirname "$out_dir")" && pwd)/$(basename "$out_dir")"
pubkey="$(cd "$(dirname "$pubkey")" && pwd)/$(basename "$pubkey")"

cd "$out_dir"

echo "==> Generating packages.adb"
# Fail closed: a feed without a signed index is a broken feed. This used
# to warn and exit 0, which published apk files nobody could install (#296).
apk mkndx --allow-untrusted -o packages.adb *.apk

echo "==> Signing packages.adb"
usign -S -s "$privkey" -m packages.adb -x packages.adb.asc

echo "==> Publishing public key"
cp "$pubkey" .

echo "==> Done"
echo "Index: $out_dir/packages.adb (signed: packages.adb.asc)"
