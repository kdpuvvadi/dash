import { useEffect, useMemo, useState } from "react";
import { connectWS, fetchLayout, formatBytes, formatRate, iconUrl, timeNow } from "./api";
import Sparkline from "./Sparkline";
import type { AppSnap, Container, HostSnap, LayoutPayload, Snapshot } from "./types";

const ACCENTS = ["#fb923c", "#38bdf8", "#a78bfa", "#f472b6", "#2dd4bf", "#fbbf24", "#3dd68c"];

type Hist = Record<string, { t: string; v: number }[]>;

type ServiceCard = {
  key: string;
  title: string;
  sub: string;
  icon: string;
  url: string;
  online: boolean;
  metricLabel: string;
  metricValue: string;
  spark: { t: string; v: number }[];
  accent: string;
  tint: string;
};

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

function roleLabel(role?: string) {
  switch ((role || "").toLowerCase()) {
    case "proxmox":
      return "Virtualization";
    case "docker":
      return "Containers";
    default:
      return "Server";
  }
}

function hostUrl(h: HostSnap) {
  if (h.hostname && h.hostname.includes(".")) return h.hostname;
  if (h.hostname) return `${h.hostname}.home`;
  return h.name.includes(".") ? h.name : `${h.name}.home`;
}

function hexAlpha(hex: string, a: number) {
  const h = hex.replace("#", "");
  const n = parseInt(h.length === 3 ? h.split("").map((c) => c + c).join("") : h, 16);
  const r = (n >> 16) & 255;
  const g = (n >> 8) & 255;
  const b = n & 255;
  return `rgba(${r},${g},${b},${a})`;
}

function buildServices(snap: Snapshot, history: Hist, category: string): ServiceCard[] {
  const hosts = Object.values(snap.hosts || {});
  const filtered = category
    ? hosts.filter((h) => !h.category || h.category === category)
    : hosts;
  const cards: ServiceCard[] = [];
  let i = 0;

  for (const h of filtered) {
    const accent = ACCENTS[i++ % ACCENTS.length];
    cards.push({
      key: `host:${h.name}`,
      title: h.name,
      sub: roleLabel(h.role),
      icon: h.icon || (h.role === "proxmox" ? "proxmox" : h.role === "docker" ? "docker" : "linux"),
      url: hostUrl(h),
      online: !!h.online,
      metricLabel: "CPU",
      metricValue: `${(h.cpu_pct || 0).toFixed(0)}%`,
      spark: history[`${h.name}:cpu`] || [],
      accent,
      tint: hexAlpha(accent, 0.08),
    });

    for (const c of h.containers || []) {
      const accentC = ACCENTS[i++ % ACCENTS.length];
      cards.push(containerCard(c, h, accentC, history));
    }
  }

  for (const app of Object.values(snap.apps || {})) {
    if (category && app.category && app.category !== category) continue;
    // skip apps that duplicate a container already shown
    const dup = cards.some(
      (c) => c.title.toLowerCase() === (app.name || "").toLowerCase() || c.key.includes(app.id)
    );
    if (dup) continue;
    const accent = ACCENTS[i++ % ACCENTS.length];
    cards.push(appCard(app, accent));
  }

  return cards;
}

function containerCard(c: Container, host: HostSnap, accent: string, history: Hist): ServiceCard {
  const up = /up/i.test(c.status);
  const mem = c.mem_usage ? c.mem_usage.split(" / ")[0] : "";
  const metricValue =
    c.cpu_pct && c.cpu_pct > 0
      ? `${c.cpu_pct.toFixed(0)}%`
      : mem
        ? mem
        : up
          ? "running"
          : "stopped";
  return {
    key: `ct:${host.name}:${c.id}`,
    title: c.app_name || c.name,
    sub: c.app_name && c.app_name !== c.name ? c.name : host.name,
    icon: c.icon || "docker",
    url: hostUrl(host),
    online: up,
    metricLabel: c.cpu_pct && c.cpu_pct > 0 ? "CPU" : mem ? "Memory" : "Status",
    metricValue,
    spark: history[`${host.name}:cpu`] || [],
    accent,
    tint: hexAlpha(accent, 0.08),
  };
}

