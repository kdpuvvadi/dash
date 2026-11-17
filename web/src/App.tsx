import { useEffect, useMemo, useState } from "react";
import { connectWS, fetchLayout, formatBytes, formatRate, iconUrl, timeNow } from "./api";
import Sparkline from "./Sparkline";
import type { AppSnap, Container, Guest, HostSnap, LayoutPayload, Snapshot } from "./types";

const ACCENTS = ["#fb923c", "#38bdf8", "#a78bfa", "#f472b6", "#2dd4bf", "#fbbf24", "#3dd68c"];

type Hist = Record<string, { t: string; v: number }[]>;

type Page = "home" | "overview" | "hosts";
type ServiceKind = "container" | "app" | "lxc" | "vm";
/** Overview tabs: Apps = Docker containers; Services = API-backed (unlinked) apps. */
type TypeFilter = "all" | "apps" | "services" | "lxc" | "vm";

function matchesTypeFilter(kind: ServiceKind, filter: TypeFilter) {
  if (filter === "all") return true;
  if (filter === "apps") return kind === "container";
  if (filter === "services") return kind === "app";
  return kind === filter;
}

function formatQueryCount(raw: string): string {
  const n = Number(String(raw).replace(/,/g, ""));
  if (!Number.isFinite(n)) return raw;
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(Math.round(n));
}

/** Prefer service metrics linked onto a container (e.g. AdGuard blocked %). */
function containerServiceMetric(c: Container): { label: string; value: string } | null {
  const fields = c.fields;
  if (!fields || !Object.keys(fields).length) return null;
  if (c.app_id === "adguardhome" || fields.blocked_pct || fields.queries) {
    const pct = fields.blocked_pct;
    const q = fields.queries;
    if (pct && q) return { label: "Blocked", value: `${pct} · ${formatQueryCount(q)}` };
    if (pct) return { label: "Blocked", value: pct };
    if (q) return { label: "Queries", value: formatQueryCount(q) };
  }
  if (fields.blocked_pct) return { label: "Blocked", value: fields.blocked_pct };
  const preferred = ["blocked", "ads_percentage", "queries", "status"];
  for (const key of preferred) {
    if (fields[key]) return { label: key.replace(/_/g, " "), value: fields[key] };
  }
  const [k, v] = Object.entries(fields)[0];
  return { label: k.replace(/_/g, " "), value: v };
}
type HealthFilter =
  | "all"
  | "online"
  | "offline"
  | "running"
  | "healthy"
  | "unhealthy"
  | "starting"
  | "restarting"
  | "paused"
  | "stopped";

type ServiceCard = {
  key: string;
  kind: ServiceKind;
  title: string;
  sub: string;
  icon: string;
  url: string;
  online: boolean;
  statusKey: string;
  statusLabel: string;
  metricLabel: string;
  metricValue: string;
  spark: { t: string; v: number }[];
  accent: string;
  tint: string;
};

function parseClientStatus(status: string, state?: string, health?: string, label?: string) {
  if (label && (state || health)) {
    const key = health || state || "unknown";
    return {
      online: state === "running" || state === "restarting" || state === "paused" || /up/i.test(status),
      statusKey: key,
      statusLabel: label,
    };
  }
  const s = (status || "").toLowerCase();
  let h = (health || "").toLowerCase();
  if (!h) {
    if (s.includes("(healthy)")) h = "healthy";
    else if (s.includes("(unhealthy)")) h = "unhealthy";
    else if (s.includes("health: starting")) h = "starting";
  }
  const st = (state || "").toLowerCase();
  if (st === "restarting" || s.startsWith("restarting")) {
    return { online: true, statusKey: "restarting", statusLabel: "Restarting" };
  }
  if (st === "paused" || s.startsWith("paused")) {
    return { online: true, statusKey: "paused", statusLabel: "Paused" };
  }
  if (st === "exited" || s.startsWith("exited") || s.startsWith("stopped") || st === "dead" || st === "created") {
    return { online: false, statusKey: "stopped", statusLabel: "Stopped" };
  }
  if (st === "running" || s.startsWith("up")) {
    if (h === "healthy") return { online: true, statusKey: "healthy", statusLabel: "Healthy" };
    if (h === "unhealthy") return { online: true, statusKey: "unhealthy", statusLabel: "Unhealthy" };
    if (h === "starting") return { online: true, statusKey: "starting", statusLabel: "Starting" };
    return { online: true, statusKey: "running", statusLabel: "Running" };
  }
  return {
    online: /up/i.test(status),
    statusKey: st || "unknown",
    statusLabel: label || status || "Unknown",
  };
}

