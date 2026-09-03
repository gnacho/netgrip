#!/bin/bash
# package.sh builds the netgrip OpenWrt package as .ipk or .apk using the
# OpenWrt SDK. Adapted from the proven NetPulse openwrt packaging.
#
# Usage:
#   package.sh <SDK_VERSION> <SDK_TARGET> <SDK_SUBTARGET> <FORMAT> <BINARY_PATH>
#
#   SDK_VERSION:   24.10.5 or 25.12.5
#   SDK_TARGET:    mediatek/filogic or qualcommax/ipq807x
#   SDK_SUBTARGET: (empty, kept for CLI compatibility)
#   FORMAT:        ipk or apk
#   BINARY_PATH:   path to the prebuilt netgrip binary (arm64)
#
# PKG_VERSION / PKG_RELEASE can be overridden through env vars (CI uses the
# release tag). PKG_ARCH sets the ipk architecture field (defaults to the
# aarch64_cortex-a53 of the qualcommax/mediatek targets; x86/64 builds must
# pass x86_64 or the ipk gets misnamed and clobbers the arm64 one, #214).
#
# Example:
#   package.sh 25.12.5 qualcommax/ipq807x "" apk ./netgrip-arm64

set -euo pipefail

SDK_VERSION="${1:?missing SDK_VERSION}"
SDK_TARGET="${2:?missing SDK_TARGET}"
SDK_SUBTARGET="${3:-}"
FORMAT="${4:?missing FORMAT (ipk|apk)}"
BINARY="${5:?missing BINARY_PATH}"

if [ ! -f "$BINARY" ]; then
  echo "ERROR: binary not found: $BINARY" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

PKG_VERSION="${PKG_VERSION:-$(sed -n 's/^PKG_VERSION:=//p' "$SCRIPT_DIR/netgrip/Makefile" | head -1)}"
PKG_RELEASE="${PKG_RELEASE:-$(sed -n 's/^PKG_RELEASE:=//p' "$SCRIPT_DIR/netgrip/Makefile" | head -1)}"
PKG_ARCH="${PKG_ARCH:-aarch64_cortex-a53}"

echo "=== package.sh: $FORMAT SDK=$SDK_VERSION target=$SDK_TARGET arch=$PKG_ARCH version=${PKG_VERSION:-0.0.0}-${PKG_RELEASE:-1} ==="

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

# The toolchain name in the SDK archive differs across releases; resolve the
# real name from the download index instead of hardcoding it.
SDK_DIR_URL="https://downloads.openwrt.org/releases/${SDK_VERSION}/targets/${SDK_TARGET}/"
SDK_NAME="$(curl -fsSL "$SDK_DIR_URL" 2>/dev/null | grep -o 'openwrt-sdk-[^"]*\.tar\.zst' | head -1 || true)"
if [ -z "$SDK_NAME" ]; then
  echo "ERROR: SDK not found under ${SDK_DIR_URL}" >&2
  exit 1
fi
SDK_URL="${SDK_DIR_URL}${SDK_NAME}"
echo "  SDK: $SDK_URL"

cd "$WORK_DIR"
curl -fsSL "$SDK_URL" -o sdk.tar.zst
tar --zstd -xf sdk.tar.zst
SDK_DIR="$WORK_DIR/${SDK_NAME%.tar.zst}"

# Stage the binary and package files
PKG_DIR="$WORK_DIR/netgrip-pkg"
mkdir -p "$PKG_DIR/CONTROL" "$PKG_DIR/usr/sbin" "$PKG_DIR/etc/init.d" "$PKG_DIR/usr/libexec"
cp "$BINARY" "$PKG_DIR/usr/sbin/netgrip"
chmod 755 "$PKG_DIR/usr/sbin/netgrip"
cp "$REPO_ROOT/deploy/openwrt/netgrip/files/netgrip.init" "$PKG_DIR/etc/init.d/netgrip"
chmod 755 "$PKG_DIR/etc/init.d/netgrip"
cp "$REPO_ROOT/deploy/openwrt/netgrip/files/netgrip-restore-rules" "$PKG_DIR/usr/libexec/netgrip-restore-rules"
chmod 755 "$PKG_DIR/usr/libexec/netgrip-restore-rules"

# CONTROL files
cat > "$PKG_DIR/CONTROL/control" << CTRL
Package: netgrip
Version: ${PKG_VERSION:-0.0.0}-${PKG_RELEASE:-1}
License: AGPL-3.0-only
Section: utils
Architecture: ${PKG_ARCH}
Maintainer: Nacho <netgrip@cloudless.club>
Description: Lightweight on-router companion panel for OpenWrt
 Simple visual panel that complements LuCI: dashboard plus service
 toggles that deploy real configuration with snapshot and rollback.
CTRL

