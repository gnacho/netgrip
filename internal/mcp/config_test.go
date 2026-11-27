package mcp

import (
	"strings"
	"testing"
)

// stubUCI replaces the uci variables and returns the writes recorder.
func stubUCIWrites(t *testing.T, values map[string]string, failSet string) *[]string {
	t.Helper()
	oldGet, oldSet, oldCommit := uciGet, uciSet, uciCommit
	t.Cleanup(func() { uciGet, uciSet, uciCommit = oldGet, oldSet, oldCommit })
	writes := []string{}
	uciGet = func(key string) string { return values[key] }
	uciSet = func(kv string) error {
		writes = append(writes, kv)
		if kv == failSet {
			return errBadSet
		}
		return nil
	}
	uciCommit = func(pkg string) error {
		writes = append(writes, "commit:"+pkg)
		return nil
	}
	return &writes
}

type uciError struct{ msg string }

func (e *uciError) Error() string { return e.msg }

var errBadSet = &uciError{msg: "uci set rejected"}

func TestReadSettingsDefaultsWhenNothingIsConfigured(t *testing.T) {
	stubUCIWrites(t, map[string]string{}, "")
	got := ReadSettings()
	if got.Enabled || got.Token != "" || got.RatePerMin != defaultRatePerMin {
		t.Fatalf("fresh install should be disabled with the default rate, got %+v", got)
	}
}

func TestReadSettingsReadsTheThreeKeys(t *testing.T) {
	stubUCIWrites(t, map[string]string{
		uciEnabled:    "1",
		uciToken:      "abcdef0123456789",
		uciRatePerMin: "120",
	}, "")
	got := ReadSettings()
	if !got.Enabled || got.Token != "abcdef0123456789" || got.RatePerMin != 120 {
		t.Fatalf("unexpected settings: %+v", got)
	}
}

func TestReadSettingsIgnoresAnOutOfRangeRate(t *testing.T) {
	for _, rate := range []string{"0", "-5", "5000", "abc"} {
		stubUCIWrites(t, map[string]string{uciRatePerMin: rate}, "")
		if got := ReadSettings(); got.RatePerMin != defaultRatePerMin {
			t.Fatalf("rate %q should fall back to the default, got %d", rate, got.RatePerMin)
		}
	}
}

func TestSaveSettingsWritesAllKeysAndCommits(t *testing.T) {
	writes := stubUCIWrites(t, map[string]string{}, "")
	got, err := SaveSettings(Settings{Enabled: true, Token: "  0123456789abcdef  ", RatePerMin: 120})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Token != "0123456789abcdef" {
		t.Fatalf("token should be trimmed, got %q", got.Token)
	}
	want := []string{
		"netgrip.mcp=mcp",
		"netgrip.mcp.enabled=1",
		"netgrip.mcp.rate_per_min=120",
		"netgrip.mcp.token=0123456789abcdef",
		"commit:netgrip",
	}
	if len(*writes) != len(want) {
		t.Fatalf("writes = %v, want %v", *writes, want)
	}
	for i := range want {
		if (*writes)[i] != want[i] {
			t.Fatalf("writes = %v, want %v", *writes, want)
		}
	}
}

func TestSaveSettingsRefusesToEnableWithoutAToken(t *testing.T) {
	writes := stubUCIWrites(t, map[string]string{}, "")
	if _, err := SaveSettings(Settings{Enabled: true, Token: "short"}); err == nil {
		t.Fatal("enabling with a short token must fail")
	}
	if len(*writes) != 0 {
		t.Fatalf("nothing should be written on validation failure, got %v", *writes)
	}
}

func TestSaveSettingsRefusesATokenWithSpaces(t *testing.T) {
	stubUCIWrites(t, map[string]string{}, "")
	if _, err := SaveSettings(Settings{Enabled: true, Token: "0123456789 abcdef"}); err == nil {
		t.Fatal("spaces in the token must fail")
	}
}

func TestSaveSettingsDisablingKeepsTheStoredToken(t *testing.T) {
	writes := stubUCIWrites(t, map[string]string{}, "")
	got, err := SaveSettings(Settings{Enabled: false, Token: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.RatePerMin != defaultRatePerMin {
		t.Fatalf("zero rate should fall back to the default, got %d", got.RatePerMin)
	}
	for _, kv := range *writes {
		if strings.HasPrefix(kv, "netgrip.mcp.token=") {
			t.Fatalf("an empty token must not overwrite the stored one, got %v", *writes)
		}
	}
}

func TestSaveSettingsClampsTheRate(t *testing.T) {
	cases := map[int]int{1: minRatePerMin, 5000: maxRatePerMin, 45: 45}
	for in, want := range cases {
		stubUCIWrites(t, map[string]string{}, "")
		got, err := SaveSettings(Settings{Enabled: false, Token: "", RatePerMin: in})
		if err != nil {
			t.Fatalf("rate %d: %v", in, err)
		}
		if got.RatePerMin != want {
			t.Fatalf("rate %d clamped to %d, want %d", in, got.RatePerMin, want)
		}
	}
}

func TestNewTokenIsLongHexAndUnique(t *testing.T) {
	a, err := NewToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := NewToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(a) != 64 {
		t.Fatalf("token should be 64 hex chars, got %d", len(a))
	}
	for _, r := range a {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Fatalf("token is not hex: %q", a)
		}
	}
	if a == b {
		t.Fatal("two tokens came out identical")
	}
}

func TestToolCountMatchesTheRegisteredTools(t *testing.T) {
	s := New(Deps{Version: "test"})
	if s.ToolCount() != 10 {
		t.Fatalf("ToolCount = %d, want 10", s.ToolCount())
	}
}
