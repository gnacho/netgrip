package modules

import "testing"

func TestSectionsForBand(t *testing.T) {
	bands := map[string]string{"default_radio0": "5g", "default_radio0_network1": "5g", "default_radio1": "2g", "wifinet1": "2g"}
	bandOf := func(section string) string { return bands[section] }
	all := []string{"default_radio0", "default_radio0_network1", "default_radio1", "wifinet1"}

	t.Run("empty band returns every section", func(t *testing.T) {
		got, err := sectionsForBand(all, "", bandOf)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != len(all) {
			t.Fatalf("want %d sections, got %d", len(all), len(got))
		}
	})

	t.Run("filters to the requested band", func(t *testing.T) {
		got, err := sectionsForBand(all, "5g", bandOf)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"default_radio0", "default_radio0_network1"}
		if len(got) != len(want) {
			t.Fatalf("want %v, got %v", want, got)
		}
	})

	t.Run("unknown band is an error", func(t *testing.T) {
		if _, err := sectionsForBand(all, "6g", bandOf); err == nil {
			t.Fatal("want error for a band no radio serves")
		}
	})
}
