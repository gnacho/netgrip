package modules

import (
	"testing"

	"github.com/gnacho/netgrip/internal/ubus"
)

func TestValidateRadioEdit(t *testing.T) {
	cases := []struct {
		name string
		edit RadioEdit
		ok   bool
	}{
		{"valid channel", RadioEdit{Radio: "radio0", Channel: "6", Htmode: "HE40"}, true},
		{"channel auto", RadioEdit{Radio: "radio0", Channel: "auto"}, true},
		{"channel 5g", RadioEdit{Radio: "radio1", Channel: "36", Htmode: "HE80"}, true},
		{"bad channel", RadioEdit{Radio: "radio0", Channel: "abc"}, false},
		{"bad htmode", RadioEdit{Radio: "radio0", Htmode: "FOO"}, false},
		{"no radio", RadioEdit{Channel: "6"}, false},
		{"nothing to edit", RadioEdit{Radio: "radio0"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateRadioEdit(c.edit)
			if c.ok && err != nil {
				t.Fatalf("expected ok, got %v", err)
			}
			if !c.ok && err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestRadioHealthy(t *testing.T) {
	radio := &ubus.WirelessRadio{Name: "radio0", Up: true, Channel: "6"}
	if !radioHealthy(RadioEdit{Radio: "radio0", Channel: "6"}, radio) {
		t.Fatal("expected healthy on requested channel")
	}
	if radioHealthy(RadioEdit{Radio: "radio0", Channel: "11"}, radio) {
		t.Fatal("expected unhealthy on channel mismatch")
	}
	if radioHealthy(RadioEdit{Radio: "radio0", Channel: "6"}, &ubus.WirelessRadio{Up: false}) {
		t.Fatal("expected unhealthy when down")
	}
	if !radioHealthy(RadioEdit{Radio: "radio0"}, radio) {
		t.Fatal("expected healthy when only htmode changed")
	}
}