function appCard(app: AppSnap, accent: string): ServiceCard {
  const fieldKey = Object.keys(app.fields || {})[0];
  const fieldVal = fieldKey ? app.fields[fieldKey] : app.online ? "ok" : "—";
  let url = app.url || "";
  try {
    if (url) url = new URL(url).host;
  } catch {
    /* keep raw */
  }
  return {
    key: `app:${app.id}`,
    title: app.name,
    sub: app.type,
    icon: app.icon || app.type,
    url: url || `${app.name}.home`,
    online: !!app.online,
    metricLabel: fieldKey || "Status",
    metricValue: String(fieldVal),
    spark: [],
    accent,
    tint: hexAlpha(accent, 0.08),
  };
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

  const services = useMemo(() => {
    if (!snap) return [];
    return buildServices(snap, history, category);
  }, [snap, history, category]);

  if (!layout || !snap) {
    return (
      <div className="app">
        <p className="loading">Loading dashboard…</p>
      </div>
    );
  }

  const summary = snap.summary;
  const online = summary?.services_online ?? 0;
  const total = summary?.services_total ?? 0;
  const allOk = online >= total && total > 0;
  const countLabel = String(services.length).padStart(2, "0");

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
        <div className="hero-copy">
          <h1>{layout.brand?.tagline || "A little lab. A lot of possibility."}</h1>
          <p className="hero-sub">All your apps, right where they belong.</p>
        </div>
        <div className="local-time">
          <div className="label">LOCAL TIME</div>
          <div className="value">{clock}</div>
        </div>
      </section>

      <section className="summary">
        <div className="summary-item">
          <div className="k">System Status</div>
          <div className="v">
            <span className={`dot ${allOk ? "ok" : ""}`} />
            {summary?.status || (allOk ? "All systems operational" : "Attention needed")}
          </div>
        </div>
        <div className="summary-item">
          <div className="k">Services</div>
          <div className="v">
            {online} / {total} online
          </div>
        </div>
        <div className="summary-item">
          <div className="k">Uptime</div>
          <div className="v">{summary?.uptime_human || "—"}</div>
        </div>
        <div className="summary-item">
          <div className="k">Storage</div>
          <div className="v">
            {summary?.storage_used || "—"} / {summary?.storage_total || "—"}
          </div>
        </div>
      </section>

      <section className="services">
        <div className="section-head">
          <h2>
            Your services <span className="count">{countLabel}</span>
          </h2>
          <div className="range">Live · every {layout.poll_seconds || 5}s</div>
        </div>

        <div className="service-grid">
          {services.map((s) => (
            <ServiceTile key={s.key} card={s} />
          ))}
        </div>
      </section>

      <footer className="footer">
        <span>
          {formatBytes(
            Object.values(snap.hosts || {}).reduce((a, h) => a + (h.mem_used || 0), 0)
          )}{" "}
          RAM · ↓ {formatRate(summary?.net_rx_bps)} · ↑ {formatRate(summary?.net_tx_bps)}
        </span>
        <span>
          Live via socket • Updated{" "}
          {snap.updated_at ? new Date(snap.updated_at).toLocaleTimeString() : "—"}
        </span>
      </footer>
    </div>
  );
}

function ServiceTile({ card }: { card: ServiceCard }) {
  const inner = (
    <>
      <span className={`card-online ${card.online ? "on" : ""}`} />
      <div className="card-body">
        <div className="card-left">
          <div
            className="card-icon"
            style={{
              background: hexAlpha(card.accent, 0.14),
              boxShadow: `inset 0 0 0 1px ${hexAlpha(card.accent, 0.28)}`,
            }}
          >
            <img src={iconUrl(card.icon)} alt="" />
          </div>
          <div className="card-url">{card.url}</div>
        </div>
        <div className="card-right">
          <div>
            <div className="card-title">{card.title}</div>
            <div className="card-sub">{card.sub}</div>
          </div>
          <div className="card-metric">
            <div className="metric-label">{card.metricLabel}</div>
            <div className="metric-value" style={{ color: card.accent }}>
              {card.metricValue}
            </div>
          </div>
          <Sparkline points={card.spark} color={card.accent} />
        </div>
      </div>
    </>
  );

  const href = card.url && /^https?:/i.test(card.url) ? card.url : undefined;
  if (href) {
    return (
      <a className="card" href={href} target="_blank" rel="noreferrer" style={{ background: card.tint }}>
        {inner}
      </a>
    );
  }
  return (
    <div className="card" style={{ background: card.tint }}>
      {inner}
    </div>
  );
}
