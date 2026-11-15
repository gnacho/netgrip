#!/bin/sh
# Packs the built frontend (internal/server/dist) into a deterministic tarball
# with "dist/" at the root. The openwrt feed package downloads this as a
# separate asset, the same way adguardhome ships its frontend, and pins its
# sha256 in the package Makefile, so the bytes must not change between builds:
# sorted entries, fixed mtime, no owner, gzip without name or timestamp.
#
# uso: deploy/pack-frontend.sh <version> [salida]
set -eu

VERSION="${1:?uso: pack-frontend.sh <version> [salida]}"
OUT="${2:-dist/netgrip-frontend-$VERSION.tar.gz}"
SRC="${SRC:-internal/server/dist}"

[ -f "$SRC/index.html" ] || {
	echo "ERROR: $SRC/index.html no existe; construye el front antes (cd app && npm run build)" >&2
	exit 1
}

mkdir -p "$(dirname "$OUT")"
tar --sort=name --owner=0 --group=0 --numeric-owner --mtime='@0' \
	--format=gnu -C "$(dirname "$SRC")" -cf - "$(basename "$SRC")" |
	gzip -n -9 >"$OUT"

echo "escrito $OUT ($(wc -c <"$OUT") bytes, sha256 $(sha256sum "$OUT" | cut -d' ' -f1))"
