import { useState } from "react";
import { api } from "./api";
import { Login } from "./pages/Login";
import { Shell } from "./components/Shell";

export default function App() {
  const [authed, setAuthed] = useState(false);

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
