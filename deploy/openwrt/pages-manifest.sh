#!/bin/sh
# Build the GitHub Pages release manifest consumed by the self-updater when
# api.github.com is unavailable. Only assets used by the updater belong here:
# unsigned debug binaries and OpenWrt packages keep their existing channels.
set -eu

tag=$1
out=$2

gh release view "$tag" --repo gnacho/netgrip --json tagName,body,assets |
	jq --arg base "https://gnacho.github.io/netgrip" '{
		tag_name: .tagName,
		body: .body,
		assets: [
			.assets[]
			| select(.name | test("^netgrip-linux-(amd64|arm64|armv7|armv6|armv5|mipsle|mips)(\\.sha256(\\.sig)?)?$"))
			| {
				name: .name,
				browser_download_url: ($base + "/" + .name),
				size: .size
			}
		]
	}' > "$out"
