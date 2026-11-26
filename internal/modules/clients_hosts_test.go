package modules

import "testing"

// Las reservas son datos de dnsmasq, no nuestros: la tarjeta salia vacia en
// un router configurado desde otro panel porque solo se listaban las netgrip_host_*
// (#621).
func TestParseHostSections(t *testing.T) {
	const uciShow = `dhcp.lan=dhcp
dhcp.@host[0]=host
dhcp.rt2=host
dhcp.rt2.name='rt2'
dhcp.rt2.mac='8C:DE:F9:33:71:58'
dhcp.netgrip_host_aabbccddeeff=host
dhcp.wan=dnsmasq
dhcp.ipset=ipset`

	got := parseHostSections(uciShow)
	want := []string{"@host[0]", "rt2", "netgrip_host_aabbccddeeff"}
	if len(got) != len(want) {
		t.Fatalf("sections = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sections = %v, want %v", got, want)
		}
	}
}

func TestMacInList(t *testing.T) {
	if !macInList("8C:DE:F9:33:71:58", "8c:de:f9:33:71:58") {
		t.Error("una MAC debe encontrarse ignorando mayusculas")
	}
	if !macInList("8c:de:f9:33:71:58 aa:bb:cc:dd:ee:ff", "aa:bb:cc:dd:ee:ff") {
		t.Error("una lista de varias MAC debe encontrar la segunda")
	}
	if macInList("8c:de:f9:33:71:58", "aa:bb:cc:dd:ee:ff") {
		t.Error("una MAC distinta no debe coincidir")
	}
}
