import React from "react";
import { createRoot } from "react-dom/client";
import {
  BarChart3,
  ExternalLink,
  LogIn,
  MonitorCog,
  PanelsTopLeft,
  ShieldCheck,
} from "lucide-react";
import "./styles.css";

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "/api";
const publicSupersetUrl = "/superset/public/";
const operationSupersetUrl = "/api/superset/operation/";
const publicDirectUrl =
  import.meta.env.VITE_SUPERSET_PUBLIC_DIRECT_URL ??
  "http://localhost:8088/superset/public/";
const publicGatewayUrl = new URL(publicSupersetUrl, window.location.origin).toString();
const operationGatewayUrl = new URL(operationSupersetUrl, window.location.origin).toString();

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
                className="zoneTab active"
                type="button"
              >
                <PanelsTopLeft size={18} />
                Public
              </button>
              <a
                className="zoneTab zoneLink"
                href={operationGatewayUrl}
                target="_blank"
                rel="noreferrer"
              >
                <MonitorCog size={18} />
                Operation
                <ExternalLink size={14} />
              </a>
            </div>
            <div className="portSummary" aria-label="Superset ports">
              <a href={publicGatewayUrl} target="_blank" rel="noreferrer">
                Public proxy <strong>8080</strong>
                <ExternalLink size={14} />
              </a>
              <a href={publicDirectUrl} target="_blank" rel="noreferrer">
                Public direct <strong>8088</strong>
                <ExternalLink size={14} />
              </a>
              <a href={operationGatewayUrl} target="_blank" rel="noreferrer">
                Operation proxy <strong>8080</strong>
                <ExternalLink size={14} />
              </a>
              <span>
                Operation internal <strong>8088</strong>
              </span>
            </div>
          </div>

          <iframe
            title="Superset public workspace"
            src={publicSupersetUrl}
            className="supersetFrame"
            allow="fullscreen"
          />
        </section>
      </section>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
