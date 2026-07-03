import React from "react";
import { createRoot } from "react-dom/client";
import { BarChart2, MonitorCog, PanelsTopLeft } from "lucide-react";
import "./styles.css";

const publicUrl = import.meta.env.VITE_SUPERSET_PUBLIC_URL ?? "/superset/public";
const operationUrl = import.meta.env.VITE_SUPERSET_OPERATION_URL ?? "/superset/operation";

function App() {
  return (
    <main className="page">
      <header className="header">
        <a href="/" className="backLink">
          BI Engine
        </a>
        <div>
          <p>Micro Frontend</p>
          <h1>Superset Workspace</h1>
        </div>
      </header>

      <section className="toolbar">
        <a href={publicUrl} target="superset-frame">
          <PanelsTopLeft size={18} />
          Public
        </a>
        <a href={operationUrl} target="superset-frame">
          <MonitorCog size={18} />
          Operation
        </a>
        <a href="/api/superset/zones" target="_blank" rel="noreferrer">
          <BarChart2 size={18} />
          Zones API
        </a>
      </section>

      <iframe
        title="Superset zone"
        name="superset-frame"
        src={publicUrl}
        className="frame"
        allow="fullscreen"
      />
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<App />);

