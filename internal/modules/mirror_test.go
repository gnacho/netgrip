package modules

import (
	"strings"
	"testing"
)

func TestParseMirrorConf(t *testing.T) {
	// Formato real escrito por writeMirrorConf.
	monitor, sources := parseMirrorConf("monitor=lan21\nsources=lan20 lan22\n")
	if monitor != "lan21" {
		t.Fatalf("monitor: se obtuvo %q, se esperaba lan21", monitor)
	}
	if len(sources) != 2 || sources[0] != "lan20" || sources[1] != "lan22" {
		t.Fatalf("sources: se obtuvo %v, se esperaban [lan20 lan22]", sources)
	}

	// Comentarios, lineas vacias y basura se toleran.
	monitor, sources = parseMirrorConf("# espejo\n\nmonitor = lan21\nsources=  lan20   lan22  \ngarbage\n")
	if monitor != "lan21" || len(sources) != 2 {
		t.Fatalf("tolerancia: monitor=%q sources=%v", monitor, sources)
	}

	// Fichero vacio o inexistente -> espejo desactivado. El slice debe
	// ser no-nil: serializado como [] y no como null, que crashea la
	// pestana Trafico en el cliente (join sobre null).
	monitor, sources = parseMirrorConf("")
	if monitor != "" || sources == nil || len(sources) != 0 {
		t.Fatalf("vacio: monitor=%q sources=%v", monitor, sources)
	}

	// sources sin monitor (estado inconsistente): se devuelve tal cual, la
	// validacion al aplicar lo rechaza.
	monitor, sources = parseMirrorConf("sources=lan20\n")
	if monitor != "" || len(sources) != 1 {
		t.Fatalf("sources sin monitor: monitor=%q sources=%v", monitor, sources)
	}
}

func TestParseMirrorShow(t *testing.T) {
	// Salida real de `tc filter show dev lan20 ingress` en el LGS352C con
	// el espejo aplicado (rtl931x, in_hw porque el ASIC lo replica).
	show := "filter protocol all pref 10 matchall chain 0 \n" +
		"filter protocol all pref 10 matchall chain 0 handle 0x1 \n" +
		"  in_hw\n" +
		"\taction order 1: mirred (Egress Mirror to device lan21) pipe\n" +
		"\tindex 1 ref 1 bind 1\n"
	monitor, ok := parseMirrorShow(show)
	if !ok || monitor != "lan21" {
		t.Fatalf("debe detectar lan21 como monitor, se obtuvo %q (ok=%v)", monitor, ok)
	}

	// Sin espejo: filtros de otro tipo (p. ej. storm) o nada.
	if _, ok := parseMirrorShow(""); ok {
		t.Fatal("salida vacia no debe detectar espejo")
	}
	storm := "filter protocol all pref 1 basic chain 0 handle 0x1 \n cmp(u16 at 0 layer 0 mask 0xffff eq 65535)\n police 0x1 rate 10Mbit burst 32Kb action drop\n"
	if _, ok := parseMirrorShow(storm); ok {
		t.Fatal("un filtro storm no es espejo")
	}
}

func TestMirrorFilterArgs(t *testing.T) {
	args := mirrorFilterArgs("lan20", "ingress", "lan21", prioMirrorIngress)
	want := "filter add dev lan20 ingress protocol all prio 10 matchall action mirred egress mirror dev lan21"
	if strings.Join(args, " ") != want {
		t.Fatalf("args ingress: se obtuvo %q, se esperaba %q", strings.Join(args, " "), want)
	}
	args = mirrorFilterArgs("lan20", "egress", "lan21", prioMirrorEgress)
	want = "filter add dev lan20 egress protocol all prio 11 matchall action mirred egress mirror dev lan21"
	if strings.Join(args, " ") != want {
		t.Fatalf("args egress: se obtuvo %q, se esperaba %q", strings.Join(args, " "), want)
	}
}

