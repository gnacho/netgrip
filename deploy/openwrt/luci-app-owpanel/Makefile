# luci-app-owpanel OpenWrt Makefile
# Files-only LuCI app (JS/JSON, PKGARCH:=all): embeds the owpanel web UI
# (served by the owpanel package on :8080) inside LuCI. Flat Makefile on
# purpose (no luci.mk): builds in a bare SDK like the owpanel package.

include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-owpanel
PKG_VERSION?=0.5.8
PKG_RELEASE?=1

PKG_LICENSE:=AGPL-3.0-only
PKG_MAINTAINER:=Nacho <owpanel@cloudless.club>
PKGARCH:=all

include $(INCLUDE_DIR)/package.mk

define Package/luci-app-owpanel
	SECTION:=luci
	CATEGORY:=LuCI
	SUBMENU:=3. Applications
	TITLE:=owpanel embedded in LuCI
	URL:=https://github.com/gnacho/owpanel
	DEPENDS:=+luci-base +owpanel
	PKGARCH:=all
endef

define Package/luci-app-owpanel/description
	LuCI menu entry that embeds the owpanel companion panel (served
	locally on port 8080) inside the LuCI interface.
endef

define Build/Compile
endef

define Package/luci-app-owpanel/install
	$(INSTALL_DIR) $(1)/www/luci-static/resources/view/owpanel
	$(INSTALL_DATA) ./files/www/luci-static/resources/view/owpanel/panel.js $(1)/www/luci-static/resources/view/owpanel/
	$(INSTALL_DIR) $(1)/usr/share/luci/menu.d
	$(INSTALL_DATA) ./files/usr/share/luci/menu.d/luci-app-owpanel.json $(1)/usr/share/luci/menu.d/
	$(INSTALL_DIR) $(1)/usr/share/rpcd/acl.d
	$(INSTALL_DATA) ./files/usr/share/rpcd/acl.d/luci-app-owpanel.json $(1)/usr/share/rpcd/acl.d/
endef

define Package/luci-app-owpanel/postinst
#!/bin/sh
if [ -z "$${IPKG_INSTROOT}" ]; then
	/etc/init.d/rpcd restart 2>/dev/null || true
	rm -f /tmp/luci-indexcache* 2>/dev/null || true
fi
exit 0
endef

define Package/luci-app-owpanel/postrm
#!/bin/sh
if [ -z "$${IPKG_INSTROOT}" ]; then
	/etc/init.d/rpcd restart 2>/dev/null || true
	rm -f /tmp/luci-indexcache* 2>/dev/null || true
fi
exit 0
endef

$(eval $(call BuildPackage,luci-app-owpanel))
