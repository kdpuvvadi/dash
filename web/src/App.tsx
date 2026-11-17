import { useEffect, useMemo, useState } from "react";
import { connectWS, fetchLayout, formatBytes, formatRate, iconUrl, timeNow } from "./api";
import Sparkline from "./Sparkline";
import type { Block, HostSnap, LayoutPayload, Section, Snapshot } from "./types";

const ACCENTS = ["#fb923c", "#38bdf8", "#a78bfa", "#f472b6", "#2dd4bf", "#fbbf24", "#3dd68c"];

type Hist = Record<string, { t: string; v: number }[]>;

function pushLiveHistory(prev: Hist, snap: Snapshot): Hist {
  const next: Hist = { ...prev };
  const t = snap.updated_at || new Date().toISOString();
  for (const h of Object.values(snap.hosts || {})) {
    const samples: [string, number][] = [
      ["cpu", h.cpu_pct || 0],
      ["mem", h.mem_pct || 0],
      ["disk", h.disk_pct || 0],
      ["net_rx", h.net_rx_bps || 0],
      ["net_tx", h.net_tx_bps || 0],
    ];
    for (const [metric, v] of samples) {
      const key = `${h.name}:${metric}`;
      const arr = [...(next[key] || [])];
      const last = arr[arr.length - 1];
      if (!last || last.t !== t) {
        arr.push({ t, v });
        if (arr.length > 120) arr.shift();
        next[key] = arr;
      }
    }
  }
  return next;
}

export default function App() {
  const [layout, setLayout] = useState<LayoutPayload | null>(null);
  const [snap, setSnap] = useState<Snapshot | null>(null);
  const [history, setHistory] = useState<Hist>({});
  const [connected, setConnected] = useState(false);
  const [category, setCategory] = useState<string>("");
  const [clock, setClock] = useState(timeNow());

  useEffect(() => {
    fetchLayout()
      .then((data) => {
        setLayout(data);
        setSnap(data.snapshot);
        setHistory(data.history || {});
        const first = data.categories?.[0]?.id || "";
        setCategory(first);
      })
      .catch(console.error);
  }, []);

  useEffect(() => {
    return connectWS(
      (s) => {
        setSnap(s);
        setHistory((prev) => pushLiveHistory(prev, s));
      },
      (ok) => setConnected(ok)
    );
  }, []);

  useEffect(() => {
    const t = window.setInterval(() => setClock(timeNow()), 1000);
    return () => clearInterval(t);
  }, []);

  const sections = useMemo(() => {
    if (!layout) return [];
    const raw = layout.layout as { sections?: Section[]; Sections?: Section[] } | undefined;
    const all = raw?.sections || raw?.Sections || [];
    if (!category) return all;
    const filtered = all.filter((s) => !s.category || s.category === category);
    return filtered.length ? filtered : all;
  }, [layout, category]);

  if (!layout || !snap) {
    return (
      <div className="app">
        <p style={{ color: "var(--muted)" }}>Loading dashboard…</p>
      </div>
    );
  }

  const summary = snap.summary;
  const groups = Object.fromEntries((layout.groups || []).map((g) => [g.id, g.name]));

  return (
    <div className="app">
      <header className="topbar">
        <div className="brand">
          <div className="brand-mark" aria-hidden>
            <span />
            <span />
            <span />
          </div>
          <span>{layout.brand?.name || "homelab"}</span>
        </div>
        <nav className="nav">
          {(layout.categories || []).map((c) => (
            <button
              key={c.id}
              className={category === c.id ? "active" : ""}
              onClick={() => setCategory(c.id)}
              type="button"
            >
              {c.name}
            </button>
          ))}
        </nav>
        <div className={`status-pill ${connected ? "ok" : ""}`}>
          <span className="dot" />
          {connected ? "Connected to home" : "Reconnecting…"}
        </div>
      </header>

      <section className="hero">
        <h1>{layout.brand?.tagline || "A little lab. A lot of possibility."}</h1>
        <div className="local-time">
          <div className="label">LOCAL TIME</div>
          <div className="value">{clock}</div>
        </div>
      </section>

      <section className="summary">
        <div className="summary-item">
          <div className="k">CPU</div>
          <div className="v">{(summary?.cpu_pct ?? 0).toFixed(0)}%</div>
        </div>
        <div className="summary-item">
          <div className="k">Memory</div>
          <div className="v">
            {summary?.mem_used || "—"} / {summary?.mem_total || "—"}
            <span className="summary-pct">{(summary?.mem_pct ?? 0).toFixed(0)}%</span>
          </div>
        </div>
        <div className="summary-item">
          <div className="k">Network</div>
          <div className="v">
            ↓ {summary?.net_rx_human || formatRate(summary?.net_rx_bps)}
            <span className="summary-sep">·</span>
            ↑ {summary?.net_tx_human || formatRate(summary?.net_tx_bps)}
          </div>
        </div>
        <div className="summary-item">
          <div className="k">Storage</div>
          <div className="v">
            {summary?.storage_used || "—"} / {summary?.storage_total || "—"}
            <span className="summary-pct">{(summary?.storage_pct ?? 0).toFixed(0)}%</span>
          </div>
        </div>
      </section>

      {sections.map((sec) => (
        <section key={sec.id}>
          <div className="section-head">
            <h2>{sec.title || "Your services"}</h2>
            <div className="range">Live · every {layout.poll_seconds || 5}s</div>
          </div>
          {sec.rows?.map((row, idx) => (
            <div key={idx}>
              {row.group && <div className="group-label">{groups[row.group] || row.group}</div>}
              <div
                className="row-grid"
                style={{ gridTemplateColumns: `repeat(${row.columns || 1}, minmax(0, 1fr))` }}
              >
                {row.blocks?.map((b, i) => (
                  <BlockCard
                    key={i}
                    block={b}
                    snap={snap}
                    history={history}
                    accent={ACCENTS[i % ACCENTS.length]}
                    requestUrl={layout.app_request_url}
                    repoUrl={layout.repo_url}
                  />
                ))}
              </div>
            </div>
          ))}
        </section>
      ))}

      <footer className="footer">
        <span>
          {summary?.status || "—"} · {summary?.services_online ?? 0}/{summary?.services_total ?? 0} online · up{" "}
          {summary?.uptime_human || "—"}
        </span>
        <span>
          Live via socket • Updated{" "}
          {snap.updated_at ? new Date(snap.updated_at).toLocaleTimeString() : "—"}
        </span>
      </footer>
    </div>
  );
}

