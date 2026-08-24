include $(TOPDIR)/rules.mk

PKG_NAME:=owpanel
PKG_VERSION:=0.1.6
PKG_RELEASE:=1

PKG_LICENSE:=AGPL-3.0-only
PKG_MAINTAINER:=Nacho <owpanel@cloudless.club>

include $(INCLUDE_DIR)/package.mk

define Package/owpanel
	SECTION:=utils
	CATEGORY:=Utilities
	TITLE:=Lightweight on-router companion panel for OpenWrt
	URL:=https://github.com/gnacho/owpanel
	DEPENDS:=@(aarch64||arm||x86_64)
endef

define Package/owpanel/description
	Simple visual panel that complements LuCI: dashboard plus service
	toggles that deploy real configuration with snapshot and rollback.
endef

define Build/Compile
endef

define Package/owpanel/install
	$(INSTALL_DIR) $(1)/usr/sbin
	$(INSTALL_BIN) ./files/usr/sbin/owpanel $(1)/usr/sbin/owpanel
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/etc/init.d/owpanel $(1)/etc/init.d/owpanel
endef

define Package/owpanel/postinst
#!/bin/sh
if [ -z "$${IPKG_INSTROOT}" ]; then
	/etc/init.d/owpanel enable 2>/dev/null || true
	/etc/init.d/owpanel restart 2>/dev/null || true
	# Survive sysupgrades: the apk registry does not survive, but the
	# preserved files do, so procd starts the panel on first boot.
	for f in /usr/sbin/owpanel /etc/init.d/owpanel /etc/rc.d/S99owpanel; do
		grep -qxF "$$f" /etc/sysupgrade.conf 2>/dev/null || echo "$$f" >> /etc/sysupgrade.conf
	done
fi
exit 0
endef

define Package/owpanel/prerm
#!/bin/sh
if [ -z "$${IPKG_INSTROOT}" ]; then
	/etc/init.d/owpanel stop 2>/dev/null || true
	/etc/init.d/owpanel disable 2>/dev/null || true
fi
exit 0
endef

$(eval $(call BuildPackage,owpanel))
