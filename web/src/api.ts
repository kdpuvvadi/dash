import type { LayoutPayload, Snapshot } from "./types";

export async function fetchLayout(): Promise<LayoutPayload> {
  const res = await fetch("/api/layout");
  if (!res.ok) throw new Error("failed to load layout");
  return res.json();
}

export function connectWS(
  onSnap: (s: Snapshot) => void,
  onStatus: (ok: boolean) => void
): () => void {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  let ws: WebSocket | null = null;
  let closed = false;
  let timer: number | undefined;

  const connect = () => {
    ws = new WebSocket(`${proto}://${location.host}/api/ws`);
    ws.onopen = () => onStatus(true);
    ws.onclose = () => {
      onStatus(false);
      if (!closed) timer = window.setTimeout(connect, 1500);
    };
    ws.onerror = () => onStatus(false);
    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data);
        if (msg.type === "snapshot" && msg.data) onSnap(msg.data);
      } catch {
        /* ignore */
      }
    };
  };
  connect();

  const ping = window.setInterval(() => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: "ping" }));
    }
  }, 20000);

  return () => {
    closed = true;
    window.clearInterval(ping);
    if (timer) window.clearTimeout(timer);
    ws?.close();
  };
}

export function iconUrl(name?: string) {
  if (!name) return "/api/icons/linux";
  return `/api/icons/${encodeURIComponent(name)}`;
}

export function formatBytes(n: number) {
  if (!n) return "—";
  const u = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(1)} ${u[i]}`;
}

export function timeNow() {
  return new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}