cat > "$PKG_DIR/CONTROL/postinst" << 'POSTINST'
#!/bin/sh
if [ -n "$IPKG_INSTROOT" ]; then exit 0; fi
# apk does not overwrite an existing /etc/init.d script (it stages the new
# one as .apk-new). The packaged init script must win on upgrades; local
# customizations live in UCI instead (#210).
#
# #210 migration: carry a local -port edit from the old init script into
# netgrip.main.port so the upgrade does not lose it. The old default (8080)
# is NOT migrated: 8090 exists precisely to avoid the GL.iNet uhttpd
# collision.
if [ -f /etc/init.d/netgrip ]; then
  old_port=$(sed -n 's/.*-port \([0-9][0-9]*\).*/\1/p' /etc/init.d/netgrip | head -n1)
  if [ -n "$old_port" ] && [ "$old_port" != "8080" ] && [ "$old_port" != "8090" ]; then
    touch /etc/config/netgrip
    grep -q "config panel 'main'" /etc/config/netgrip 2>/dev/null || printf "config panel 'main'\n" >> /etc/config/netgrip
    uci set netgrip.main.port="$old_port" 2>/dev/null || true
    uci commit netgrip 2>/dev/null || true
  fi
fi
if [ -f /etc/init.d/netgrip.apk-new ]; then
  cp /etc/init.d/netgrip.apk-new /etc/init.d/netgrip
  rm -f /etc/init.d/netgrip.apk-new
  chmod 755 /etc/init.d/netgrip
fi
# A standalone netpulse agent must not keep running next to netgrip: netgrip
# embeds the agent, running both is NOT supported. Stop it now (closing the
# double-push window); the first netgrip start adopts its config and removes
# the rest of the standalone artifacts.
if [ -f /etc/netpulse-agent.env ] || [ -f /usr/sbin/netpulse-agent ] || [ -f /etc/init.d/netpulse-agent ]; then
  echo "netgrip: standalone netpulse-agent detected."
  echo "netgrip: netgrip embeds the netpulse agent, running both is NOT supported."
  echo "netgrip: its config will be adopted and the standalone removed on first start."
  if [ -x /etc/init.d/netpulse-agent ]; then
    /etc/init.d/netpulse-agent stop >/dev/null 2>&1 || true
  fi
fi
/etc/init.d/netgrip enable 2>/dev/null || true
/etc/init.d/netgrip restart 2>/dev/null || true
# Survive sysupgrades: the apk registry does not survive, but the
# preserved files do, so procd starts the panel on first boot.
# /etc/netgrip/ keeps the netpulse embedded-agent env across upgrades.
for f in /usr/sbin/netgrip /etc/init.d/netgrip /etc/rc.d/S99netgrip /usr/libexec/netgrip-restore-rules /etc/netgrip/; do
  grep -qxF "$f" /etc/sysupgrade.conf 2>/dev/null || echo "$f" >> /etc/sysupgrade.conf
done
exit 0
POSTINST
chmod 755 "$PKG_DIR/CONTROL/postinst"

cat > "$PKG_DIR/CONTROL/prerm" << 'PRERM'
#!/bin/sh
/etc/init.d/netgrip stop 2>/dev/null || true
/etc/init.d/netgrip disable 2>/dev/null || true
exit 0
PRERM
chmod 755 "$PKG_DIR/CONTROL/prerm"

OUT_DIR="$REPO_ROOT/dist/packages"
mkdir -p "$OUT_DIR"

if [ "$FORMAT" = "ipk" ]; then
  echo "  Building .ipk..."
  "$SDK_DIR/scripts/ipkg-build" "$PKG_DIR" "$OUT_DIR"
  echo "  IPK: $(ls "$OUT_DIR"/netgrip_*.ipk)"

elif [ "$FORMAT" = "apk" ]; then
  echo "  Building .apk via SDK..."
  # Source-less Makefile with no-op Build/Compile: the prebuilt binary and
  # its files are staged in files/, so the SDK compiles nothing.
  PKG_DIR_SDK="$SDK_DIR/package/utils/netgrip"
  mkdir -p "$PKG_DIR_SDK/files"
  cp -r "$PKG_DIR/usr" "$PKG_DIR/etc" "$PKG_DIR_SDK/files/"
  cp "$REPO_ROOT/deploy/openwrt/netgrip/Makefile" "$PKG_DIR_SDK/Makefile"
  sed -i "s/^PKG_VERSION:=.*/PKG_VERSION:=${PKG_VERSION:-0.0.0}/; s/^PKG_RELEASE:=.*/PKG_RELEASE:=${PKG_RELEASE:-1}/" "$PKG_DIR_SDK/Makefile"

  cd "$SDK_DIR"
  # A valid .config without a terminal (the 25.12 SDK invokes menuconfig
  # when .config is missing and dies with 'Error opening terminal' in CI).
  make defconfig >/dev/null 2>&1 || true
  make package/netgrip/compile V=s 2>&1 | tail -15
  find bin/packages -name "netgrip*.apk" -exec cp {} "$OUT_DIR/" \;
  echo "  APK: $(ls "$OUT_DIR"/netgrip*.apk 2>/dev/null || echo 'NOT FOUND')"

else
  echo "ERROR: FORMAT must be ipk or apk" >&2
  exit 1
fi

echo "=== package.sh: OK ==="
