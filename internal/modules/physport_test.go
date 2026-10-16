package modules

import (
	"errors"
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

// La clasificacion de `ethtool -m` decide si una boca muestra la seccion SFP
// (#535): sin binario en el target la salida llega vacia y el cobre corriente
// dice "Not supported" - en ambos casos no hay seccion. Solo un fallo de
// lectura de verdad la muestra. Casos reales: salida vacia = rt-lab (sin
// ethtool), "Not supported" = rt3 (boca RJ45 con ethtool).
func TestEtoolModuleClassifiesFailures(t *testing.T) {
	orig := ethtoolModuleOutputFn
	t.Cleanup(func() { ethtoolModuleOutputFn = orig })

	cases := []struct {
		name      string
		out       string
		wantState string // "" = nil: sin seccion
	}{
		{name: "sin ethtool (salida vacia)", out: "", wantState: ""},
		{name: "cobre not supported", out: "netlink error: Not supported\n", wantState: ""},
		{name: "jaula sin modulo", out: "Cannot get module EEPROM information: No such device\n", wantState: "empty"},
		{name: "fallo inesperado", out: "netlink error: boom\n", wantState: "error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ethtoolModuleOutputFn = func(string) ([]byte, error) {
				return []byte(tc.out), errors.New("exit status 1")
			}
			got := ethtoolModule("lan1")
			if tc.wantState == "" {
				if got != nil {
					t.Fatalf("ethtoolModule = %+v, want nil (sin seccion SFP)", got)
				}
				return
			}
			if got == nil || got.State != tc.wantState {
				t.Fatalf("ethtoolModule = %+v, want state %q", got, tc.wantState)
			}
		})
	}
}

// execErrDetail no puede soltar un detalle vacio cuando el binario falta
// (#535): explica con el error de ejecucion.
func TestExecErrDetailNeverEmpty(t *testing.T) {
	got := execErrDetail(nil, errors.New(`exec: "ethtool": executable file not found in $PATH`))
	if !strings.Contains(got, "not found") {
		t.Fatalf("execErrDetail sin salida = %q, want el error de ejecucion", got)
	}
	if got := execErrDetail([]byte("  Operation not permitted\n"), errors.New("exit status 1")); got != "Operation not permitted" {
		t.Fatalf("execErrDetail con salida = %q, want el mensaje del driver", got)
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