function BlockCard({
  block,
  snap,
  history,
  accent,
  requestUrl,
  repoUrl,
}: {
  block: Block;
  snap: Snapshot;
  history: Hist;
  accent: string;
  requestUrl: string;
  repoUrl: string;
}) {
  if (block.type === "host") {
    const h = snap.hosts?.[block.ref || ""];
    return <HostCard host={h} accent={accent} history={history} icon={block.icon} />;
  }
  if (block.type === "app") {
    const app =
      Object.values(snap.apps || {}).find((a) => a.type === block.ref) ||
      snap.apps?.[block.ref || ""];
    return <AppCard app={app} accent={accent} icon={block.icon} />;
  }
  if (block.type === "guests") {
    const h = snap.hosts?.[block.ref || ""];
    const count = h?.guests?.length || 0;
    const running = h?.guests?.filter((g) => g.status === "running").length || 0;
    return (
      <GenericCard
        title="Guests"
        sub={h?.name || block.ref || ""}
        icon={block.icon || "proxmox"}
        online={!!h?.online}
        metricLabel="VMs / LXCs"
        metricValue={`${running} / ${count}`}
        accent={accent}
        url={h?.hostname}
      />
    );
  }
  if (block.type === "containers") {
    const h = snap.hosts?.[block.ref || ""];
    const cts = h?.containers || [];
    const up = cts.filter((c) => /up/i.test(c.status)).length;
    return (
      <div className="card">
        <span className={`card-online ${h?.online ? "on" : ""}`} />
        <div className="host-head">
          <div className="card-icon" style={{ boxShadow: `inset 0 0 0 1px ${accent}33` }}>
            <img src={iconUrl(block.icon || "docker")} alt="" />
          </div>
          <div>
            <div className="card-title">Docker</div>
            <div className="card-sub">
              {up} / {cts.length} running
            </div>
          </div>
        </div>
        <div className="container-list">
          {cts.length === 0 && <span style={{ color: "var(--muted)" }}>No containers</span>}
          {cts.map((c) => (
            <div key={c.id} className="container-row">
              <div className="container-icon">
                <img src={iconUrl(c.icon || "docker")} alt="" />
                <span className={`container-dot ${/up/i.test(c.status) ? "on" : ""}`} />
              </div>
              <div className="container-meta">
                <strong>{c.app_name || c.name}</strong>
                <span>
                  {c.name}
                  {c.image ? ` · ${c.image}` : ""}
                </span>
              </div>
              <div className="container-stats">
                {(c.cpu_pct || 0).toFixed(0)}%
                {c.mem_usage ? ` · ${c.mem_usage.split(" / ")[0]}` : ""}
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  }
  if (block.type === "metric") {
    const h = snap.hosts?.[block.ref || ""];
    const field = block.field || "cpu";
    const value =
      field === "mem"
        ? `${(h?.mem_pct || 0).toFixed(0)}%`
        : field === "disk"
          ? `${(h?.disk_pct || 0).toFixed(0)}%`
          : field === "net" || field === "net_rx"
            ? formatRate(h?.net_rx_bps)
            : `${(h?.cpu_pct || 0).toFixed(0)}%`;
    const histKey =
      field === "mem" || field === "disk" || field === "net_rx" || field === "net_tx"
        ? `${h?.name}:${field}`
        : field === "net"
          ? `${h?.name}:net_rx`
          : `${h?.name}:cpu`;
    return (
      <GenericCard
        title={h?.name || block.ref || ""}
        sub={field.toUpperCase()}
        icon={block.icon || h?.icon || "linux"}
        online={!!h?.online}
        metricLabel={field}
        metricValue={value}
        accent={accent}
        spark={history[histKey] || []}
      />
    );
  }
  if (block.type === "unknown_apps") {
    const list = snap.unknown_apps || [];
    return (
      <div className="card">
        <div className="card-title">Unknown apps</div>
        <div className="card-sub">No catalog / icon match yet</div>
        <div className="container-list" style={{ marginTop: 12 }}>
          {list.length === 0 && <span style={{ color: "var(--muted)" }}>None right now</span>}
          {list.slice(0, 8).map((u, i) => (
            <div key={i} className="container-row">
              <div className="container-icon">
                <img src={iconUrl(u.icon || "docker")} alt="" />
              </div>
              <div className="container-meta">
                <strong>{u.app_name || u.name}</strong>
                <span>
                  {u.name} · {u.image}
                </span>
              </div>
            </div>
          ))}
        </div>
        <div className="links">
          <a href={requestUrl} target="_blank" rel="noreferrer">
            Request this app
          </a>
          <a href={`${repoUrl}/blob/main/docs/add-an-app.md`} target="_blank" rel="noreferrer">
            How to add locally
          </a>
          <a href={`${repoUrl}/blob/main/CONTRIBUTING.md`} target="_blank" rel="noreferrer">
            Open a PR
          </a>
        </div>
      </div>
    );
  }
  return (
    <div className="card">
      <div className="card-title">{block.type}</div>
      <div className="card-sub">Unknown block type</div>
    </div>
  );
}

function HostCard({
  host,
  accent,
  history,
  icon,
}: {
  host?: HostSnap;
  accent: string;
  history: Hist;
  icon?: string;
}) {
  if (!host) {
    return (
      <div className="card">
        <div className="card-title">Missing host</div>
        <div className="badge">Not in snapshot</div>
      </div>
    );
  }
  return (
    <div className="card host-card">
      <span className={`card-online ${host.online ? "on" : ""}`} />
      <div className="host-head">
        <div className="card-icon" style={{ boxShadow: `inset 0 0 0 1px ${accent}33` }}>
          <img src={iconUrl(icon || host.icon || "linux")} alt="" />
        </div>
        <div>
          <div className="card-title">{host.name}</div>
          <div className="card-sub">{host.hostname || host.os || host.role}</div>
        </div>
      </div>
      {host.error && <div className="badge">{host.error}</div>}
      <div className="host-metrics">
        <div>
          <div className="metric-label">CPU</div>
          <div className="metric-value" style={{ color: accent }}>
            {(host.cpu_pct || 0).toFixed(0)}%
          </div>
          <Sparkline points={history[`${host.name}:cpu`] || []} color={accent} />
        </div>
        <div>
          <div className="metric-label">Memory</div>
          <div className="metric-value" style={{ color: accent }}>
            {(host.mem_pct || 0).toFixed(0)}%
          </div>
          <div className="metric-sub">
            {formatBytes(host.mem_used)} / {formatBytes(host.mem_total)}
          </div>
        </div>
        <div>
          <div className="metric-label">Disk</div>
          <div className="metric-value" style={{ color: accent }}>
            {(host.disk_pct || 0).toFixed(0)}%
          </div>
          <div className="metric-sub">
            {formatBytes(host.disk_used)} / {formatBytes(host.disk_total)}
          </div>
        </div>
        <div>
          <div className="metric-label">Network</div>
          <div className="metric-value" style={{ color: accent }}>
            ↓ {formatRate(host.net_rx_bps)} · ↑ {formatRate(host.net_tx_bps)}
          </div>
        </div>
      </div>
    </div>
  );
}

function AppCard({
  app,
  accent,
  icon,
}: {
  app?: import("./types").AppSnap;
  accent: string;
  icon?: string;
}) {
  if (!app) {
    return (
      <div className="card">
        <div className="card-title">App</div>
        <div className="badge">needs config in app.yaml</div>
      </div>
    );
  }
  const fieldKey = Object.keys(app.fields || {})[0];
  const fieldVal = fieldKey ? app.fields[fieldKey] : app.online ? "ok" : "—";
  return (
    <GenericCard
      title={app.name}
      sub={app.type}
      icon={icon || app.icon || app.type}
      online={app.online}
      metricLabel={fieldKey || "status"}
      metricValue={String(fieldVal)}
      accent={accent}
      url={app.url}
      needsKey={app.needs_key}
      footnote={app.error}
    />
  );
}

function GenericCard(props: {
  title: string;
  sub: string;
  icon: string;
  online: boolean;
  metricLabel: string;
  metricValue: string;
  accent: string;
  url?: string;
  spark?: { t: string; v: number }[];
  footnote?: string;
  needsKey?: boolean;
}) {
  const href = props.url && /^https?:/i.test(props.url) ? props.url : undefined;
  const inner = (
    <>
      <span className={`card-online ${props.online ? "on" : ""}`} />
      <div className="card-body">
        <div className="card-left">
          <div className="card-icon" style={{ boxShadow: `inset 0 0 0 1px ${props.accent}33` }}>
            <img src={iconUrl(props.icon)} alt="" />
          </div>
          <div>
            <div className="card-title">{props.title}</div>
            <div className="card-sub">{props.sub}</div>
          </div>
          <div className="card-url">{props.url || ""}</div>
          {props.needsKey && (
            <div className="badge">needs an API key — add it in secrets.yaml (UI is read-only)</div>
          )}
          {props.footnote && !props.needsKey && (
            <div className="card-sub" style={{ marginTop: 4 }}>
              {props.footnote}
            </div>
          )}
        </div>
        <div className="card-right">
          <div>
            <div className="metric-label">{props.metricLabel}</div>
            <div className="metric-value" style={{ color: props.accent }}>
              {props.metricValue}
            </div>
          </div>
          <Sparkline points={props.spark || []} color={props.accent} />
        </div>
      </div>
    </>
  );
  if (href) {
    return (
      <a className="card" href={href} target="_blank" rel="noreferrer">
        {inner}
      </a>
    );
  }
  return <div className="card">{inner}</div>;
}
