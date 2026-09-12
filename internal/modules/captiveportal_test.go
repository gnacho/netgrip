package modules

import (
	"strings"
	"testing"
)

func TestValidateCaptivePortal(t *testing.T) {
	valid := CaptivePortalConfig{
		Enabled:        true,
		Title:          "Welcome to Casa Garcia",
		Message:        "Enjoy your stay.",
		AccessCode:     "guest-2026",
		SessionMinutes: 120,
	}
	if err := validateCaptivePortal(valid); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*CaptivePortalConfig)
		want   string
	}{
		{"title too long", func(c *CaptivePortalConfig) { c.Title = strings.Repeat("x", cpTitleMax+1) }, "title"},
		{"message too long", func(c *CaptivePortalConfig) { c.Message = strings.Repeat("x", cpMessageMax+1) }, "message"},
		{"code too short", func(c *CaptivePortalConfig) { c.AccessCode = "abc" }, "access code"},
		{"code too long", func(c *CaptivePortalConfig) { c.AccessCode = strings.Repeat("a", cpCodeMax+1) }, "access code"},
		{"code bad charset", func(c *CaptivePortalConfig) { c.AccessCode = "guest code" }, "access code"},
		{"negative session", func(c *CaptivePortalConfig) { c.SessionMinutes = -1 }, "session duration"},
		{"session too long", func(c *CaptivePortalConfig) { c.SessionMinutes = cpSessionMax + 1 }, "session duration"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := valid
			tc.mutate(&c)
			err := validateCaptivePortal(c)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %q", tc.want, err.Error())
			}
		})
	}
}

func TestValidateCaptivePortalEmptyCodeAndNeverSession(t *testing.T) {
	if err := validateCaptivePortal(CaptivePortalConfig{Enabled: true, SessionMinutes: 0}); err != nil {
		t.Fatalf("empty code + never session should be valid: %v", err)
	}
}

func TestEscapeHTML(t *testing.T) {
	got := escapeHTML(`<a href="x">&$5</a>`)
	want := `&lt;a href=&#34;x&#34;&gt;&amp;&#36;5&lt;/a&gt;`
	if got != want {
		t.Fatalf("escapeHTML = %q, want %q", got, want)
	}
}

func TestSplashHTML(t *testing.T) {
	html := splashHTML("Casa Garcia", "Welcome <b>guests</b> - $5 off", "png", true)

	if !strings.Contains(html, `action="$authaction"`) {
		t.Error("splash page must submit to $authaction")
	}
	if !strings.Contains(html, `name="tok" value="$tok"`) {
		t.Error("splash page must carry the tok token")
	}
	if !strings.Contains(html, `name="password"`) {
		t.Error("splash page must include a password field when a code is required")
	}
	if !strings.Contains(html, `src="/images/logo.png"`) {
		t.Error("splash page must reference the logo image")
	}
	// User text must be escaped, and '$' neutralized so nodogsplash does not
	// treat it as a template variable.
	if strings.Contains(html, "<b>") || !strings.Contains(html, "&lt;b&gt;") {
		t.Errorf("message not HTML-escaped: %s", html)
	}
	if !strings.Contains(html, "&#36;5") {
		t.Errorf("expected '$' to be escaped: %s", html)
	}
}

func TestSplashHTMLNoCodeNoImage(t *testing.T) {
	html := splashHTML("", "hello", "", false)
	if strings.Contains(html, `name="password"`) {
		t.Error("no password field expected when no code is required")
	}
	if strings.Contains(html, "<img") {
		t.Error("no image expected when ext is empty")
	}
	if !strings.Contains(html, "hello") {
		t.Error("message should be present")
	}
}

func TestValidImageExt(t *testing.T) {
	for _, ok := range []string{"png", "PNG", "jpg", "jpeg", "gif", "webp", " JPG "} {
		if !validImageExt(ok) {
			t.Errorf("validImageExt(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"svg", "html", "exe", ""} {
		if validImageExt(bad) {
			t.Errorf("validImageExt(%q) = true, want false", bad)
		}
	}
}

func TestNdsAuthScript(t *testing.T) {
	if !strings.Contains(ndsAuthScript, "auth_client") {
		t.Error("binauth script must handle auth_client")
	}
	if !strings.Contains(ndsAuthScript, "exit 1") {
		t.Error("binauth script must reject with a non-zero exit")
	}
	if strings.Contains(ndsAuthScript, "echo 0") {
		t.Error("binauth script must not echo 0 to reject (nodogsplash v5 treats 0 as unlimited)")
	}
}
