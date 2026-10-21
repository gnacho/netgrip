package modules

import (
	"errors"
	"testing"
)

var errTestSnapshot = errors.New("create failed")

// #553: CreateAndPushSnapshot crea un snapshot fresco y empuja el último.
// Los pasos son inyectables: sin OpenWrt (sin uci) el create real fallaría.
func TestCreateAndPushSnapshot(t *testing.T) {
	oldCreate, oldPush := createSnapshotFn, pushLatestFn
	t.Cleanup(func() { createSnapshotFn, pushLatestFn = oldCreate, oldPush })

	t.Run("create falla y no empuja", func(t *testing.T) {
		createSnapshotFn = func() (*ConfigSnapshot, error) { return nil, errTestSnapshot }
		pushed := false
		pushLatestFn = func() PushResult { pushed = true; return PushResult{Ok: true} }
		res := CreateAndPushSnapshot()
		if res.Ok || pushed {
			t.Fatalf("create falló: got %+v pushed=%v", res, pushed)
		}
	})

	t.Run("create ok y empuja el resultado", func(t *testing.T) {
		createSnapshotFn = func() (*ConfigSnapshot, error) {
			return &ConfigSnapshot{ID: "20261021-000000"}, nil
		}
		pushLatestFn = func() PushResult { return PushResult{Ok: true, SnapshotID: "20261021-000000"} }
		res := CreateAndPushSnapshot()
		if !res.Ok || res.SnapshotID != "20261021-000000" {
			t.Fatalf("ok: got %+v", res)
		}
	})

	t.Run("el push fallo se propaga", func(t *testing.T) {
		createSnapshotFn = func() (*ConfigSnapshot, error) { return &ConfigSnapshot{ID: "x"}, nil }
		pushLatestFn = func() PushResult { return PushResult{Error: "push failed"} }
		res := CreateAndPushSnapshot()
		if res.Ok || res.Error != "push failed" {
			t.Fatalf("push falló: got %+v", res)
		}
	})
}
