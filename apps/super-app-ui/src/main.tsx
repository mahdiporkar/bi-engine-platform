import React from "react";
import { createRoot } from "react-dom/client";
import { BarChart3, LogIn, MonitorCog, PanelsTopLeft, ShieldCheck } from "lucide-react";
import "./styles.css";

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "/api";
const publicSupersetUrl = "/superset/public/";
const operationSupersetUrl = "/api/superset/operation/";

type SupersetZone = "public" | "operation";

function App() {
  const [activeZone, setActiveZone] = React.useState<SupersetZone>("public");
  const frameUrl = activeZone === "public" ? publicSupersetUrl : operationSupersetUrl;

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
      </aside>

      <section className="content">
        <header className="topbar">
          <div>
            <p className="eyebrow">Super App</p>
            <h1>Integration Platform</h1>
          </div>
          <div className="topbarActions">
            <a className="iconButton" href={`${apiBaseUrl}/health`} title="Backend health">
              <ShieldCheck size={18} />
            </a>
            <a className="loginButton" href={`${apiBaseUrl}/auth/login?return_to=/`}>
              <LogIn size={18} />
              Login
            </a>
          </div>
        </header>

        <section className="workspace">
          <div className="workspaceBar">
            <div className="zoneTabs" aria-label="Superset zone">
              <button
                className={activeZone === "public" ? "zoneTab active" : "zoneTab"}
                type="button"
                onClick={() => setActiveZone("public")}
              >
                <PanelsTopLeft size={18} />
                Public
              </button>
              <button
                className={activeZone === "operation" ? "zoneTab active" : "zoneTab"}
                type="button"
                onClick={() => setActiveZone("operation")}
              >
                <MonitorCog size={18} />
                Operation
              </button>
            </div>
          </div>

          <iframe
            title="Superset workspace"
            src={frameUrl}
            className="supersetFrame"
            allow="fullscreen"
          />
        </section>
      </section>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
