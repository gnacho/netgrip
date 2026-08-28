import { useCallback, useEffect, useState } from "react";
import { api } from "./api";
import { Login } from "./pages/Login";
import { Shell } from "./components/Shell";
import { Wizard } from "./pages/Wizard";

export default function App() {
  const [authed, setAuthed] = useState(false);
  const [checking, setChecking] = useState(true);
  const [wizardDone, setWizardDone] = useState(true);

  const checkSession = useCallback(async () => {
    try {
      await api.me();
      setAuthed(true);
      try {
        const ws = await api.wizardState();
        setWizardDone(ws.completed);
      } catch {
        setWizardDone(true);
      }
    } catch {
      // not authed
    } finally {
      setChecking(false);
    }
  }, []);

  useEffect(() => { checkSession(); }, [checkSession]);

  if (checking) {
    return <main className="min-h-screen" />;
  }
  if (!authed) {
    return <Login onSuccess={async () => {
      setAuthed(true);
      try {
        const ws = await api.wizardState();
        setWizardDone(ws.completed);
      } catch {
        setWizardDone(true);
      }
    }} />;
  }
  if (!wizardDone) {
    return <Wizard onDone={() => setWizardDone(true)} />;
  }
  return (
    <Shell
      onLogout={async () => {
        await api.logout().catch(() => {});
        setAuthed(false);
      }}
    />
  );
}
