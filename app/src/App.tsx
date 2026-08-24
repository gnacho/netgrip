import { useState } from "react";
import { api } from "./api";
import { Login } from "./pages/Login";
import { Dashboard } from "./pages/Dashboard";

export default function App() {
  const [authed, setAuthed] = useState(false);

  if (!authed) {
    return <Login onSuccess={() => setAuthed(true)} />;
  }
  return (
    <Dashboard
      onLogout={async () => {
        await api.logout().catch(() => {});
        setAuthed(false);
      }}
    />
  );
}
