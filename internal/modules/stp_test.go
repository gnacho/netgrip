package modules

import "testing"

func TestStpStateNames(t *testing.T) {
	cases := map[int]string{0: "disabled", 1: "blocking", 2: "listening", 3: "learning", 4: "forwarding"}
	for in, want := range cases {
		if got := stpStateName(in); got != want {
			t.Fatalf("stpStateName(%d) = %q, want %q", in, got, want)
		}
	}
	if got := stpStateName(9); got != "unknown" {
		t.Fatalf("stpStateName(9) = %q, want unknown", got)
	}
}

func TestValidateSTPBridge(t *testing.T) {
	valid := STPBridgeEdit{Priority: 32768, HelloTime: 2, MaxAge: 20, ForwardDelay: 15}
	if err := validateSTPBridge(valid); err != nil {
		t.Fatalf("valid bridge rejected: %v", err)
	}
	for name, edit := range map[string]STPBridgeEdit{
		"priority 4095":    {Priority: 4095, HelloTime: 2, MaxAge: 20, ForwardDelay: 15},
		"priority 61441":   {Priority: 61441, HelloTime: 2, MaxAge: 20, ForwardDelay: 15},
		"priority -4096":   {Priority: -4096, HelloTime: 2, MaxAge: 20, ForwardDelay: 15},
		"hello 0":          {Priority: 32768, HelloTime: 0, MaxAge: 20, ForwardDelay: 15},
		"hello 11":         {Priority: 32768, HelloTime: 11, MaxAge: 20, ForwardDelay: 15},
		"max age 5":        {Priority: 32768, HelloTime: 2, MaxAge: 5, ForwardDelay: 15},
		"max age 41":       {Priority: 32768, HelloTime: 2, MaxAge: 41, ForwardDelay: 15},
		"forward delay 3":  {Priority: 32768, HelloTime: 2, MaxAge: 20, ForwardDelay: 3},
		"forward delay 31": {Priority: 32768, HelloTime: 2, MaxAge: 20, ForwardDelay: 31},
	} {
		if err := validateSTPBridge(edit); err == nil {
			t.Fatalf("%s must be rejected", name)
		}
	}
}

func TestValidateSTPPort(t *testing.T) {
	cost := int64(5)
	prio := 32
	if err := validateSTPPort(STPPortEdit{PathCost: &cost, Priority: &prio}); err != nil {
		t.Fatalf("valid port rejected: %v", err)
	}
	badCost := int64(0)
	if err := validateSTPPort(STPPortEdit{PathCost: &badCost}); err == nil {
		t.Fatal("path cost 0 must be rejected")
	}
	bigCost := int64(200000001)
	if err := validateSTPPort(STPPortEdit{PathCost: &bigCost}); err == nil {
		t.Fatal("path cost 200000001 must be rejected")
	}
	badPrio := 20
	if err := validateSTPPort(STPPortEdit{Priority: &badPrio}); err == nil {
		t.Fatal("priority 20 (not multiple of 16) must be rejected")
	}
	highPrio := 256
	if err := validateSTPPort(STPPortEdit{Priority: &highPrio}); err == nil {
		t.Fatal("priority 256 must be rejected")
	}
}

func TestSTPPortEditOmittedFieldsStayUntouched(t *testing.T) {
	cost := int64(10)
	e := STPPortEdit{Name: "lan20", PathCost: &cost}
	if v := int64PtrString(e.PathCost); v != "10" {
		t.Fatalf("path cost = %q, want 10", v)
	}
	if v := intPtrString(e.Priority); v != "" {
		t.Fatalf("omitted priority = %q, want empty", v)
	}
	if v := boolPtrString(e.BpduGuard); v != "" {
		t.Fatalf("omitted bpdu_guard = %q, want empty", v)
	}
	on := true
	if v := boolPtrString(&on); v != "1" {
		t.Fatalf("true bool = %q, want 1", v)
	}
}

const ethtoolSample = `Settings for lan1:
	Supported ports: [ TP ]
	Supported link modes:
		10baseT/Half 10baseT/Full
		100baseT/Half 100baseT/Full
		1000baseT/Full
	Supports auto-negotiation: Yes
	Supported pause frame use: Symmetric Receive-only
	Supports Wake-on: d
	Wake-on: d
	Current message level: 0x00000007 (7)
			       drv probe link
	Link detected: yes
	Auto-negotiation: on
	Broadcast mode: disabled
	Speed: 1000Mb/s
	Duplex: Full
	Port: Twisted Pair
	PHYAD: 0
	Transceiver: internal
`

