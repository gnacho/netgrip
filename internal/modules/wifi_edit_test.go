package modules

import "testing"

func TestWifiTargetSections(t *testing.T) {
	cases := []struct {
		name string
		edit WifiEdit
		want []string
	}{
		{"single", WifiEdit{Section: "default_radio0"}, []string{"default_radio0"}},
		{
			"band steering",
			WifiEdit{Section: "default_radio0", Sections: []string{"default_radio1"}},
			[]string{"default_radio0", "default_radio1"},
		},
		{
			"primary is not duplicated",
			WifiEdit{Section: "default_radio0", Sections: []string{"default_radio0", "default_radio1"}},
			[]string{"default_radio0", "default_radio1"},
		},
		{
			"empty entries are dropped",
			WifiEdit{Section: "default_radio0", Sections: []string{"", "default_radio1", "default_radio1"}},
			[]string{"default_radio0", "default_radio1"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := wifiTargetSections(c.edit)
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("got %v, want %v", got, c.want)
				}
			}
		})
	}
}

func TestWifiEditOpsForTargetsTheGivenSection(t *testing.T) {
	hidden := false
	ops, err := wifiEditOpsFor("default_radio1", WifiEdit{Section: "default_radio0", SSID: "Home", Hidden: &hidden})
	if err != nil {
		t.Fatal(err)
	}
	// ssid set on the requested section, hidden delete on the same one.
	var sawSSID, sawHidden bool
	for _, op := range ops {
		if op.Kind == "uci_set" && len(op.Args) == 2 && op.Args[0] == "wireless.default_radio1.ssid" && op.Args[1] == "Home" {
			sawSSID = true
		}
		if op.Kind == "uci_delete" && len(op.Args) == 1 && op.Args[0] == "wireless.default_radio1.hidden" {
			sawHidden = true
		}
		if op.Kind == "uci_commit" {
			t.Fatal("wifiEditOpsFor must not commit; SetWifi adds a single commit")
		}
	}
	if !sawSSID || !sawHidden {
		t.Fatalf("ops did not target the given section: %+v", ops)
	}
}

func TestWifiEditOpsForRejectsEmptyEdit(t *testing.T) {
	if _, err := wifiEditOpsFor("default_radio0", WifiEdit{Section: "default_radio0"}); err == nil {
		t.Fatal("an edit with no fields should be rejected")
	}
}

func TestMissingWifiSections(t *testing.T) {
	seen := map[string]bool{"default_radio0": true}
	got := missingWifiSections(seen, []string{"default_radio0", "default_radio1", "", "guest2g"})
	want := []string{"default_radio1", "guest2g"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
