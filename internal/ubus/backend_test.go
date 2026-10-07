package ubus

import (
	"encoding/json"
	"testing"
)

type fakeBackend struct {
	calls int
	info  *SystemInfo
	err   error
}

func (f *fakeBackend) SystemInfo() (*SystemInfo, error) {
	f.calls++
	return f.info, f.err
}

func (f *fakeBackend) WanStatus() (*WanStatus, error) { return nil, nil }

func (f *fakeBackend) WirelessStatus() ([]WirelessRadio, error) { return nil, nil }
func (f *fakeBackend) SystemBoard() (json.RawMessage, error) { return nil, nil }

// SetBackendForTest must swap the backend for the test and restore the real
// one on cleanup, so suites run in any order.
func TestSetBackendForTestSwapsAndRestores(t *testing.T) {
	if _, ok := DefaultBackend.(realBackend); !ok {
		t.Fatalf("DefaultBackend = %T, want realBackend before the swap", DefaultBackend)
	}
	fake := &fakeBackend{info: &SystemInfo{Uptime: 42}}
	SetBackendForTest(t, fake)
	if _, ok := DefaultBackend.(*fakeBackend); !ok {
		t.Fatalf("DefaultBackend = %T, want *fakeBackend after the swap", DefaultBackend)
	}
	if info, err := DefaultBackend.SystemInfo(); err != nil || info.Uptime != 42 {
		t.Errorf("SystemInfo() = %v, %v; want uptime 42, nil error", info, err)
	}
	if fake.calls != 1 {
		t.Errorf("fake.calls = %d, want 1", fake.calls)
	}
}
