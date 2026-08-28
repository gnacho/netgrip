include $(TOPDIR)/rules.mk

PKG_NAME:=netgrip
PKG_VERSION:=0.1.6
PKG_RELEASE:=1

PKG_LICENSE:=AGPL-3.0-only
PKG_MAINTAINER:=Nacho <netgrip@cloudless.club>

include $(INCLUDE_DIR)/package.mk

define Package/netgrip
	SECTION:=utils
	CATEGORY:=Utilities
	TITLE:=Lightweight on-router companion panel for OpenWrt
	URL:=https://github.com/gnacho/netgrip
	DEPENDS:=@(aarch64||arm||x86_64)
endef

define Package/netgrip/description
	Simple visual panel that complements LuCI: dashboard plus service
	toggles that deploy real configuration with snapshot and rollback.
endef

define Build/Compile
endef

define Package/netgrip/install
	$(INSTALL_DIR) $(1)/usr/sbin
	$(INSTALL_BIN) ./files/usr/sbin/netgrip $(1)/usr/sbin/netgrip
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/etc/init.d/netgrip $(1)/etc/init.d/netgrip
endef

define Package/netgrip/postinst
#!/bin/sh
if [ -z "$${IPKG_INSTROOT}" ]; then
	/etc/init.d/netgrip enable 2>/dev/null || true
	/etc/init.d/netgrip restart 2>/dev/null || true
	# Survive sysupgrades: the apk registry does not survive, but the
	# preserved files do, so procd starts the panel on first boot.
	for f in /usr/sbin/netgrip /etc/init.d/netgrip /etc/rc.d/S99netgrip; do
		grep -qxF "$$f" /etc/sysupgrade.conf 2>/dev/null || echo "$$f" >> /etc/sysupgrade.conf
	done
fi
exit 0
endef

define Package/netgrip/prerm
#!/bin/sh
if [ -z "$${IPKG_INSTROOT}" ]; then
	/etc/init.d/netgrip stop 2>/dev/null || true
	/etc/init.d/netgrip disable 2>/dev/null || true
fi
exit 0
endef

$(eval $(call BuildPackage,netgrip))
