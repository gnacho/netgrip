package modules

import "testing"

func TestParsePartnerMax(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   int
	}{
		{
			// Salida real de lan4 en un Xiaomi AX9000 con un SLZB-06M, que
			// solo hace 10/100 (#612).
			name: "peer de 100 en boca de 1G",
			output: `Settings for lan4:
	Supported ports: [ TP	 MII ]
	Supported link modes:   10baseT/Half 10baseT/Full
	                        100baseT/Half 100baseT/Full
	                        1000baseT/Full
	Speed: 100Mb/s
	Duplex: Full
	Auto-negotiation: on
	Link partner advertised link modes:  10baseT/Half 10baseT/Full
	                                     100baseT/Half 100baseT/Full
	Link partner advertised pause frame use: No
	Link partner advertised auto-negotiation: Yes
	Link detected: yes
`,
			want: 100,
		},
		{
			name: "peer que anuncia 1G y negocia 100",
			output: `	Speed: 100Mb/s
	Link partner advertised link modes:  10baseT/Half 10baseT/Full
	                                     100baseT/Half 100baseT/Full
	                                     1000baseT/Full
	Link partner advertised pause frame use: No
`,
			want: 1000,
		},
		{
			name: "driver sin datos del peer",
			output: `	Speed: 100Mb/s
	Link partner advertised link modes:  Not reported
	Link partner advertised pause frame use: No
`,
			want: 0,
		},
		{
			name: "sin bloque de peer",
			output: `	Speed: 100Mb/s
	Duplex: Full
`,
			want: 0,
		},
		{
			name: "variante en singular",
			output: `	Link partner advertised link mode: 100baseT/Full
	Speed: 100Mb/s
`,
			want: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parsePartnerMax(tt.output); got != tt.want {
				t.Fatalf("parsePartnerMax() = %d, want %d", got, tt.want)
			}
		})
	}
}
