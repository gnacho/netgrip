# SPDX-License-Identifier: AGPL-3.0-only
#
# Feed package for openwrt/packages. Built from source: the Go binary is
# compiled by the buildroot with golang-package.mk, and the frontend bundle
# (built with Node in the upstream repo) ships as its own pinned download,
# the same approach adguardhome uses for its frontend.

include $(TOPDIR)/rules.mk

PKG_NAME:=netgrip
PKG_VERSION:=0.79.0
PKG_RELEASE:=1

PKG_SOURCE:=$(PKG_NAME)-$(PKG_VERSION).tar.gz
PKG_SOURCE_URL:=@GITHUB/gnacho/netgrip/tar.gz/v$(PKG_VERSION)?
PKG_HASH:=74c326f2bed7641cbd1f512351de15286de7db1b18c65ed9b7c959b56c129a64
PKG_BUILD_DIR:=$(BUILD_DIR)/netgrip-$(PKG_VERSION)

# Se genera con deploy/pack-frontend.sh y se publica como asset de la release.
# El hash es el del asset, no el de un build local distinto.
FRONTEND_TARBALL:=netgrip-frontend-$(PKG_VERSION).tar.gz
FRONTEND_URL:=https://github.com/gnacho/netgrip/releases/download/v$(PKG_VERSION)/
FRONTEND_HASH:=21048616223e105162b6203586b4ded7632481a813eeee09275aee5f3a7d09eb

PKG_LICENSE:=AGPL-3.0-only
PKG_LICENSE_FILES:=LICENSE
PKG_MAINTAINER:=Nacho <netgrip@cloudless.club>

PKG_BUILD_DEPENDS:=golang/host
PKG_BUILD_PARALLEL:=1
PKG_BUILD_FLAGS:=no-mips16

GO_PKG:=github.com/gnacho/netgrip
GO_PKG_BUILD_PKG:=github.com/gnacho/netgrip/cmd/netgrip
GO_PKG_INSTALL_BIN_PATH:=/usr/sbin
# Recursivo (=) a propósito: GO_ARCH y GO_ARM los define golang-package.mk,
# que se incluye más abajo. Sin esto, en las variantes arm el binario se
# identificaría como "arm" y no como armv5/armv6/armv7.
GO_PKG_LDFLAGS_X=main.version=$(PKG_VERSION) github.com/gnacho/netgrip/internal/modules.releaseArch=$(if $(filter arm,$(GO_ARCH)),armv$(GO_ARM),$(GO_ARCH))

include $(INCLUDE_DIR)/package.mk
include ../../lang/golang/golang-package.mk

define Download/netgrip-frontend
  URL:=$(FRONTEND_URL)
  URL_FILE:=$(FRONTEND_TARBALL)
  FILE:=$(FRONTEND_TARBALL)
  HASH:=$(FRONTEND_HASH)
endef
$(eval $(call Download,netgrip-frontend))

define Package/netgrip
  SECTION:=utils
  CATEGORY:=Utilities
  TITLE:=Companion panel for OpenWrt services, with snapshot and rollback
  URL:=https://github.com/gnacho/netgrip
  DEPENDS:=$(GO_ARCH_DEPENDS) @(aarch64||arm||mips||mipsel||x86_64) +ethtool-full +rpcd-mod-rpcsys
endef

define Package/netgrip/description
 NetGrip runs on the router itself and puts a switch in front of the services
 OpenWrt already exposes through rpcd: WireGuard, guest Wi-Fi, SQM, DNS,
 firewall, VLANs and more. Every change takes a config snapshot, is checked,
 and rolls back by itself if the service does not come up. It serves its own
 interface, so it does not need any other runtime.
endef

define Build/Prepare
	$(call Build/Prepare/Default)
	mkdir -p $(PKG_BUILD_DIR)/internal/server
	gzip -dc $(DL_DIR)/$(FRONTEND_TARBALL) | $(HOST_TAR) -C $(PKG_BUILD_DIR)/internal/server $(TAR_OPTIONS)
endef

define Package/netgrip/install
	$(call GoPackage/Package/Install/Bin,$(1))

	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/netgrip.init $(1)/etc/init.d/netgrip

	$(INSTALL_DIR) $(1)/usr/libexec
	$(INSTALL_BIN) ./files/netgrip-restore-rules $(1)/usr/libexec/netgrip-restore-rules
	$(INSTALL_BIN) ./files/netgrip-heal-register $(1)/usr/libexec/netgrip-heal-register
endef

$(eval $(call BuildPackage,netgrip))
