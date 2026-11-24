package modules

import (
	"strings"
	"testing"
)

// El registro del executor tiene que hablar por el mismo transporte con pin
// SPKI que el agente; con un cliente por defecto un servidor con CA privada
// se rechazaba aunque la ingesta funcionara (#606).
func TestExecutorHTTPClientUsesThePinnedTransport(t *testing.T) {
	pinned := executorHTTPClient("https://192.0.2.10:8443", strings.Repeat("ab", 32))
	if pinned.Transport == nil {
		t.Fatal("con un pin válido debe usar el transporte con pinning")
	}

	// Sin pin utilizable cae al cliente por defecto en vez de romper.
	plain := executorHTTPClient("https://192.0.2.10:8443", "no-es-un-pin")
	if plain.Transport != nil {
		t.Fatal("sin pin válido debe usar el transporte por defecto")
	}
}
