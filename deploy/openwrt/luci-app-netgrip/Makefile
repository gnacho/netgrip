# luci-app-netgrip OpenWrt Makefile
# Files-only LuCI app (JS/JSON, PKGARCH:=all): embeds the netgrip web UI
# (served by the netgrip package on :8090) inside LuCI. Flat Makefile on
# purpose (no luci.mk): builds in a bare SDK like the netgrip package.

include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-netgrip
PKG_VERSION?=0.5.8
PKG_RELEASE?=1

PKG_LICENSE:=AGPL-3.0-only
PKG_MAINTAINER:=Nacho <netgrip@cloudless.club>
PKGARCH:=all

include $(INCLUDE_DIR)/package.mk

define Package/luci-app-netgrip
	SECTION:=luci
	CATEGORY:=LuCI
	SUBMENU:=3. Applications
	TITLE:=netgrip embedded in LuCI
	URL:=https://github.com/gnacho/netgrip
	DEPENDS:=+luci-base +netgrip
	PKGARCH:=all
endef

define Package/luci-app-netgrip/description
	LuCI menu entry that embeds the netgrip companion panel (served
	locally on port 8090) inside the LuCI interface.
endef

define Build/Compile
endef

define Package/luci-app-netgrip/install
	$(INSTALL_DIR) $(1)/www/luci-static/resources/view/netgrip
	$(INSTALL_DATA) ./files/www/luci-static/resources/view/netgrip/panel.js $(1)/www/luci-static/resources/view/netgrip/
	$(INSTALL_DIR) $(1)/usr/share/luci/menu.d
	$(INSTALL_DATA) ./files/usr/share/luci/menu.d/luci-app-netgrip.json $(1)/usr/share/luci/menu.d/
	$(INSTALL_DIR) $(1)/usr/share/rpcd/acl.d
	$(INSTALL_DATA) ./files/usr/share/rpcd/acl.d/luci-app-netgrip.json $(1)/usr/share/rpcd/acl.d/
endef

define Package/luci-app-netgrip/postinst
#!/bin/sh
if [ -z "$${IPKG_INSTROOT}" ]; then
	/etc/init.d/rpcd restart 2>/dev/null || true
	rm -f /tmp/luci-indexcache* 2>/dev/null || true
fi
exit 0
endef

define Package/luci-app-netgrip/postrm
#!/bin/sh
if [ -z "$${IPKG_INSTROOT}" ]; then
	/etc/init.d/rpcd restart 2>/dev/null || true
	rm -f /tmp/luci-indexcache* 2>/dev/null || true
fi
exit 0
endef

$(eval $(call BuildPackage,luci-app-netgrip))