func TestValidateMirrorReq(t *testing.T) {
	bridge := map[string]bool{"lan1": true, "lan20": true, "lan21": true}

	cases := []struct {
		name    string
		req     MirrorSetRequest
		wantErr string
	}{
		{"ok", MirrorSetRequest{Monitor: "lan21", Sources: []string{"lan20"}}, ""},
		{"varias fuentes", MirrorSetRequest{Monitor: "lan21", Sources: []string{"lan20", "lan1"}}, ""},
		{"desactivar", MirrorSetRequest{}, ""},
		{"monitor fuera del puente", MirrorSetRequest{Monitor: "eth9", Sources: []string{"lan20"}}, "not in the bridge"},
		{"origen fuera del puente", MirrorSetRequest{Monitor: "lan21", Sources: []string{"eth9"}}, "not in the bridge"},
		{"origen igual que monitor", MirrorSetRequest{Monitor: "lan21", Sources: []string{"lan21"}}, "is the monitor port"},
		{"origen duplicado", MirrorSetRequest{Monitor: "lan21", Sources: []string{"lan20", "lan20"}}, "duplicate"},
		{"sin origenes", MirrorSetRequest{Monitor: "lan21"}, "at least one"},
		{"fuentes sin monitor", MirrorSetRequest{Sources: []string{"lan20"}}, "require a monitor"},
	}
	for _, tc := range cases {
		err := validateMirrorReq(tc.req, bridge)
		if tc.wantErr == "" && err != nil {
			t.Fatalf("%s: error inesperado: %v", tc.name, err)
		}
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("%s: se esperaba error con %q, se obtuvo %v", tc.name, tc.wantErr, err)
			}
		}
	}
}

func TestDefaultGatewayIP(t *testing.T) {
	// Fragmento real de /proc/net/route del LGS352C (gateway 192.168.1.1).
	route := "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n" +
		"switch.1\t00000000\tC0A80101\t0003\t0\t0\t0\t00000000\t0\t0\t0\n" +
		"switch.1\t00FFFFFF\t00000000\t0001\t0\t0\t0\t00000000\t0\t0\t0\n"
	if got := defaultGatewayIP(route); got != "192.168.1.1" {
		t.Fatalf("gateway: se obtuvo %q, se esperaba 192.168.1.1", got)
	}
	if got := defaultGatewayIP("Iface\tDestination\n"); got != "" {
		t.Fatalf("sin ruta por defecto debe quedar vacio, se obtuvo %q", got)
	}
}

func TestArpMACForIP(t *testing.T) {
	arp := "IP address       HW type     Flags       HW address            Mask     Device\n" +
		"192.168.1.1      0x1         0x2         94:83:c4:ba:bf:aa     *        switch.1\n" +
		"192.168.1.104    0x1         0x4         c0:1c:6a:6f:53:d9     *        switch.1\n"
	if got := arpMACForIP(arp, "192.168.1.1"); got != "94:83:c4:ba:bf:aa" {
		t.Fatalf("MAC: se obtuvo %q, se esperaba 94:83:c4:ba:bf:aa", got)
	}
	if got := arpMACForIP(arp, "10.0.0.1"); got != "" {
		t.Fatalf("IP ausente debe quedar vacio, se obtuvo %q", got)
	}
	// MAC nula (entrada incompleta): no vale como gateway resoluble.
	nullArp := "192.168.1.1      0x1         0x0         00:00:00:00:00:00     *        switch.1\n"
	if got := arpMACForIP(nullArp, "192.168.1.1"); got != "" {
		t.Fatalf("MAC nula no debe usarse, se obtuvo %q", got)
	}
}

func TestMirrorSupportCachedAndSeam(t *testing.T) {
	oldProbe := probeTcMirred
	t.Cleanup(func() { probeTcMirred = oldProbe; resetMirrorSupportForTest() })

	calls := 0
	probeTcMirred = func() (bool, bool) { calls++; return true, true }
	resetMirrorSupportForTest()

	m, e := mirrorSupport()
	if !m || !e || calls != 1 {
		t.Fatalf("primera llamada: (%v,%v) calls=%d", m, e, calls)
	}
	mirrorSupport()
	if calls != 1 {
		t.Fatalf("la cache debe evitar re-probar, calls=%d", calls)
	}
}
