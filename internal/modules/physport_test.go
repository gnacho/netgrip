package modules

import (
	"strings"
	"testing"
)

// Deteccion de Realtek DSA para eee_writable (#485): el `compatible` del
// devicetree es una lista NUL-separada; basta con que una entrada lleve
// rtl83/rtl93 para considerar la plataforma de solo lectura en EEE. Dato
// real capturado en el switch de validacion (rtl9311).
func TestCompatibleIsRealtekDSA(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{"rtl9311 real", "linksys,lgs352c\x00realtek,rtl9311-soc\x00", true},
		{"rtl838x", "realtek,rtl8382-soc\x00", true},
		{"rtl930x", "realtek,rtl9303-soc\x00", true},
		{"generico", "generic,x86\x00", false},
		{"meditek con substring ajeno", "mediatek,mt7622\x00", false},
		{"vacio", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := compatibleIsRealtekDSA(tc.content); got != tc.want {
				t.Fatalf("compatibleIsRealtekDSA(%q) = %v, want %v", tc.content, got, tc.want)
			}
		})
	}
}

// El guard de SetPhysPort debe rechazar un write de EEE cuando la
// plataforma lo marca de solo lectura, ANTES de ejecutar ethtool: en el
// hardware real ese comando reinicia el SoC (#485).
func TestSetPhysPortEEEReadOnly(t *testing.T) {
	orig := eeeWritableFn
	eeeWritableFn = func() bool { return false }
	t.Cleanup(func() { eeeWritableFn = orig })

	on := true
	err := SetPhysPort(PhysPortEdit{Name: "lo", EEE: &on})
	if err == nil {
		t.Fatal("SetPhysPort con EEE en plataforma de solo lectura debe fallar")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("error inesperado: %v", err)
	}
}
