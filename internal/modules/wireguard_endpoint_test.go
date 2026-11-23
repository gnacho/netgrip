package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// El nombre externo que va en las configs de cliente: un dominio sobrevive a
// un cambio de IP del operador, una IP no (#629).
func TestSetWGEndpointNormalizes(t *testing.T) {
	dir := t.TempDir()
	orig := wgEndpointFile
	wgEndpointFile = filepath.Join(dir, "wg_endpoint")
	t.Cleanup(func() { wgEndpointFile = orig })

	cases := map[string]string{
		"casa.midominio.net":          "casa.midominio.net",
		"https://casa.midominio.net/": "casa.midominio.net",
		"casa.midominio.net:51820":    "casa.midominio.net",
		"  vpn.example.org  ":         "vpn.example.org",
	}
	for in, want := range cases {
		if _, err := SetWGEndpoint(in); err != nil {
			t.Fatalf("SetWGEndpoint(%q): %v", in, err)
		}
		data, err := os.ReadFile(wgEndpointFile)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if got := strings.TrimSpace(string(data)); got != want {
			t.Fatalf("SetWGEndpoint(%q) guardo %q, want %q", in, got, want)
		}
	}

	if _, err := SetWGEndpoint("esto no vale"); err == nil {
		t.Fatal("un nombre con espacios deberia rechazarse")
	}

	if _, err := SetWGEndpoint(""); err != nil {
		t.Fatalf("vaciar: %v", err)
	}
	if _, err := os.Stat(wgEndpointFile); !os.IsNotExist(err) {
		t.Fatalf("vacio debe borrar el fichero, stat err=%v", err)
	}
}
