package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Snapshot struct {
	UpdatedAt time.Time              `json:"updated_at"`
	Summary   Summary                `json:"summary"`
	Hosts     map[string]*HostSnap   `json:"hosts"`
	Apps      map[string]*AppSnap    `json:"apps"`
	Unknown   []UnknownApp           `json:"unknown_apps"`
	Alerts    []AlertEvent           `json:"alerts"`
}

type Summary struct {
	Status         string  `json:"status"`
	ServicesOnline int     `json:"services_online"`
	ServicesTotal  int     `json:"services_total"`
	UptimeHuman    string  `json:"uptime_human"`
	CPUPct         float64 `json:"cpu_pct"`
	MemPct         float64 `json:"mem_pct"`
	MemUsed        string  `json:"mem_used"`
	MemTotal       string  `json:"mem_total"`
	StorageUsed    string  `json:"storage_used"`
	StorageTotal   string  `json:"storage_total"`
	StoragePct     float64 `json:"storage_pct"`
	NetRxBps       float64 `json:"net_rx_bps"`
	NetTxBps       float64 `json:"net_tx_bps"`
	NetRxHuman     string  `json:"net_rx_human"`
	NetTxHuman     string  `json:"net_tx_human"`
}

type HostSnap struct {
	Name       string            `json:"name"`
	Online     bool              `json:"online"`
	Role       string            `json:"role"`
	Icon       string            `json:"icon"`
	Category   string            `json:"category"`
	Group      string            `json:"group"`
	Tags       []string          `json:"tags"`
	Hostname   string            `json:"hostname"`
	OS         string            `json:"os"`
	CPUPct     float64           `json:"cpu_pct"`
	MemUsed    uint64            `json:"mem_used"`
	MemTotal   uint64            `json:"mem_total"`
	MemPct     float64           `json:"mem_pct"`
	DiskUsed   uint64            `json:"disk_used"`
	DiskTotal  uint64            `json:"disk_total"`
	DiskPct    float64           `json:"disk_pct"`
	Load1      float64           `json:"load1"`
	UptimeSec  int64             `json:"uptime_sec"`
	NetRx      uint64            `json:"net_rx"`
	NetTx      uint64            `json:"net_tx"`
	NetRxBps   float64           `json:"net_rx_bps"`
	NetTxBps   float64           `json:"net_tx_bps"`
	Error      string            `json:"error,omitempty"`
	Guests     []Guest           `json:"guests,omitempty"`
	Containers []Container       `json:"containers,omitempty"`
	Extras     map[string]string `json:"extras,omitempty"`
}

type Guest struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Type   string  `json:"type"` // qemu | lxc
	Status string  `json:"status"`
	CPUPct float64 `json:"cpu_pct"`
	MemPct float64 `json:"mem_pct"`
	MaxMem uint64  `json:"max_mem"`
}

type Container struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Image    string  `json:"image"`
	Status   string  `json:"status"`             // raw Docker status string
	State    string  `json:"state,omitempty"`    // running, stopped, restarting, …
	Health   string  `json:"health,omitempty"`   // healthy, unhealthy, starting
	Label    string  `json:"label,omitempty"`    // display label e.g. Healthy
	CPUPct   float64 `json:"cpu_pct"`
	MemUsage string  `json:"mem_usage"`
	NetIO    string  `json:"net_io"`
	Ports    []int   `json:"ports,omitempty"`
	HostRef  string  `json:"host_ref"`
	GuestID  string  `json:"guest_id,omitempty"`
	AppID    string  `json:"app_id,omitempty"`
	AppName  string  `json:"app_name,omitempty"`
	Icon     string  `json:"icon,omitempty"`
	Known    bool    `json:"known"`
}

type AppSnap struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Name     string            `json:"name"`
	HostRef  string            `json:"host_ref"`
	URL      string            `json:"url"`
	Icon     string            `json:"icon"`
	Online   bool              `json:"online"`
	NeedsKey bool              `json:"needs_key"`
	Fields   map[string]string `json:"fields"`
	Error    string            `json:"error,omitempty"`
	Category string            `json:"category"`
	Group    string            `json:"group"`
}

type UnknownApp struct {
	HostRef string `json:"host_ref"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Ports   []int  `json:"ports"`
	Icon    string `json:"icon,omitempty"`
	AppName string `json:"app_name,omitempty"`
}

type AlertEvent struct {
	Level   string    `json:"level"`
	Host    string    `json:"host"`
	Message string    `json:"message"`
	At      time.Time `json:"at"`
}

type Point struct {
	T time.Time `json:"t"`
	V float64   `json:"v"`
}

type Store struct {
	mu       sync.RWMutex
	db       *sql.DB
	snap     Snapshot
	started  time.Time
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "dash.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, started: time.Now(), snap: Snapshot{
		Hosts: map[string]*HostSnap{},
		Apps:  map[string]*AppSnap{},
	}}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS metrics (
  host TEXT NOT NULL,
  metric TEXT NOT NULL,
  ts INTEGER NOT NULL,
  value REAL NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_metrics_host_metric_ts ON metrics(host, metric, ts);
`)
	return err
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) SetSnapshot(snap Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap.UpdatedAt = time.Now().UTC()
	if snap.Hosts == nil {
		snap.Hosts = map[string]*HostSnap{}
	}
	if snap.Apps == nil {
		snap.Apps = map[string]*AppSnap{}
	}
	s.snap = snap
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSnap(s.snap)
}

func (s *Store) Started() time.Time { return s.started }

func (s *Store) Record(host, metric string, value float64, at time.Time) error {
	_, err := s.db.Exec(`INSERT INTO metrics(host, metric, ts, value) VALUES(?,?,?,?)`,
		host, metric, at.Unix(), value)
	return err
}

func (s *Store) History(host, metric string, since time.Time) ([]Point, error) {
	rows, err := s.db.Query(`SELECT ts, value FROM metrics WHERE host=? AND metric=? AND ts>=? ORDER BY ts`,
		host, metric, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Point
	for rows.Next() {
		var ts int64
		var v float64
		if err := rows.Scan(&ts, &v); err != nil {
			return nil, err
		}
		out = append(out, Point{T: time.Unix(ts, 0).UTC(), V: v})
	}
	return out, rows.Err()
}

func (s *Store) Prune(olderThan time.Time) error {
	_, err := s.db.Exec(`DELETE FROM metrics WHERE ts < ?`, olderThan.Unix())
	return err
}

func cloneSnap(in Snapshot) Snapshot {
	b, _ := json.Marshal(in)
	var out Snapshot
	_ = json.Unmarshal(b, &out)
	if out.Hosts == nil {
		out.Hosts = map[string]*HostSnap{}
	}
	if out.Apps == nil {
		out.Apps = map[string]*AppSnap{}
	}
	return out
}

func FormatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func FormatRate(bps float64) string {
	if bps < 0 {
		bps = 0
	}
	units := []string{"B/s", "KB/s", "MB/s", "GB/s"}
	v := bps
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", v, units[i])
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

func FormatUptime(sec int64) string {
	if sec <= 0 {
		return "—"
	}
	d := sec / 86400
	h := (sec % 86400) / 3600
	if d > 0 {
		return fmt.Sprintf("%d days, %d hours", d, h)
	}
	m := (sec % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%d hours, %d min", h, m)
	}
	return fmt.Sprintf("%d min", m)
}
