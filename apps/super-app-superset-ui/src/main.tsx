import React from "react";
import { createRoot } from "react-dom/client";
import { BarChart2, ExternalLink, MonitorCog, PanelsTopLeft } from "lucide-react";
import "./styles.css";

const publicUrl = import.meta.env.VITE_SUPERSET_PUBLIC_URL ?? "/superset/public/";
const operationUrl =
  import.meta.env.VITE_SUPERSET_OPERATION_URL ?? "/api/superset/operation/";
const publicDirectUrl =
  import.meta.env.VITE_SUPERSET_PUBLIC_DIRECT_URL ??
  "http://localhost:8088/superset/public/";
const gatewayOrigin = window.location.origin;
const publicGatewayUrl = new URL(publicUrl, gatewayOrigin).toString();
const operationGatewayUrl = new URL(operationUrl, gatewayOrigin).toString();

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
        <a href={operationGatewayUrl} target="_blank" rel="noreferrer">
          <MonitorCog size={18} />
          Operation
          <ExternalLink size={14} />
        </a>
        <a href="/api/superset/zones" target="_blank" rel="noreferrer">
          <BarChart2 size={18} />
          Zones API
        </a>
      </section>

      <section className="portsPanel" aria-label="Superset ports">
        <div className="portItem">
          <span className="portLabel">Public via proxy</span>
          <strong>8080</strong>
          <a href={publicGatewayUrl} target="_blank" rel="noreferrer" title="Open Public via proxy">
            <ExternalLink size={16} />
          </a>
        </div>
        <div className="portItem">
          <span className="portLabel">Public direct demo</span>
          <strong>8088</strong>
          <a href={publicDirectUrl} target="_blank" rel="noreferrer" title="Open Public direct demo">
            <ExternalLink size={16} />
          </a>
        </div>
        <div className="portItem">
          <span className="portLabel">Operation via proxy</span>
          <strong>8080</strong>
          <a href={operationGatewayUrl} target="_blank" rel="noreferrer" title="Open Operation via proxy">
            <ExternalLink size={16} />
          </a>
        </div>
        <div className="portItem muted">
          <span className="portLabel">Operation internal</span>
          <strong>8088</strong>
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
