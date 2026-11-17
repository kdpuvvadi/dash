package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/puvvadi/dash/internal/alerts"
	"github.com/puvvadi/dash/internal/api"
	"github.com/puvvadi/dash/internal/apps"
	"github.com/puvvadi/dash/internal/collector"
	"github.com/puvvadi/dash/internal/config"
	"github.com/puvvadi/dash/internal/embedui"
	"github.com/puvvadi/dash/internal/store"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			os.Exit(cmdInit(os.Args[2:]))
		case "validate":
			os.Exit(cmdValidate(os.Args[2:]))
		case "apps":
			os.Exit(cmdApps(os.Args[2:]))
		case "discover":
			os.Exit(cmdDiscover(os.Args[2:]))
		case "serve":
			os.Exit(cmdServe(os.Args[2:]))
		case "help", "-h", "--help":
			printHelp()
			return
		}
	}
	os.Exit(cmdServe(os.Args[1:]))
}

func printHelp() {
	fmt.Println(`dash — read-only homelab metrics dashboard

Commands:
  serve      Run the hub (default)
  init       Create starter config in the config folder
  validate   Check app.yaml and app definition files
  apps       List loaded app definitions
  discover   Probe one host over SSH and print what was found

Environment:
  DASH_CONFIG   Config folder (default /config)
`)
}

func configDirFromFlags(args []string) string {
	fs := flag.NewFlagSet("dash", flag.ContinueOnError)
	dir := fs.String("config", config.DefaultConfigDir(), "config directory")
	_ = fs.Parse(args)
	return *dir
}

func cmdInit(args []string) int {
	dir := configDirFromFlags(args)
	created, err := config.EnsureFirstLaunch(dir)
	if err != nil {
		fmt.Println("error:", err)
		return 1
	}
	if created {
		fmt.Println("Created starter config in", dir)
	} else {
		fmt.Println("Config already exists in", dir)
	}
	return 0
}

func cmdValidate(args []string) int {
	dir := configDirFromFlags(args)
	cfg, err := config.Load(dir)
	if err != nil {
		fmt.Println("config error:", err)
		return 1
	}
	bundled := bundledAppsDir()
	defs, errs := apps.LoadDefs(bundled, cfg.AppDefDir)
	ok := true
	for _, e := range errs {
		fmt.Println("app def error:", e)
		ok = false
	}
	fmt.Printf("OK: %d hosts, %d app defs\n", len(cfg.Hosts), len(defs))
	if !ok {
		return 1
	}
	return 0
}

func cmdApps(args []string) int {
	dir := configDirFromFlags(args)
	userDir := filepath.Join(dir, "apps.d")
	defs, errs := apps.LoadDefs(bundledAppsDir(), userDir)
	for _, e := range errs {
		fmt.Println("warning:", e)
	}
	for id, d := range defs {
		fmt.Printf("%s\t%s\ticon=%s\n", id, d.Name, d.UI.Icon)
	}
	return 0
}

func cmdDiscover(args []string) int {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)
	host := fs.String("host", "", "hostname or IP")
	user := fs.String("user", "root", "SSH user")
	port := fs.Int("port", 22, "SSH port")
	identity := fs.String("identity", "", "SSH private key path")
	_ = fs.Parse(args)
	if *host == "" {
		fmt.Println("usage: dash discover --host 10.20.20.1")
		return 2
	}
	client, err := collector.DialSSH(*host, *user, *port, *identity)
	if err != nil {
		fmt.Println("ssh error:", err)
		return 1
	}
	defer client.Close()
	run := func(cmd string) (string, error) { return client.RunOK(cmd) }
	hs, err := collector.CollectHostMetrics(run)
	if err != nil {
		fmt.Println("collect error:", err)
		return 1
	}
	fmt.Printf("role=%s hostname=%s os=%s cpu=%.1f%% mem=%.1f%% disk=%.1f%%\n",
		hs.Role, hs.Hostname, hs.OS, hs.CPUPct, hs.MemPct, hs.DiskPct)
	if hs.Role == "proxmox" {
		guests, nested, _ := collector.CollectProxmox(run, *host)
		fmt.Printf("guests=%d nested_containers=%d\n", len(guests), len(nested))
		for _, g := range guests {
			fmt.Printf("  %s %s %s status=%s\n", g.Type, g.ID, g.Name, g.Status)
		}
	}
	cts, _ := collector.CollectDocker(run, *host, "")
	fmt.Printf("docker_containers=%d\n", len(cts))
	for _, ct := range cts {
		fmt.Printf("  %s image=%s status=%s\n", ct.Name, ct.Image, ct.Status)
	}
	return 0
}

func cmdServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	dir := fs.String("config", config.DefaultConfigDir(), "config directory")
	_ = fs.Parse(args)

	if _, err := config.EnsureFirstLaunch(*dir); err != nil {
		log.Fatal(err)
	}
	cfg, err := config.Load(*dir)
	if err != nil {
		log.Fatal(err)
	}

	st, err := store.Open(filepath.Join(*dir, "data"))
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	defs, defErrs := apps.LoadDefs(bundledAppsDir(), cfg.AppDefDir)
	for _, e := range defErrs {
		log.Println("app def:", e)
	}

	hub := api.NewHub()
	col := collector.New(cfg, st, defs, hub)
	alerter := alerts.New(cfg)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go col.Run(ctx)
	go func() {
		t := time.NewTicker(time.Duration(cfg.PollSeconds) * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				snap := st.Snapshot()
				ev := alerter.Evaluate(snap)
				if len(ev) > 0 {
					snap.Alerts = ev
					st.SetSnapshot(snap)
					hub.Broadcast(snap)
				}
			}
		}
	}()
	go watchConfig(*dir, func() {
		newCfg, err := config.Load(*dir)
		if err != nil {
			log.Println("reload config:", err)
			return
		}
		newDefs, errs := apps.LoadDefs(bundledAppsDir(), newCfg.AppDefDir)
		for _, e := range errs {
			log.Println("app def:", e)
		}
		col.SetConfig(newCfg, newDefs)
		alerter.SetConfig(newCfg)
		cfg = newCfg
		defs = newDefs
		log.Println("reloaded config")
		col.CollectOnce()
	})

	uiFS, err := embedui.FS()
	if err != nil {
		log.Fatal(err)
	}
	var currentCfg = cfg
	var currentDefs = defs
	srvAPI := &api.Server{
		Cfg:  func() *config.Config { return currentCfg },
		Store: st,
		Defs: func() map[string]*apps.Def { return currentDefs },
		Hub:  hub,
		UI:   uiFS,
	}
	// keep closures updated
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
				currentCfg = cfg
				currentDefs = defs
			}
		}
	}()

	httpSrv := &http.Server{Addr: cfg.Listen, Handler: srvAPI.Handler()}
	go func() {
		log.Println("listening on", cfg.Listen)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdown, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	_ = httpSrv.Shutdown(shutdown)
	return 0
}

func bundledAppsDir() string {
	candidates := []string{
		"apps",
		"/app/apps",
		filepath.Join(filepath.Dir(os.Args[0]), "apps"),
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c
		}
	}
	return "apps"
}

func watchConfig(dir string, onChange func()) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	_ = w.Add(dir)
	_ = w.Add(filepath.Join(dir, "apps.d"))
	var timer *time.Timer
	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(400*time.Millisecond, onChange)
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
		}
	}
}
