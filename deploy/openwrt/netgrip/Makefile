include $(TOPDIR)/rules.mk

PKG_NAME:=netgrip
PKG_VERSION:=0.24.0
PKG_RELEASE:=1

PKG_LICENSE:=AGPL-3.0-only
PKG_MAINTAINER:=Nacho <netgrip@cloudless.club>

include $(INCLUDE_DIR)/package.mk

define Package/netgrip
	SECTION:=utils
	CATEGORY:=Utilities
	TITLE:=Lightweight on-router companion panel for OpenWrt
	URL:=https://github.com/gnacho/netgrip
	DEPENDS:=@(aarch64||arm||x86_64) +ethtool-full
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
	$(INSTALL_DIR) $(1)/usr/libexec
	$(INSTALL_BIN) ./files/usr/libexec/netgrip-restore-rules $(1)/usr/libexec/netgrip-restore-rules
endef

define Package/netgrip/postinst
#!/bin/sh
if [ -z "$${IPKG_INSTROOT}" ]; then
	# apk does not overwrite an existing /etc/init.d script (it stages the
	# new one as .apk-new). The packaged init script must win on upgrades.
	if [ -f /etc/init.d/netgrip.apk-new ]; then
		cp /etc/init.d/netgrip.apk-new /etc/init.d/netgrip
		rm -f /etc/init.d/netgrip.apk-new
		chmod 755 /etc/init.d/netgrip
	fi
	# A standalone netpulse agent must not keep running next to netgrip:
	# netgrip embeds the agent, running both is NOT supported. Stop it now;
	# the first netgrip start adopts its config and removes the standalone.
	if [ -f /etc/netpulse-agent.env ] || [ -f /usr/sbin/netpulse-agent ] || [ -f /etc/init.d/netpulse-agent ]; then
		echo "netgrip: standalone netpulse-agent detected."
		echo "netgrip: netgrip embeds the netpulse agent, running both is NOT supported."
		echo "netgrip: its config will be adopted and the standalone removed on first start."
		if [ -x /etc/init.d/netpulse-agent ]; then
			/etc/init.d/netpulse-agent stop >/dev/null 2>&1 || true
		fi
	fi
	/etc/init.d/netgrip enable 2>/dev/null || true
	/etc/init.d/netgrip restart 2>/dev/null || true
	# Survive sysupgrades: the apk registry does not survive, but the
	# preserved files do, so procd starts the panel on first boot.
	# /etc/netgrip/ keeps the netpulse embedded-agent env across upgrades.
	for f in /usr/sbin/netgrip /etc/init.d/netgrip /etc/rc.d/S99netgrip /usr/libexec/netgrip-restore-rules /etc/netgrip/; do
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
