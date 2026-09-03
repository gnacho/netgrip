package modules

import (
	"encoding/json"
	"testing"
)

const ptFixture = `netgrip.main='main'
netgrip.main.port='8090'
netgrip.hardened=port_template
netgrip.hardened.description='Disable unused ports'
netgrip.hardened.admin_up='0'
netgrip.hardened.speed_mbps='1000'
netgrip.hardened.vlans_json='[{"vid":1,"tagged":false}]'
netgrip.@port_template[0]=port_template
netgrip.@port_template[0].description='IoT profile'
netgrip.@port_template[0].admin_up='1'
`

func TestListPortTemplatesFromRealUCIShape(t *testing.T) {
	templates := listPortTemplatesFrom(ptFixture)
	if len(templates) != 2 {
		t.Fatalf("want 2 templates, got %d: %+v", len(templates), templates)
	}
	var hardened *PortTemplate
	for i := range templates {
		if templates[i].Name == "hardened" {
			hardened = &templates[i]
		}
	}
	if hardened == nil {
		t.Fatalf("named section template not parsed: %+v", templates)
	}
	if hardened.Description != "Disable unused ports" || hardened.AdminUp || hardened.SpeedMbps != 1000 {
		t.Errorf("hardened fields wrong: %+v", hardened)
	}
	if len(hardened.VLANs) != 1 || hardened.VLANs[0].VID != 1 {
		t.Errorf("vlans_json not parsed: %+v", hardened.VLANs)
	}
}

func TestListPortTemplatesFromEmptyNeverNil(t *testing.T) {
	empty := "netgrip.main='main'\nnetgrip.main.port='8090'\n"
	got := listPortTemplatesFrom(empty)
	if got == nil {
		t.Fatal("got nil slice on config without templates")
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "[]" {
		t.Errorf("empty config must marshal to [], got %s", b)
	}
}