func TestParseEtoolOutput(t *testing.T) {
	autoneg, speed, duplex, modes := parseEtoolOutput(ethtoolSample)
	if !autoneg {
		t.Fatal("autoneg must be on")
	}
	if speed != 1000 || duplex != "full" {
		t.Fatalf("speed/duplex = %d/%s, want 1000/full", speed, duplex)
	}
	if len(modes) != 5 {
		t.Fatalf("modes = %+v, want 5 entries", modes)
	}
	want := LinkMode{SpeedMbps: 100, Duplex: "half"}
	if modes[2] != want {
		t.Fatalf("modes[2] = %+v, want %+v", modes[2], want)
	}
}

const ethtoolDownSample = `Settings for lan20:
	Supported link modes:
		10baseT/Half 10baseT/Full
		100baseT/Half 100baseT/Full
		1000baseT/Full
	Auto-negotiation: off
	Speed: Unknown!
	Duplex: Unknown! (255)
	Link detected: no
`

func TestParseEtoolOutputDownLink(t *testing.T) {
	autoneg, speed, duplex, modes := parseEtoolOutput(ethtoolDownSample)
	if autoneg {
		t.Fatal("autoneg must be off")
	}
	if speed != 0 || duplex != "" {
		t.Fatalf("down link speed/duplex = %d/%q, want 0/empty", speed, duplex)
	}
	if len(modes) != 5 {
		t.Fatalf("modes = %+v, want 5 entries", modes)
	}
}

const eeeSample = `EEE Settings for lan1:
	EEE status: enabled
	Tx LPI: disabled
	Supported EEE link modes: 100baseT/Full
	                 1000baseT/Full
	EEE for link partner: disabled
`

const eeeDisabledSample = `EEE Settings for lan1:
	EEE status: disabled
	Tx LPI: disabled
	Supported EEE link modes: 100baseT/Full
	                 1000baseT/Full
`

func TestParseEeeOutput(t *testing.T) {
	supported, enabled := parseEeeOutput(eeeSample)
	if !supported || !enabled {
		t.Fatalf("EEE sample: supported=%v enabled=%v, want true/true", supported, enabled)
	}
	supported, enabled = parseEeeOutput(eeeDisabledSample)
	if !supported || enabled {
		t.Fatalf("EEE disabled sample: supported=%v enabled=%v, want true/false", supported, enabled)
	}
	noModes := "EEE Settings for lan2:\n\tEEE status: disabled\n\tSupported EEE link modes: Not reported\n"
	if supported, _ = parseEeeOutput(noModes); supported {
		t.Fatal("EEE without supported modes must report unsupported")
	}
}

const eepromSample = `	Identifier		: 0x03 (SFP)
	Vendor name		: OEM
	Vendor PN		: SFP-1G-T
	Vendor SN		: ABC12345
	Date code(yymmdd)	: 230315
	Temperature		: 33.50 Celsius
	Supply voltage		: 3.30 V
	Laser bias current	: 10.00 mA
	TX power		: 0.50 mW / -3.01 dBm
	RX power		: 0.40 mW / -3.98 dBm
`

func TestParseSFPEeprom(t *testing.T) {
	info := parseSFPEeprom(eepromSample)
	if info.Vendor != "OEM" || info.PN != "SFP-1G-T" || info.SN != "ABC12345" {
		t.Fatalf("vendor/pn/sn = %q/%q/%q", info.Vendor, info.PN, info.SN)
	}
	if info.Date != "230315" {
		t.Fatalf("date = %q, want 230315", info.Date)
	}
	if info.TempC != 33.50 || info.Voltage != 3.30 || info.BiasMA != 10.00 {
		t.Fatalf("dom = %v/%v/%v", info.TempC, info.Voltage, info.BiasMA)
	}
	if info.TxPowerMW != 0.50 || info.TxPowerDBM != -3.01 {
		t.Fatalf("tx power = %v mW / %v dBm", info.TxPowerMW, info.TxPowerDBM)
	}
	if info.RxPowerMW != 0.40 || info.RxPowerDBM != -3.98 {
		t.Fatalf("rx power = %v mW / %v dBm", info.RxPowerMW, info.RxPowerDBM)
	}
}
