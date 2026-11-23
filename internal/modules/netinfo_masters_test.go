package modules

import "testing"

func TestLinkMasters(t *testing.T) {
	physical := func(name string) bool {
		switch name {
		case "wan", "lan1", "lan2", "lan3", "lan4", "eth0":
			return true
		}
		return false
	}

	tests := []struct {
		name string
		link string
		want map[string]bool
	}{
		{
			name: "vlan on a user port does not hide the port",
			link: `15: wan.20@wan: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP mode DEFAULT group default qlen 1000
7: wan: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000`,
			want: map[string]bool{},
		},
		{
			name: "dsa port marks the soc uplink",
			link: `4: lan1@eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue master br-lan state UP mode DEFAULT group default qlen 1000`,
			want: map[string]bool{"eth0": true},
		},
		{
			name: "plain links mark nothing",
			link: `3: lan4: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq master br-lan state UP mode DEFAULT group default qlen 1000`,
			want: map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := linkMasters(tt.link, physical)
			if len(got) != len(tt.want) {
				t.Fatalf("linkMasters() = %v, want %v", got, tt.want)
			}
			for name := range tt.want {
				if !got[name] {
					t.Fatalf("linkMasters() = %v, missing %q", got, name)
				}
			}
		})
	}
}
