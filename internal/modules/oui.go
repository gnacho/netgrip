package modules

import "strings"

// oui.go maps MAC OUIs (the first three octets) to a vendor name for the
// L2 client inventory (#520). The table below is a partial, best-effort
// extraction from the local IEEE OUI database shipped with nmap
// (/usr/share/nmap/nmap-mac-prefixes), not typed from memory. Vendors that
// do not appear in that database are deliberately omitted rather than
// invented. Long legal suffixes are trimmed only when they add no signal
// (", Ltd", "Co.,Ltd"); the name is otherwise kept as the database has it.

// ouiVendors maps an uppercase "XXXXXX" prefix (bare hex, no separators) to
// its vendor name. Keys must stay in that single format: vendorFor builds the
// lookup key by concatenating the first three octets, and a key with
// separators would silently never match.
var ouiVendors = map[string]string{
	// Espressif (ESP8266/ESP32 and the Tuya modules built around them).
	"004B12": "Espressif",
	"007007": "Espressif",
	"048308": "Espressif",
	"04B247": "Espressif",

	// Xiaomi.
	"009EC8": "Xiaomi Communications",
	"00C30A": "Xiaomi Communications",
	"00EC0A": "Xiaomi Communications",
	"04106B": "Xiaomi Communications",
	"9C9D7E": "Beijing Xiaomi Mobile Software",

	// Tuya.
	"00337A": "Tuya Smart",
	"105A17": "Tuya Smart",
	"10D561": "Tuya Smart",
	"1869D8": "Tuya Smart",

	// TP-Link.
	"000AEB": "TP-Link Technologies",
	"001478": "TP-Link Technologies",
	"0019E0": "TP-Link Technologies",
	"001D0F": "TP-Link Technologies",

	// Amazon.
	"007147": "Amazon Technologies",
	"008621": "Amazon Technologies",
	"00BB3A": "Amazon Technologies",
	"00F361": "Amazon Technologies",

	// Google.
	"001A11": "Google",
	"00F620": "Google",
	"04006E": "Google",
	"04C8B0": "Google",

	// Apple.
	"000393": "Apple",
	"000502": "Apple",
	"000A27": "Apple",
	"000A95": "Apple",

	// Samsung.
	"0000F0": "Samsung Electronics",
	"0007AB": "Samsung Electronics",
	"001247": "Samsung Electronics",
	"0012FB": "Samsung Electronics",

	// Huawei.
	"001882": "Huawei Technologies",
	"001E10": "Huawei Technologies",
	"002568": "Huawei Technologies",
	"00259E": "Huawei Technologies",

	// Sonos.
	"00043C": "Sonos",
	"000E58": "Sonos",
	"347E5C": "Sonos",

	// ASUSTek.
	"000C6E": "ASUSTek Computer",
	"000EA6": "ASUSTek Computer",
	"00112F": "ASUSTek Computer",
	"0011D8": "ASUSTek Computer",

	// Raspberry Pi.
	"28CDC1": "Raspberry Pi Trading",
	"2CCF67": "Raspberry Pi Trading",
	"88A29E": "Raspberry Pi Trading",
	"B827EB": "Raspberry Pi Foundation",

	// Intel.
	"0012F0": "Intel Corporate",
	"001302": "Intel Corporate",
	"001320": "Intel Corporate",
	"0013CE": "Intel Corporate",

	// Broadcom.
	"0005B5": "Broadcom Technologies",
	"000AF7": "Broadcom",
	"000DB6": "Broadcom",
	"001018": "Broadcom",

	// Realtek.
	"00E04C": "Realtek Semiconductor",

	// Netgear.
	"00095B": "Netgear",
	"000FB5": "Netgear",
	"00146C": "Netgear",
	"00184D": "Netgear",

	// Ubiquiti.
	"00156D": "Ubiquiti",
	"002722": "Ubiquiti",
	"0418D6": "Ubiquiti",
	"0CEA14": "Ubiquiti",

	// Nintendo.
	"0009BF": "Nintendo",
	"001656": "Nintendo",
	"0017AB": "Nintendo",
	"00191D": "Nintendo",

	// D&M (Denon/Marantz).
	"0005CD": "D&M Holdings",
	"000678": "D&M Holdings",
	"8CA96F": "D&M Holdings",

	// OnePlus.
	"2CA7EF": "OnePlus Technology",
	"30BB7D": "OnePlus Technology",
	"4801C5": "OnePlus Technology",
	"487412": "OnePlus Technology",

	// GL.iNet.
	"9483C4": "GL Technologies",

	// Aqara / Lumi.
	"18C23C": "Lumi United Technology",
	"54EF44": "Lumi United Technology",

	// Philips.
	"00026C": "Philips CFT",
	"00054E": "Philips",
	"0008C6": "Philips Consumer Communications",
	"001788": "Philips Lighting",

	// Renesas.
	"001E01": "Renesas Technology Sales",
	"003055": "Renesas Technology America",
	"0091EB": "Renesas Electronics",

	// Texas Instruments.
	"001237": "Texas Instruments",
	"00124B": "Texas Instruments",
	"0012D1": "Texas Instruments",
	"0012D2": "Texas Instruments",
}

// vendorFor resolves a MAC's vendor from its OUI. random is true when the
// MAC's first octet has the locally administered bit set and the address is
// not multicast; such MACs are self-assigned (or privacy-randomized) and do
// not carry a meaningful OUI, so no vendor is looked up. mac must be in
// "xx:xx:xx:xx:xx:xx" form (upper or lower case).
func vendorFor(mac string) (vendor string, random bool) {
	octets := strings.Split(mac, ":")
	if len(octets) < 6 {
		return "", false
	}
	// Parse the first octet for the locally administered / multicast bits.
	var first byte
	for _, c := range []byte(octets[0]) {
		first = first<<4 | hexVal(c)
	}
	// Locally administered (0x02) but not multicast (0x01): no real OUI.
	if first&0x01 == 0 && first&0x02 != 0 {
		return "", true
	}
	prefix := strings.ToUpper(octets[0] + octets[1] + octets[2])
	return ouiVendors[prefix], false
}

func hexVal(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return 0
	}
}
