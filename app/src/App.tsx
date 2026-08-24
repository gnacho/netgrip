import { useEffect, useState } from "react";
import { api } from "./api";
import { Login } from "./pages/Login";
import { Shell } from "./components/Shell";

export default function App() {
  const [authed, setAuthed] = useState(false);
  const [checking, setChecking] = useState(true);

  // Session tokens are HMAC-signed and survive service restarts: a page
  // refresh only needs to ask whether the cookie is still valid.
  useEffect(() => {
    api.me()
      .then(() => setAuthed(true))
      .catch(() => {})
      .finally(() => setChecking(false));
  }, []);

  if (checking) {
    return <main className="min-h-screen" />;
  }
  if (!authed) {
    return <Login onSuccess={() => setAuthed(true)} />;
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
