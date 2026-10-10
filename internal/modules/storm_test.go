package modules

import (
	"errors"
	"strings"
	"testing"

	"github.com/gnacho/netgrip/internal/executor"
)

// stubStormTc fija las costuras de ensureTc: lookPath controla que binarios
// "existen" (tc, apk) y install registra las ops pkg_add ejecutadas.
// tcAppearsAfterInstall simula si los paquetes aportan el binario tc.
func stubStormTc(t *testing.T, hasTc, hasApk bool, installErr error, tcAppearsAfterInstall bool) *[]executor.Op {
	t.Helper()
	installed := []executor.Op{}
	tcPresent := hasTc
	oldLook, oldInstall, oldCheck := stormLookPath, stormInstallPkg, stormTcCheck
	t.Cleanup(func() {
		stormLookPath = oldLook
		stormInstallPkg = oldInstall
		stormTcCheck = oldCheck
	})
	stormLookPath = func(name string) (string, error) {
		if name == "tc" && tcPresent {
			return "/usr/sbin/tc", nil
		}
		if name == "apk" && hasApk {
			return "/sbin/apk", nil
		}
		return "", errors.New("not found")
	}
	stormTcCheck = func() error {
		if tcPresent {
			return nil
		}
		return errors.New("applet not found")
	}
	stormInstallPkg = func(op executor.Op) error {
		installed = append(installed, op)
		if installErr == nil && tcAppearsAfterInstall {
			tcPresent = true
		}
		return installErr
	}
	return &installed
}

func TestEnsureTcAlreadyInstalled(t *testing.T) {
	installed := stubStormTc(t, true, true, nil, false)

	if err := ensureTc(); err != nil {
		t.Fatalf("ensureTc con tc presente debe ser no-op: %v", err)
	}
	if len(*installed) != 0 {
		t.Fatalf("no debe instalar nada con tc presente, ops: %v", *installed)
	}
}

func TestEnsureTcInstallsAllPackages(t *testing.T) {
	// Mismo conjunto en apk y opkg (verificado en el LGS352C, #489): tc
	// (virtual que resuelve tc-tiny), kmod-sched (em_cmp) y
	// kmod-sched-act-police (la accion police).
	for _, hasApk := range []bool{true, false} {
		installed := stubStormTc(t, false, hasApk, nil, true)

		if err := ensureTc(); err != nil {
			t.Fatalf("ensureTc (apk=%v) tras instalar: %v", hasApk, err)
		}
		if len(*installed) != 1 || (*installed)[0].Kind != "pkg_add" {
			t.Fatalf("se esperaba un unico pkg_add (apk=%v), ops: %v", hasApk, *installed)
		}
		want := strings.Join(tcPackages, " ")
		if got := strings.Join((*installed)[0].Args, " "); got != want {
			t.Fatalf("paquetes (apk=%v): se pidieron %q, se esperaban %q", hasApk, got, want)
		}
	}
}

func TestEnsureTcFallsBackToOpkgSet(t *testing.T) {
	// opkg <= 24.10: kmod-sched-act-police no existe (act_police viaja
	// dentro de kmod-sched). El primer pkg_add falla, el fallback cubre.
	installed := []executor.Op{}
	tcPresent := false
	oldLook, oldInstall, oldCheck := stormLookPath, stormInstallPkg, stormTcCheck
	t.Cleanup(func() {
		stormLookPath = oldLook
		stormInstallPkg = oldInstall
		stormTcCheck = oldCheck
	})
	stormLookPath = func(name string) (string, error) {
		if name == "tc" && tcPresent {
			return "/usr/sbin/tc", nil
		}
		return "", errors.New("not found")
	}
	stormTcCheck = func() error {
		if tcPresent {
			return nil
		}
		return errors.New("applet not found")
	}
	firstTry := true
	stormInstallPkg = func(op executor.Op) error {
		installed = append(installed, op)
		if firstTry {
			firstTry = false
			return errors.New("unknown package: kmod-sched-act-police")
		}
		tcPresent = true
		return nil
	}

	if err := ensureTc(); err != nil {
		t.Fatalf("ensureTc con fallback opkg debe funcionar: %v", err)
	}
	if len(installed) != 2 {
		t.Fatalf("se esperaban 2 intentos (completo + fallback), ops: %v", installed)
	}
	if got := strings.Join(installed[1].Args, " "); got != strings.Join(tcPackagesOpkg, " ") {
		t.Fatalf("fallback: se pidieron %q, se esperaban %q", got, tcPackagesOpkg)
	}
}

func TestEnsureTcInstallFailureIsActionable(t *testing.T) {
	stubStormTc(t, false, true, errors.New("network unreachable"), false)

	err := ensureTc()
	if err == nil {
		t.Fatal("debe fallar si la instalacion falla")
	}
	msg := err.Error()
	for _, want := range []string{"kmod-sched-act-police", "manually"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("el error debe ser accionable (falta %q): %s", want, msg)
		}
	}
}

func TestEnsureTcStillMissingAfterInstall(t *testing.T) {
	// La instalacion "tuvo exito" pero tc sigue sin aparecer: el pkg_add
	// salio bien pero el paquete no aporta el binario (scriptlets rotos,
	// feed inconsistente). Debe avisar, no aplicar a ciegas. El stub con
	// tcAppearsAfterInstall=false deja tc ausente tras instalar.
	stubStormTc(t, false, true, nil, false)

	err := ensureTc()
	if err == nil {
		t.Fatal("debe fallar si tc sigue ausente tras instalar")
	}
	if !strings.Contains(err.Error(), "still not in PATH") {
		t.Fatalf("error inesperado: %v", err)
	}
}

func TestParseTcRateUnits(t *testing.T) {
	// `tc filter show` imprime "rate 50Mbit" aunque se configure 50000kbit
	// (verificado en el LGS352C). Los kbps del probe deben seguir en kbit.
	// La linea de cabecera del filtro simula el formato real (parseTcRate
	// solo mira lineas i>0, donde vive el police en la salida de tc).
	show := "filter parent ffff: protocol all pref 1 u32 chain 0 \n police 0x1 rate 50Mbit burst 32Kb mtu 2Kb action drop overhead 0b \n"
	if got := parseTcRate(show, "broadcast"); got != 50000 {
		t.Fatalf("50Mbit debe parsearse como 50000 kbit, se obtuvo %d", got)
	}
	if got := parseTcRate("filter protocol all\n police rate 50000Kbit burst 32k \n", "broadcast"); got != 50000 {
		t.Fatalf("50000Kbit debe mantenerse, se obtuvo %d", got)
	}
}
