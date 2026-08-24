#!/bin/bash
# package-luci.sh builds luci-app-owpanel as .ipk or .apk using the SDK.
# The package is files-only (PKGARCH all), so one artifact serves every
# aarch64 router. The .apk is built with the REAL apk mkpkg from the SDK
# (an .ipk renamed to .apk does NOT install on apk v3).
#
# Usage:
#   package-luci.sh <SDK_VERSION> <SDK_TARGET> <SDK_SUBTARGET> <FORMAT>
#
# Example:
#   package-luci.sh 25.12.5 qualcommax/ipq807x "" apk

set -euo pipefail

SDK_VERSION="${1:?missing SDK_VERSION}"
SDK_TARGET="${2:?missing SDK_TARGET}"
SDK_SUBTARGET="${3:-}"
FORMAT="${4:?missing FORMAT (ipk|apk)}"

echo "=== package-luci.sh: $FORMAT SDK=$SDK_VERSION target=$SDK_TARGET ==="

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
APP_DIR="$SCRIPT_DIR/luci-app-owpanel"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

PKG_VERSION="${PKG_VERSION:-$(sed -n 's/^PKG_VERSION?=//p' "$APP_DIR/Makefile" | head -1)}"
PKG_RELEASE="${PKG_RELEASE:-$(sed -n 's/^PKG_RELEASE?=//p' "$APP_DIR/Makefile" | head -1)}"

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

OUT_DIR="$REPO_ROOT/dist/packages"
mkdir -p "$OUT_DIR"

PKG_DIR="$WORK_DIR/luci-app-owpanel-pkg"
mkdir -p "$PKG_DIR/CONTROL"
cp -r "$APP_DIR/files/." "$PKG_DIR/"

write_controls() {
  cat > "$PKG_DIR/CONTROL/control" << CTRL
Package: luci-app-owpanel
Version: ${PKG_VERSION:-0.0.0}-${PKG_RELEASE:-1}
Depends: luci-base, owpanel
License: AGPL-3.0-only
Section: luci
Architecture: all
Maintainer: Nacho <owpanel@cloudless.club>
Description: owpanel embedded in LuCI
 LuCI menu entry that embeds the owpanel companion panel (served
 locally on port 8080) inside the LuCI interface.
CTRL

  cat > "$PKG_DIR/CONTROL/postinst" << 'POSTINST'
#!/bin/sh
[ -n "$IPKG_INSTROOT" ] && exit 0
/etc/init.d/rpcd restart 2>/dev/null || true
rm -f /tmp/luci-indexcache* 2>/dev/null || true
exit 0
POSTINST
  chmod 755 "$PKG_DIR/CONTROL/postinst"

  cat > "$PKG_DIR/CONTROL/postrm" << 'POSTRM'
#!/bin/sh
[ -n "$IPKG_INSTROOT" ] && exit 0
/etc/init.d/rpcd restart 2>/dev/null || true
rm -f /tmp/luci-indexcache* 2>/dev/null || true
exit 0
POSTRM
  chmod 755 "$PKG_DIR/CONTROL/postrm"
}

if [ "$FORMAT" = "ipk" ]; then
  echo "  Building .ipk..."
  write_controls
  "$SDK_DIR/scripts/ipkg-build" "$PKG_DIR" "$OUT_DIR"
  echo "  IPK: $(ls "$OUT_DIR"/luci-app-owpanel_*.ipk)"

elif [ "$FORMAT" = "apk" ]; then
  echo "  Building .apk via SDK make (same proven flow as the owpanel package)..."
  PKG_DIR_SDK="$SDK_DIR/package/utils/luci-app-owpanel"
  mkdir -p "$PKG_DIR_SDK/files"
  cp -r "$APP_DIR/files/." "$PKG_DIR_SDK/files/"
  cp "$APP_DIR/Makefile" "$PKG_DIR_SDK/Makefile"
  sed -i "s/^PKG_VERSION?=.*/PKG_VERSION:=${PKG_VERSION:-0.0.0}/; s/^PKG_RELEASE?=.*/PKG_RELEASE:=${PKG_RELEASE:-1}/" "$PKG_DIR_SDK/Makefile"

  cd "$SDK_DIR"
  make defconfig >/dev/null 2>&1 || true
  make package/luci-app-owpanel/compile V=s 2>&1 | tail -10
  find bin/packages -name "luci-app-owpanel*.apk" -exec cp {} "$OUT_DIR/" \;
  echo "  APK: $(ls "$OUT_DIR"/luci-app-owpanel-*.apk 2>/dev/null || echo 'NOT FOUND')"

else
  echo "ERROR: FORMAT must be ipk or apk" >&2
  exit 1
fi

echo "=== package-luci.sh: OK ==="