function matchesHealthFilter(card: ServiceCard, filter: HealthFilter) {
  if (filter === "all") return true;
  if (filter === "online") return card.online;
  if (filter === "offline") return !card.online;
  if (filter === "stopped") {
    return card.statusKey === "stopped" || card.statusKey === "dead" || card.statusKey === "created";
  }
  if (filter === "running") {
    return card.statusKey === "running" || card.statusKey === "healthy" || card.statusKey === "unhealthy" || card.statusKey === "starting";
  }
  return card.statusKey === filter;
}

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

/** Home: only services explicitly listed under host `services:` / `apps:` in config. */
function buildConfiguredServices(snap: Snapshot, history: Hist, category: string): ServiceCard[] {
  const hosts = Object.values(snap.hosts || {});
  const filtered = category
    ? hosts.filter((h) => !h.category || h.category === category)
    : hosts;
  const cards: ServiceCard[] = [];
  let i = 0;

  for (const h of filtered) {
    for (const c of h.containers || []) {
      if (!c.configured) continue;
      cards.push(containerCard(c, h, ACCENTS[i++ % ACCENTS.length], history));
    }
  }

  for (const app of Object.values(snap.apps || {})) {
    if (!app.configured) continue;
    if (category && app.category && app.category !== category) continue;
    const dup = cards.some(
      (c) => c.title.toLowerCase() === (app.name || "").toLowerCase() || c.key.includes(app.id)
    );
    if (dup) continue;
    cards.push(appCard(app, ACCENTS[i++ % ACCENTS.length]));
  }

  return cards;
}

/** Overview: all detected guests, containers, and collected apps. */
function buildServices(snap: Snapshot, history: Hist, category: string): ServiceCard[] {
  const hosts = Object.values(snap.hosts || {});
  const filtered = category
    ? hosts.filter((h) => !h.category || h.category === category)
    : hosts;
  const cards: ServiceCard[] = [];
  let i = 0;

  for (const h of filtered) {
    for (const g of h.guests || []) {
      const accentG = ACCENTS[i++ % ACCENTS.length];
      cards.push(guestCard(g, h, accentG, history));
    }
    for (const c of h.containers || []) {
      const accentC = ACCENTS[i++ % ACCENTS.length];
      cards.push(containerCard(c, h, accentC, history));
    }
  }

  for (const app of Object.values(snap.apps || {})) {
    if (category && app.category && app.category !== category) continue;
    const dup = cards.some(
      (c) => c.title.toLowerCase() === (app.name || "").toLowerCase() || c.key.includes(app.id)
    );
    if (dup) continue;
    const accent = ACCENTS[i++ % ACCENTS.length];
    cards.push(appCard(app, accent));
  }

  return cards;
}

function hostList(snap: Snapshot, category: string): HostSnap[] {
  const hosts = Object.values(snap.hosts || {});
  if (!category) return hosts;
  return hosts.filter((h) => !h.category || h.category === category);
}

function guestCard(g: Guest, host: HostSnap, accent: string, history: Hist): ServiceCard {
  const kind: ServiceKind = g.type === "lxc" ? "lxc" : "vm";
  const running = /running/i.test(g.status);
  const typeLabel = kind === "lxc" ? "LXC" : "VM";
  return {
    key: `guest:${host.name}:${g.type}:${g.id}`,
    kind,
    title: g.name,
    sub: `${typeLabel} ${g.id} · ${host.name}`,
    icon: kind === "lxc" ? "linux" : "proxmox",
    url: hostUrl(host),
    online: running,
    statusKey: running ? "running" : "stopped",
    statusLabel: running ? "Running" : "Stopped",
    metricLabel: running ? "CPU" : "Status",
    metricValue: running ? `${(g.cpu_pct || 0).toFixed(0)}%` : "Stopped",
    spark: history[`${host.name}:cpu`] || [],
    accent,
    tint: hexAlpha(accent, 0.08),
  };
}

