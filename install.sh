#!/bin/sh
# netgrip one-line installer for OpenWrt (POSIX sh, busybox ash compatible).
#
# Usage (on the router):
#   wget -qO- https://raw.githubusercontent.com/gnacho/netgrip/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/gnacho/netgrip/main/install.sh | sh
#
# Env overrides (for testing local builds):
#   NETGRIP_VERSION=vX.Y.Z   install that release tag instead of latest
#   NETGRIP_ASSET_BASE=URL   download assets from URL/<asset> instead of the
#                            GitHub release (e.g. a local http.server)
#
# Exit codes: 1 usage/privileges, 2 no downloader, 3 unsupported arch or
# package manager, 4 download/resolve failure, 5 install/service failure.

set -eu

REPO="gnacho/netgrip"
RELEASES_URL="https://github.com/${REPO}/releases"
VERSION="${NETGRIP_VERSION:-}"
ASSET_BASE="${NETGRIP_ASSET_BASE:-}"

say() { echo "netgrip: $*"; }
die() { code=$1; shift; say "ERROR: $*" >&2; exit "$code"; }

[ "$(id -u)" = 0 ] || die 1 "run as root on the router (ssh root@<router>)"

# ---------------------------------------------------------- pkg manager --
if command -v apk >/dev/null 2>&1; then
  PKGR=apk
  # --print-arch reports apk's generic build ABI, not OpenWrt's package target.
  PKG_ARCH=$(awk 'NF && $1 != "all" && $1 != "noarch" { print $1; exit }' /etc/apk/arch 2>/dev/null || true)
  [ -n "$PKG_ARCH" ] || PKG_ARCH=$(apk --print-arch 2>/dev/null || true)
elif command -v opkg >/dev/null 2>&1; then
  PKGR=opkg
  # Select the installable native architecture with the highest priority.
  PKG_ARCH=$(opkg print-architecture 2>/dev/null | awk '
    tolower($1) == "arch" && $2 != "all" && $2 != "noarch" {
      priority = $3 + 0
      if (!found || priority >= best) { arch = $2; best = priority; found = 1 }
    }
    END { print arch }
  ')
else
  die 3 "neither apk nor opkg found (OpenWrt 24.10 or newer required)"
fi
[ -n "$PKG_ARCH" ] || die 3 "could not determine the native package architecture"

# ----------------------------------------------------------- Go ABI arch --
# Package architectures are target-specific; release binaries use the much
# smaller set of Go ABI families. Packages retain the exact package architecture
# so several packages can reuse one generic release binary without collisions.
case "$PKG_ARCH" in
  x86_64) ASSET_ARCH=amd64 ;;
  aarch64|aarch64_*) ASSET_ARCH=arm64 ;;
  arm_arm926ej-s*|arm_xscale*|arm_fa526*) ASSET_ARCH=armv5 ;;
  arm_arm1176jzf-s*|arm_mpcore*) ASSET_ARCH=armv6 ;;
  armv7|arm_cortex-*|arm_*) ASSET_ARCH=armv7 ;;
  mipsel|mipsel_*) ASSET_ARCH=mipsle ;;
  mips|mips_*) ASSET_ARCH=mips ;;
  *)
    die 3 "unsupported package architecture: $PKG_ARCH"
    ;;
esac

# -------------------------------------------------------------- download --
fetch() {
  _url=$1
  _dest=$2
  rm -f "$_dest"
  if command -v wget >/dev/null 2>&1; then
    wget -q -T 30 -O "$_dest" "$_url" && [ -s "$_dest" ] && return 0
    rm -f "$_dest"
  fi
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --connect-timeout 20 -o "$_dest" "$_url" && [ -s "$_dest" ] && return 0
    rm -f "$_dest"
  fi
  return 1
}

command -v wget >/dev/null 2>&1 || command -v curl >/dev/null 2>&1 \
  || die 2 "need wget or curl to download (opkg/apt install one first)"

# ------------------------------------------------------------ latest tag --
latest_tag() {
  # Assets carry the version in their names, so the tag must be resolved
  # first. busybox wget has no -S/header mode, so both paths use the GitHub
  # API (small JSON, "tag_name" extracted with sed).
  _api="https://api.github.com/repos/${REPO}/releases/latest"
  _tag=""
  if command -v curl >/dev/null 2>&1; then
    _tag=$(curl -fsSL --connect-timeout 20 "$_api" 2>/dev/null \
      | sed -n 's/.*"tag_name": *"\(v[0-9][^"]*\)".*/\1/p' | head -n 1)
  fi
  if [ -z "$_tag" ] && command -v wget >/dev/null 2>&1; then
    _tag=$(wget -q -T 30 -O - "$_api" 2>/dev/null \
      | sed -n 's/.*"tag_name": *"\(v[0-9][^"]*\)".*/\1/p' | head -n 1)
  fi
  echo "$_tag"
}

if [ -z "$VERSION" ]; then
  VERSION=$(latest_tag) || true
  case "$VERSION" in
    v[0-9]*) ;;
    *) die 4 "could not resolve the latest release tag; set NETGRIP_VERSION=vX.Y.Z" ;;
  esac
fi
say "installing netgrip $VERSION ($PKG_ARCH -> $ASSET_ARCH, $PKGR)"

# --------------------------------------------------------------- package --
VER_NUM=${VERSION#v}
if [ -z "$ASSET_BASE" ]; then
  ASSET_BASE="${RELEASES_URL}/download/${VERSION}"
fi
if [ "$PKGR" = apk ]; then
  ASSET="netgrip-${VER_NUM}-r1-${PKG_ARCH}.apk"
else
  ASSET="netgrip_${VER_NUM}-1_${PKG_ARCH}.ipk"
fi
URL="${ASSET_BASE%/}/${ASSET}"

TMP=$(mktemp -d) || die 4 "mktemp failed"
trap 'rm -rf "$TMP"' EXIT INT TERM

say "downloading $URL"
if ! fetch "$URL" "$TMP/$ASSET"; then
  # Releases predating native APK filenames used a generic Go ABI suffix.
  if [ "$PKGR" != apk ] || ! fetch "${ASSET_BASE%/}/netgrip-${VER_NUM}-r1-${ASSET_ARCH}.apk" "$TMP/$ASSET"; then
    die 4 "download failed; no asset for this combination? check $RELEASES_URL"
  fi
fi

# --------------------------------------------------------------- install --
case "$PKGR" in
  apk)
    apk add --allow-untrusted "$TMP/$ASSET" || die 5 "apk add failed"
    ;;
  opkg)
    opkg install "$TMP/$ASSET" || die 5 "opkg install failed"
    ;;
esac

if [ -x /etc/init.d/netgrip ]; then
  /etc/init.d/netgrip enable || die 5 "could not enable the service"
  /etc/init.d/netgrip restart || die 5 "service failed to start; check with: logread | grep netgrip"
else
  die 5 "package installed but /etc/init.d/netgrip is missing"
fi

# ------------------------------------------------------------ final info --
lan_ip() {
  if command -v ubus >/dev/null 2>&1 && command -v jsonfilter >/dev/null 2>&1; then
    ubus call network.interface.lan status 2>/dev/null \
      | jsonfilter -e '@["ipv4-address"][0].address' 2>/dev/null && return 0
  fi
  if command -v uci >/dev/null 2>&1; then
    uci -q get network.lan.ipaddr 2>/dev/null && return 0
  fi
  echo "192.168.1.1"
}

say "done. netgrip $VERSION is running."
say "open the panel: http://$(lan_ip):8090"
say "log in with the router root password (same as LuCI); the setup wizard opens on first run."
