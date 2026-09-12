package modules

import (
	"strings"
	"testing"
)

func TestValidDDNSSection(t *testing.T) {
	valid := []string{"a", "9", "casa_duckdns_org", "A9_z", "a-b", "a@b", "x:y"}
	for _, s := range valid {
		if !validDDNSSection(s) {
			t.Errorf("section %q should be valid", s)
		}
	}
	invalid := []string{"", "bad section", "a.b", "semi;colon", "a/b", "a\nb", "é", strings.Repeat("a", 65)}
	for _, s := range invalid {
		if validDDNSSection(s) {
			t.Errorf("section %q should be invalid", s)
		}
	}
}

func TestForceDDNSUpdateInvalidSection(t *testing.T) {
	_, err := ForceDDNSUpdate("bad section!")
	if err == nil || !strings.Contains(err.Error(), "invalid section name") {
		t.Fatalf("expected invalid section name error, got %v", err)
	}
}

func TestForceDDNSUpdateMissingSection(t *testing.T) {
	orig := ddnsSectionExists
	ddnsSectionExists = func(string) bool { return false }
	defer func() { ddnsSectionExists = orig }()

	_, err := ForceDDNSUpdate("valid_section")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}