function containerCard(c: Container, host: HostSnap, accent: string, history: Hist): ServiceCard {
  const st = parseClientStatus(c.status, c.state, c.health, c.label);
  const mem = c.mem_usage ? c.mem_usage.split(" / ")[0] : "";
  const showResource = st.online && (c.cpu_pct > 0 || !!mem);
  const serviceMetric = containerServiceMetric(c);
  let url = c.url || hostUrl(host);
  try {
    if (c.url) url = new URL(c.url).host;
  } catch {
    /* keep raw */
  }
  return {
    key: `ct:${host.name}:${c.id}`,
    kind: "container",
    title: c.app_name || c.name,
    sub: c.app_name && c.app_name !== c.name ? `${c.name} · ${host.name}` : host.name,
    icon: c.icon || "docker",
    url,
    online: st.online,
    statusKey: st.statusKey,
    statusLabel: st.statusLabel,
    metricLabel: serviceMetric
      ? serviceMetric.label
      : showResource
        ? c.cpu_pct > 0
          ? "CPU"
          : "Memory"
        : "Status",
    metricValue: serviceMetric
      ? serviceMetric.value
      : showResource
        ? c.cpu_pct > 0
          ? `${c.cpu_pct.toFixed(0)}%`
          : mem
        : st.statusLabel,
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
    kind: "app",
    title: app.name,
    sub: app.type,
    icon: app.icon || app.type,
    url: url || `${app.name}.home`,
    online: !!app.online,
    statusKey: app.online ? "running" : "stopped",
    statusLabel: app.online ? "Online" : "Offline",
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
  const [page, setPage] = useState<Page>("home");
  const [category, setCategory] = useState<string>("");
  const [typeFilter, setTypeFilter] = useState<TypeFilter>("all");
  const [healthFilter, setHealthFilter] = useState<HealthFilter>("all");
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

  const homeServices = useMemo(() => {
    if (!snap) return [];
    return buildConfiguredServices(snap, history, category);
  }, [snap, history, category]);

  const allServices = useMemo(() => {
    if (!snap) return [];
    return buildServices(snap, history, category);
  }, [snap, history, category]);

  const typeTabs = useMemo(() => {
    const kinds = new Set(allServices.map((s) => s.kind));
    const count = (pred: (k: ServiceKind) => boolean) =>
      allServices.filter((s) => pred(s.kind)).length;
    const tabs: { value: TypeFilter; label: string; count: number }[] = [
      { value: "all", label: "All", count: allServices.length },
    ];
    const appsCount = count((k) => k === "container");
    const servicesCount = count((k) => k === "app");
    if (appsCount > 0) tabs.push({ value: "apps", label: "Apps", count: appsCount });
    if (servicesCount > 0) tabs.push({ value: "services", label: "Services", count: servicesCount });
    if (kinds.has("lxc")) tabs.push({ value: "lxc", label: "LXCs", count: count((k) => k === "lxc") });
    if (kinds.has("vm")) tabs.push({ value: "vm", label: "VMs", count: count((k) => k === "vm") });
    return tabs;
  }, [allServices]);

  const overviewServices = useMemo(() => {
    return allServices.filter((s) => {
      if (!matchesTypeFilter(s.kind, typeFilter)) return false;
      return matchesHealthFilter(s, healthFilter);
    });
  }, [allServices, typeFilter, healthFilter]);

  const homeFiltered = useMemo(() => {
    return homeServices.filter((s) => matchesHealthFilter(s, healthFilter));
  }, [homeServices, healthFilter]);

  useEffect(() => {
    if (typeFilter !== "all" && !typeTabs.some((t) => t.value === typeFilter)) {
      setTypeFilter("all");
    }
  }, [typeFilter, typeTabs]);

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
  const statusText = summary?.status || "All systems operational";
  const allOk = /operational/i.test(statusText);
  const homeOnline = homeFiltered.filter((s) => s.online).length;
  const homeCountLabel = String(homeFiltered.length).padStart(2, "0");
  const overviewCountLabel = String(overviewServices.length).padStart(2, "0");
  const hosts = hostList(snap, category);
  const hostsOnline = hosts.filter((h) => h.online).length;

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
          <button
            className={page === "home" ? "active" : ""}
            onClick={() => setPage("home")}
            type="button"
          >
            Home
          </button>
          <button
            className={page === "overview" ? "active" : ""}
            onClick={() => setPage("overview")}
            type="button"
          >
            Overview
          </button>
          <button
            className={page === "hosts" ? "active" : ""}
            onClick={() => setPage("hosts")}
            type="button"
          >
            Hosts
          </button>
          {(layout.categories || [])
            .filter((c) => c.id !== "overview")
            .map((c) => (
              <button
                key={c.id}
                className={(page === "home" || page === "overview") && category === c.id ? "active" : ""}
                onClick={() => {
                  setPage("home");
                  setCategory(c.id);
                }}
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

      {page === "home" ? (
        <>
          <section className="hero">
            <div className="hero-copy">
              <h1>{layout.brand?.tagline || "A little lab. A lot of possibility."}</h1>
              <p className="hero-sub">Services you added in config.</p>
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
                {statusText}
              </div>
            </div>
            <div className="summary-item">
              <div className="k">Services</div>
              <div className="v">
                {homeOnline} / {homeFiltered.length} online
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
                Your services <span className="count">{homeCountLabel}</span>
              </h2>
              <div className="section-tools">
                <label className="filter-field">
                  <span>Health</span>
                  <select
                    value={healthFilter}
                    onChange={(e) => setHealthFilter(e.target.value as HealthFilter)}
                  >
                    <option value="all">All status</option>
                    <option value="online">Online</option>
                    <option value="offline">Offline</option>
                    <option value="running">Running</option>
                    <option value="healthy">Healthy</option>
                    <option value="unhealthy">Unhealthy</option>
                    <option value="starting">Starting</option>
                    <option value="restarting">Restarting</option>
                    <option value="paused">Paused</option>
                    <option value="stopped">Stopped</option>
                  </select>
                </label>
                <div className="range">Live · every {layout.poll_seconds || 5}s</div>
              </div>
            </div>

            {homeFiltered.length === 0 ? (
              <p className="empty-filter">
                No services added yet. List them under a host&apos;s <code>services:</code> in app.yaml.
              </p>
            ) : (
              <div className="service-grid">
                {homeFiltered.map((s) => (
                  <ServiceTile key={s.key} card={s} />
                ))}
              </div>
            )}
          </section>
        </>
      ) : page === "overview" ? (
        <>
          <section className="hero hero-compact">
            <div className="hero-copy">
              <h1>Overview</h1>
              <p className="hero-sub">
                Detected containers, LXCs, and VMs across the lab · {online}/{total} online
              </p>
            </div>
            <div className="local-time">
              <div className="label">LOCAL TIME</div>
              <div className="value">{clock}</div>
            </div>
          </section>

          <section className="services">
            <div className="section-head">
              <h2>
                Everything <span className="count">{overviewCountLabel}</span>
              </h2>
              <div className="section-tools">
                <label className="filter-field">
                  <span>Health</span>
                  <select
                    value={healthFilter}
                    onChange={(e) => setHealthFilter(e.target.value as HealthFilter)}
                  >
                    <option value="all">All status</option>
                    <option value="online">Online</option>
                    <option value="offline">Offline</option>
                    <option value="running">Running</option>
                    <option value="healthy">Healthy</option>
                    <option value="unhealthy">Unhealthy</option>
                    <option value="starting">Starting</option>
                    <option value="restarting">Restarting</option>
                    <option value="paused">Paused</option>
                    <option value="stopped">Stopped</option>
                  </select>
                </label>
                <div className="range">Live · every {layout.poll_seconds || 5}s</div>
              </div>
            </div>

            <div className="type-tabs" role="tablist" aria-label="Service type">
              {typeTabs.map((t) => (
                <button
                  key={t.value}
                  type="button"
                  role="tab"
                  aria-selected={typeFilter === t.value}
                  className={typeFilter === t.value ? "active" : ""}
                  onClick={() => setTypeFilter(t.value)}
                >
                  {t.label}
                  <span className="tab-count">{t.count}</span>
                </button>
              ))}
            </div>

            {overviewServices.length === 0 ? (
              <p className="empty-filter">No services match these filters.</p>
            ) : (
              <div className="service-grid">
                {overviewServices.map((s) => (
                  <ServiceTile key={s.key} card={s} />
                ))}
              </div>
            )}
          </section>
        </>
      ) : (
        <>
          <section className="hero hero-compact">
            <div className="hero-copy">
              <h1>Hosts</h1>
              <p className="hero-sub">
                {hostsOnline} / {hosts.length} online · machine CPU, memory, disk, and network
              </p>
            </div>
            <div className="local-time">
              <div className="label">LOCAL TIME</div>
              <div className="value">{clock}</div>
            </div>
          </section>

          <section className="hosts-page">
            <div className="section-head">
              <h2>
                Machines <span className="count">{String(hosts.length).padStart(2, "0")}</span>
              </h2>
              <div className="range">Live · every {layout.poll_seconds || 5}s</div>
            </div>
            {hosts.length === 0 ? (
              <p className="empty-filter">No hosts configured.</p>
            ) : (
              <div className="host-list">
                {hosts.map((h, i) => (
                  <HostDetailCard
                    key={h.name}
                    host={h}
                    history={history}
                    accent={ACCENTS[i % ACCENTS.length]}
                  />
                ))}
              </div>
            )}
          </section>
        </>
      )}

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

function HostDetailCard({
  host,
  history,
  accent,
}: {
  host: HostSnap;
  history: Hist;
  accent: string;
}) {
  const cts = host.containers || [];
  const running = cts.filter((c) => /up/i.test(c.status) || c.state === "running").length;
  return (
    <div className="host-detail" style={{ background: hexAlpha(accent, 0.06) }}>
      <span className={`status-badge ${host.online ? "status-running" : "status-stopped"}`}>
        <span className="status-dot" />
        {host.online ? "Online" : "Offline"}
      </span>
      <div className="host-detail-head">
        <div
          className="card-icon"
          style={{
            background: hexAlpha(accent, 0.14),
            boxShadow: `inset 0 0 0 1px ${hexAlpha(accent, 0.28)}`,
          }}
        >
          <img
            src={iconUrl(
              host.icon || (host.role === "proxmox" ? "proxmox" : host.role === "docker" ? "docker" : "linux")
            )}
            alt=""
          />
        </div>
        <div className="host-detail-id">
          <div className="card-title">{host.name}</div>
          <div className="card-sub">
            {host.hostname || hostUrl(host)}
            {host.os ? ` · ${host.os}` : ""}
            {` · ${roleLabel(host.role)}`}
          </div>
          {host.error && <div className="badge">{host.error}</div>}
        </div>
        <div className="host-detail-meta">
          {cts.length > 0 && (
            <div className="host-meta-pill">
              {running}/{cts.length} containers
            </div>
          )}
        </div>
      </div>
      <div className="host-detail-metrics">
        <div className="host-metric-block">
          <div className="metric-label">CPU</div>
          <div className="metric-value" style={{ color: accent }}>
            {(host.cpu_pct || 0).toFixed(0)}%
          </div>
          <Sparkline points={history[`${host.name}:cpu`] || []} color={accent} />
        </div>
        <div className="host-metric-block">
          <div className="metric-label">Memory</div>
          <div className="metric-value" style={{ color: accent }}>
            {(host.mem_pct || 0).toFixed(0)}%
          </div>
          <div className="metric-sub">
            {formatBytes(host.mem_used)} / {formatBytes(host.mem_total)}
          </div>
        </div>
        <div className="host-metric-block">
          <div className="metric-label">Disk</div>
          <div className="metric-value" style={{ color: accent }}>
            {(host.disk_pct || 0).toFixed(0)}%
          </div>
          <div className="metric-sub">
            {formatBytes(host.disk_used)} / {formatBytes(host.disk_total)}
          </div>
        </div>
        <div className="host-metric-block">
          <div className="metric-label">Network</div>
          <div className="metric-value net" style={{ color: accent }}>
            ↓ {formatRate(host.net_rx_bps)}
            <span className="net-sep">·</span>↑ {formatRate(host.net_tx_bps)}
          </div>
        </div>
      </div>
    </div>
  );
}

function ServiceTile({ card }: { card: ServiceCard }) {
  const inner = (
    <>
      <span className={`status-badge status-${card.statusKey}`} title={card.statusLabel}>
        <span className="status-dot" />
        {card.statusLabel}
      </span>
      <div className="card-body">
        <div className="card-top">
          <div
            className="card-icon"
            style={{
              background: hexAlpha(card.accent, 0.14),
              boxShadow: `inset 0 0 0 1px ${hexAlpha(card.accent, 0.28)}`,
            }}
          >
            <img src={iconUrl(card.icon)} alt="" />
          </div>
          <div className="card-id">
            <div className="card-title">{card.title}</div>
            <div className="card-sub">{card.sub}</div>
            <div className="card-url">{card.url}</div>
          </div>
        </div>
        <div className="card-bottom">
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
