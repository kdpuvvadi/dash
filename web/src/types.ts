export type Category = { id: string; name: string };
export type Group = { id: string; name: string; category?: string };

export type Block = {
  type: string;
  ref?: string;
  field?: string;
  icon?: string;
};

export type Row = {
  columns: number;
  group?: string;
  blocks: Block[];
};

export type Section = {
  id: string;
  title?: string;
  category?: string;
  rows: Row[];
};

export type HostSnap = {
  name: string;
  online: boolean;
  role: string;
  icon?: string;
  category?: string;
  group?: string;
  tags?: string[];
  hostname?: string;
  os?: string;
  cpu_pct: number;
  mem_used: number;
  mem_total: number;
  mem_pct: number;
  disk_used: number;
  disk_total: number;
  disk_pct: number;
  load1?: number;
  uptime_sec?: number;
  net_rx?: number;
  net_tx?: number;
  net_rx_bps?: number;
  net_tx_bps?: number;
  error?: string;
  guests?: Guest[];
  containers?: Container[];
};

export type Guest = {
  id: string;
  name: string;
  type: string;
  status: string;
  cpu_pct: number;
  mem_pct: number;
};

export type Container = {
  id: string;
  name: string;
  image: string;
  status: string;
  state?: string;
  health?: string;
  label?: string;
  cpu_pct: number;
  mem_usage?: string;
  net_io?: string;
  ports?: number[];
  app_id?: string;
  app_name?: string;
  icon?: string;
  known?: boolean;
};

export type AppSnap = {
  id: string;
  type: string;
  name: string;
  host_ref: string;
  url?: string;
  icon?: string;
  online: boolean;
  needs_key: boolean;
  fields: Record<string, string>;
  error?: string;
  category?: string;
  group?: string;
};

export type UnknownApp = {
  host_ref: string;
  name: string;
  image: string;
  ports?: number[];
  icon?: string;
  app_name?: string;
};

export type Snapshot = {
  updated_at: string;
  summary: {
    status: string;
    services_online: number;
    services_total: number;
    uptime_human: string;
    cpu_pct?: number;
    mem_pct?: number;
    mem_used?: string;
    mem_total?: string;
    storage_used: string;
    storage_total: string;
    storage_pct: number;
    net_rx_bps?: number;
    net_tx_bps?: number;
    net_rx_human?: string;
    net_tx_human?: string;
  };
  hosts: Record<string, HostSnap>;
  apps: Record<string, AppSnap>;
  unknown_apps: UnknownApp[];
  alerts?: { level: string; host: string; message: string }[];
};

export type LayoutPayload = {
  categories: Category[];
  groups: Group[];
  layout: { sections: Section[] };
  snapshot: Snapshot;
  history: Record<string, { t: string; v: number }[]>;
  brand: { name: string; tagline: string };
  read_only: boolean;
  repo_url: string;
  app_request_url: string;
  poll_seconds: number;
};
