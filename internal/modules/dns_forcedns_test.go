package modules

import (
	"strings"
	"testing"

	"github.com/gnacho/netgrip/internal/executor"
)

func TestParseDHCPOptionShow(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []string
	}{
		{name: "single option", line: "dhcp.lan.dhcp_option='3,192.168.1.1'", want: []string{"3,192.168.1.1"}},
		{name: "multiple options with force dns", line: "dhcp.lan.dhcp_option='3,192.168.1.1' '6,192.168.1.1'", want: []string{"3,192.168.1.1", "6,192.168.1.1"}},
		{name: "empty output", line: "", want: nil},
		{name: "no equals sign", line: "garbage", want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseDHCPOptionShow(tc.line)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestDHCPHasForceDNS(t *testing.T) {
	values := []string{"3,192.168.1.1", "6,192.168.1.1"}
	if !dhcpHasForceDNS(values, "192.168.1.1") {
		t.Error("must detect the 6,192.168.1.1 entry")
	}
	if dhcpHasForceDNS(values, "192.168.1.254") {
		t.Error("must not match a different router IP")
	}
	if dhcpHasForceDNS([]string{"3,192.168.1.1"}, "192.168.1.1") {
		t.Error("must be false when no 6, entry is present")
	}
	if dhcpHasForceDNS(nil, "192.168.1.1") {
		t.Error("must be false for an empty list")
	}
}

func TestPlanForceDNSOpsEnable(t *testing.T) {
	ops := planForceDNSOps([]string{"3,192.168.1.1", "6,8.8.8.8"}, "192.168.1.1", true)
	want := []executor.Op{
		{Kind: "uci_del_list", Args: []string{"dhcp.lan.dhcp_option", "6,8.8.8.8"}},
		{Kind: "uci_add_list", Args: []string{"dhcp.lan.dhcp_option", "6,192.168.1.1"}},
	}
	if len(ops) != len(want) {
		t.Fatalf("want %d ops, got %d: %v", len(want), len(ops), ops)
	}
	for i, op := range ops {
		if op.Kind != want[i].Kind || strings.Join(op.Args, "|") != strings.Join(want[i].Args, "|") {
			t.Errorf("op %d = %v %v, want %v %v", i, op.Kind, op.Args, want[i].Kind, want[i].Args)
		}
		if err := executor.Validate(op); err != nil {
			t.Errorf("op %d fails validation: %v", i, err)
		}
	}
}

func TestPlanForceDNSOpsDisable(t *testing.T) {
	ops := planForceDNSOps([]string{"3,192.168.1.1", "6,192.168.1.1"}, "192.168.1.1", false)
	want := []executor.Op{
		{Kind: "uci_del_list", Args: []string{"dhcp.lan.dhcp_option", "6,192.168.1.1"}},
	}
	if len(ops) != len(want) {
		t.Fatalf("want %d ops, got %d: %v", len(want), len(ops), ops)
	}
	for i, op := range ops {
		if op.Kind != want[i].Kind || strings.Join(op.Args, "|") != strings.Join(want[i].Args, "|") {
			t.Errorf("op %d = %v %v, want %v %v", i, op.Kind, op.Args, want[i].Kind, want[i].Args)
		}
		if err := executor.Validate(op); err != nil {
			t.Errorf("op %d fails validation: %v", i, err)
		}
	}
}
