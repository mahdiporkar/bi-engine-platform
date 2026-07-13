import React from "react";
import { createRoot } from "react-dom/client";
import { BarChart2, MonitorCog, PanelsTopLeft } from "lucide-react";
import "./styles.css";

const publicUrl = import.meta.env.VITE_SUPERSET_PUBLIC_URL ?? "/superset/public/";

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
        <span className="serviceBadge">
          <MonitorCog size={18} />
          Operation API
        </span>
        <a href="/api/superset/zones" target="_blank" rel="noreferrer">
          <BarChart2 size={18} />
          Zones API
        </a>
      </section>

      <section className="portsPanel" aria-label="Superset ports">
        <div className="portItem">
          <span className="portLabel">Public UI via proxy</span>
          <strong>80</strong>
        </div>
        <div className="portItem">
          <span className="portLabel">Operation API via proxy</span>
          <strong>80</strong>
        </div>
      </section>

      <iframe
        title="Superset public zone"
        name="superset-frame"
        src={publicUrl}
        className="frame"
        allow="fullscreen"
      />
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
