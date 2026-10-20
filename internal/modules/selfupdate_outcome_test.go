package modules

import (
	"testing"
)

// El hook de resultado es one-shot: el primer estado terminal lo consume y
// los siguientes no llegan a ningún sitio (#549). Así un self-update
// posterior (panel o scheduler) nunca reporta a NetPulse un resultado que
// NetPulse no pidió.
func TestReportSelfUpdateOutcomeOneShot(t *testing.T) {
	calls := 0
	var lastOk bool
	var lastMsg string
	setSelfUpdateOutcomeHook(func(ok bool, errMsg string) {
		calls++
		lastOk, lastMsg = ok, errMsg
	})
	t.Cleanup(func() { setSelfUpdateOutcomeHook(nil) })

	reportSelfUpdateOutcome(false, "no space")
	reportSelfUpdateOutcome(true, "")
	reportSelfUpdateOutcome(false, "otro")

	if calls != 1 {
		t.Fatalf("hook llamado %d veces, want 1", calls)
	}
	if lastOk {
		t.Fatal("primer resultado debía ser ok=false")
	}
	if lastMsg != "no space" {
		t.Fatalf("mensaje: %q", lastMsg)
	}
}

// Sin hook armado, reportSelfUpdateOutcome no hace nada (nil-safe): los
// self-update iniciados desde el panel no tienen por qué llevar hook.
func TestReportSelfUpdateOutcomeNoHook(t *testing.T) {
	reportSelfUpdateOutcome(true, "")
}

// failUpdate (el camino de todos los fallos de runSelfUpdate) entrega el
// motivo real al hook: es lo que convierte "el botón no hace nada" en el
// error visible en NetPulse (#549).
func TestFailUpdateReportsOutcome(t *testing.T) {
	var got bool
	var msg string
	setSelfUpdateOutcomeHook(func(ok bool, errMsg string) {
		got = true
		msg = errMsg
	})
	t.Cleanup(func() {
		setSelfUpdateOutcomeHook(nil)
		setUpdateStatus("idle", 0, "")
	})

	failUpdate("install: not enough space (need ~%d bytes)", 1234)

	if !got {
		t.Fatal("failUpdate no reportó el outcome al hook")
	}
	want := "install: not enough space (need ~1234 bytes)"
	if msg != want {
		t.Fatalf("mensaje %q, want %q", msg, want)
	}
	if st := GetSelfUpdateStatus(); st.Phase != "error" {
		t.Fatalf("fase %q, want error", st.Phase)
	}
}
