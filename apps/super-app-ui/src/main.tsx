import React from "react";
import { createRoot } from "react-dom/client";
import { BarChart3, ExternalLink, ShieldCheck } from "lucide-react";
import "./styles.css";

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "/api";
const supersetMfeUrl = import.meta.env.VITE_SUPERSET_MFE_URL ?? "/superset-mfe";

function App() {
  return (
    <main className="shell">
      <aside className="nav">
        <div className="brand">
          <BarChart3 size={24} />
          <span>BI Engine</span>
        </div>
        <a className="navItem active" href="/">
          Overview
        </a>
        <a className="navItem" href={supersetMfeUrl}>
          Superset
        </a>
      </aside>

      <section className="content">
        <header className="topbar">
          <div>
            <p className="eyebrow">Super App</p>
            <h1>Integration Platform</h1>
          </div>
          <a className="iconButton" href={`${apiBaseUrl}/health`} title="Backend health">
            <ShieldCheck size={18} />
          </a>
        </header>

        <section className="grid">
          <article className="panel">
            <h2>Public Zone</h2>
            <p>External-facing Superset surface routed through the local gateway.</p>
            <a href="/superset/public/" target="_blank" rel="noreferrer">
              Open <ExternalLink size={16} />
            </a>
          </article>

          <article className="panel">
            <h2>Operation Zone</h2>
            <p>Internal operation Superset surface for privileged workflows.</p>
            <a href="/superset/operation/" target="_blank" rel="noreferrer">
              Open <ExternalLink size={16} />
            </a>
          </article>

          <article className="panel wide">
            <h2>Superset Micro Frontend</h2>
            <p>
              The dedicated React shell for BI embedding lives separately from the main
              Super App UI and is mounted behind the gateway at <code>/superset-mfe</code>.
            </p>
            <a href={supersetMfeUrl}>
              Launch <ExternalLink size={16} />
            </a>
          </article>
        </section>
      </section>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<App />);

