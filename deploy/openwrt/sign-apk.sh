#!/bin/sh
# sign-apk.sh - lays out the OpenWrt 25 apk feed and signs every index.
#
# Usage:
#   sign-apk.sh <feed-dir> <ec-private-key.pem> <ec-public-key.pem>
#
# Input: a directory holding the release .apk files, whatever names they have
# (release assets carry the package architecture as a suffix).
#
# Output, inside the same directory, the layout apk expects:
#   <feed-dir>/<pkgarch>/packages.adb        signed index
#   <feed-dir>/<pkgarch>/netgrip-<version>.apk
#   <feed-dir>/<pkgarch>/netgrip-feed.pem    public key that trusts that index
#   <feed-dir>/netgrip-feed.pem              same key, stable URL
#
# Why the split: a v3 index records no filenames. apk derives
# "<name>-<version>.apk" beside the index, so one flat directory cannot hold
# twelve architectures of the same name and version, and the architecture
# suffix in the release asset name turns every download into a 404. One
# directory per package architecture is the layout OpenWrt itself publishes.
#
# Why apk signs the index: the trust anchor for a feed is an EC public key in
# /etc/apk/keys, and apk reads signatures embedded in the adb container. A
# usign sidecar (packages.adb.asc) is not read by apk at all, so the signing
# has to happen with `apk mkndx --sign-key` (or `apk adbsign`).
#
# Requires: the apk-tools 3 from the OpenWrt SDK on PATH (the Alpine build
# refuses these packages).
#
# NOTE (#296): the packages themselves are unsigned because the OpenWrt SDK
# cannot sign them. That is fine for feed installs, where trust comes from the
# signed index and the hashes it records. A local `apk add ./file.apk` still
# needs --allow-untrusted because there is no index to check against.

set -eu

feed_dir="${1:?usage: sign-apk.sh <feed-dir> <private-key.pem> <public-key.pem>}"
privkey="${2:?usage: sign-apk.sh <feed-dir> <private-key.pem> <public-key.pem>}"
pubkey="${3:?usage: sign-apk.sh <feed-dir> <private-key.pem> <public-key.pem>}"

[ -d "$feed_dir" ] || { echo "Error: feed directory not found: $feed_dir" >&2; exit 1; }
[ -f "$privkey" ] || { echo "Error: signing key not found: $privkey" >&2; exit 1; }
[ -f "$pubkey" ] || { echo "Error: public key not found: $pubkey" >&2; exit 1; }
command -v apk >/dev/null 2>&1 || { echo "Error: apk not on PATH (export the SDK host bin)" >&2; exit 1; }

feed_dir="$(cd "$feed_dir" && pwd)"
pubkey="$(cd "$(dirname "$pubkey")" && pwd)/$(basename "$pubkey")"
privkey="$(cd "$(dirname "$privkey")" && pwd)/$(basename "$privkey")"

# Keys are only loaded when the directory is absolute: apk resolves --keys-dir
# relative to --root and silently finds nothing otherwise.
verify_dir="$(mktemp -d)"
trap 'rm -rf "$verify_dir"' EXIT INT TERM
cp "$pubkey" "$verify_dir/"

# Field of the package info block, e.g. "arch".
info_field() {
	apk adbdump "$1" 2>/dev/null | awk -v key="$2:" '$1 == key { print $2; exit }'
}

noarch_pkgs=""
for pkg in "$feed_dir"/*.apk; do
	[ -f "$pkg" ] || continue
	name="$(info_field "$pkg" name)"
	version="$(info_field "$pkg" version)"
	arch="$(info_field "$pkg" arch)"
	if [ -z "$name" ] || [ -z "$version" ] || [ -z "$arch" ]; then
		echo "Error: cannot read name/version/arch from $pkg" >&2
		exit 1
	fi
	if [ "$arch" = "noarch" ]; then
		# noarch packages are listed by every index, the way OpenWrt keeps the
		# LuCI packages inside each target directory.
		noarch_pkgs="$noarch_pkgs $pkg"
		continue
	fi
	mkdir -p "$feed_dir/$arch"
	mv "$pkg" "$feed_dir/$arch/${name}-${version}.apk"
	echo "==> $arch/${name}-${version}.apk"
done

arch_dirs=0
for dir in "$feed_dir"/*/; do
	[ -d "$dir" ] || continue
	dir="${dir%/}"
	arch="${dir##*/}"
	arch_dirs=$((arch_dirs + 1))

	for pkg in $noarch_pkgs; do
		name="$(info_field "$pkg" name)"
		version="$(info_field "$pkg" version)"
		cp "$pkg" "$dir/${name}-${version}.apk"
	done
	cp "$pubkey" "$dir/netgrip-feed.pem"

	echo "==> Signing index for $arch"
	# mkndx signs and verifies in one step, and unlike adbsign it fails loudly
	# on an unusable key (adbsign prints the error and still exits 0).
	apk --allow-untrusted mkndx --sign-key "$privkey" -o "$dir/packages.adb" "$dir"/*.apk

	# Re-read what was just written: this is the check that catches a signing
	# step which silently did nothing. `apk verify` exits 0 even on an
	# untrusted file, so the verdict has to be read from the output.
	out="$(apk --keys-dir "$verify_dir" verify "$dir/packages.adb" 2>&1)"
	case "$out" in
		*": OK"*) ;;
		*)
			echo "Error: index for $arch does not verify with the published key: $out" >&2
			exit 1
			;;
	esac
done

if [ "$arch_dirs" -eq 0 ]; then
	echo "Error: no packages to index in $feed_dir" >&2
	exit 1
fi

# The noarch sources are only staging now that every index has its copy.
for pkg in $noarch_pkgs; do
	rm -f "$pkg"
done
cp "$pubkey" "$feed_dir/netgrip-feed.pem"

echo "==> Done"
find "$feed_dir" -maxdepth 2 -name 'packages.adb' | sort
