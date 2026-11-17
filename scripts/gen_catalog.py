#!/usr/bin/env python3
"""Generate popular apps catalog from dashboard-icons + selfhosted survey + known images."""
from __future__ import annotations

import json
import os
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "internal" / "apps" / "catalog" / "popular.json"

# Common docker image path aliases for popular apps (image substring -> icon/id overrides)
IMAGE_ALIASES: dict[str, dict] = {
    "jellyfin/jellyfin": {"id": "jellyfin", "name": "Jellyfin", "icon": "jellyfin"},
    "homeassistant/home-assistant": {"id": "homeassistant", "name": "Home Assistant", "icon": "home-assistant"},
    "home-assistant": {"id": "homeassistant", "name": "Home Assistant", "icon": "home-assistant"},
    "vaultwarden/server": {"id": "vaultwarden", "name": "Vaultwarden", "icon": "vaultwarden"},
    "ghcr.io/immich-app/immich-server": {"id": "immich", "name": "Immich", "icon": "immich"},
    "ghcr.io/immich-app/immich-machine-learning": {"id": "immich-ml", "name": "Immich ML", "icon": "immich"},
    "plexinc/pms-docker": {"id": "plex", "name": "Plex", "icon": "plex"},
    "linuxserver/plex": {"id": "plex", "name": "Plex", "icon": "plex"},
    "nextcloud": {"id": "nextcloud", "name": "Nextcloud", "icon": "nextcloud"},
    "linuxserver/nextcloud": {"id": "nextcloud", "name": "Nextcloud", "icon": "nextcloud"},
    "lscr.io/linuxserver/sonarr": {"id": "sonarr", "name": "Sonarr", "icon": "sonarr"},
    "linuxserver/sonarr": {"id": "sonarr", "name": "Sonarr", "icon": "sonarr"},
    "lscr.io/linuxserver/radarr": {"id": "radarr", "name": "Radarr", "icon": "radarr"},
    "linuxserver/radarr": {"id": "radarr", "name": "Radarr", "icon": "radarr"},
    "lscr.io/linuxserver/lidarr": {"id": "lidarr", "name": "Lidarr", "icon": "lidarr"},
    "lscr.io/linuxserver/prowlarr": {"id": "prowlarr", "name": "Prowlarr", "icon": "prowlarr"},
    "lscr.io/linuxserver/bazarr": {"id": "bazarr", "name": "Bazarr", "icon": "bazarr"},
    "adguard/adguardhome": {"id": "adguardhome", "name": "AdGuard Home", "icon": "adguard-home"},
    "pihole/pihole": {"id": "pihole", "name": "Pi-hole", "icon": "pi-hole"},
    "ghcr.io/paperless-ngx/paperless-ngx": {"id": "paperless-ngx", "name": "Paperless-ngx", "icon": "paperless-ngx"},
    "gitea/gitea": {"id": "gitea", "name": "Gitea", "icon": "gitea"},
    "codeberg.org/forgejo/forgejo": {"id": "forgejo", "name": "Forgejo", "icon": "forgejo"},
    "portainer/portainer-ce": {"id": "portainer", "name": "Portainer", "icon": "portainer"},
    "portainer/portainer-ee": {"id": "portainer", "name": "Portainer", "icon": "portainer"},
    "louislam/uptime-kuma": {"id": "uptime-kuma", "name": "Uptime Kuma", "icon": "uptime-kuma"},
    "ghcr.io/mydraft-cc/server": {"id": "mydraft", "name": "MyDraft", "icon": "docker"},
    "caddy": {"id": "caddy", "name": "Caddy", "icon": "caddy"},
    "caddybuilds/caddy-cloudflare": {"id": "caddy", "name": "Caddy", "icon": "caddy"},
    "nginx": {"id": "nginx", "name": "Nginx", "icon": "nginx"},
    "nginxproxy/nginx-proxy-manager": {"id": "nginx-proxy-manager", "name": "Nginx Proxy Manager", "icon": "nginx-proxy-manager"},
    "jc21/nginx-proxy-manager": {"id": "nginx-proxy-manager", "name": "Nginx Proxy Manager", "icon": "nginx-proxy-manager"},
    "grafana/grafana": {"id": "grafana", "name": "Grafana", "icon": "grafana"},
    "prom/prometheus": {"id": "prometheus", "name": "Prometheus", "icon": "prometheus"},
    "prom/node-exporter": {"id": "node-exporter", "name": "Node Exporter", "icon": "prometheus"},
    "influxdb": {"id": "influxdb", "name": "InfluxDB", "icon": "influxdb"},
    "redis": {"id": "redis", "name": "Redis", "icon": "redis"},
    "postgres": {"id": "postgres", "name": "PostgreSQL", "icon": "postgresql"},
    "postgres:": {"id": "postgres", "name": "PostgreSQL", "icon": "postgresql"},
    "mysql": {"id": "mysql", "name": "MySQL", "icon": "mysql"},
    "mariadb": {"id": "mariadb", "name": "MariaDB", "icon": "mariadb"},
    "mongo": {"id": "mongodb", "name": "MongoDB", "icon": "mongodb"},
    "ghcr.io/kdpuvvadi/dash": {"id": "dash", "name": "Dash", "icon": "docker"},
    "gitea/act_runner": {"id": "act-runner", "name": "Gitea Act Runner", "icon": "githubactions"},
    "gitea/runner": {"id": "gitea-runner", "name": "Gitea Actions Runner", "icon": "githubactions"},
    "ghcr.io/kdpuvvadi/otf-agent": {"id": "otf-agent", "name": "OpenTofu Agent", "icon": "opentofu"},
    "kdpuvvadi/otf-agent": {"id": "otf-agent", "name": "OpenTofu Agent", "icon": "opentofu"},
    "kdpuvvadi/dash": {"id": "dash", "name": "Dash", "icon": "docker"},
    "ghcr.io/actions/actions-runner": {"id": "github-runner", "name": "GitHub Actions Runner", "icon": "githubactions"},
    "myoung34/github-runner": {"id": "github-runner", "name": "GitHub Actions Runner", "icon": "githubactions"},
    "summerwind/actions-runner": {"id": "github-runner", "name": "GitHub Actions Runner", "icon": "githubactions"},
    "ghcr.io/open-webui/open-webui": {"id": "open-webui", "name": "Open WebUI", "icon": "ollama"},
    "ollama/ollama": {"id": "ollama", "name": "Ollama", "icon": "ollama"},
    "ghcr.io/open-telemetry/opentelemetry-collector": {"id": "otel-collector", "name": "OpenTelemetry Collector", "icon": "opentelemetry"},
    "traefik": {"id": "traefik", "name": "Traefik", "icon": "traefik"},
    "authentik": {"id": "authentik", "name": "Authentik", "icon": "authentik"},
    "ghcr.io/goauthentik/server": {"id": "authentik", "name": "Authentik", "icon": "authentik"},
    "authelia/authelia": {"id": "authelia", "name": "Authelia", "icon": "authelia"},
    "ghcr.io/blakeblackshear/frigate": {"id": "frigate", "name": "Frigate", "icon": "frigate"},
    "ghcr.io/home-assistant/home-assistant": {"id": "homeassistant", "name": "Home Assistant", "icon": "home-assistant"},
    "esphome/esphome": {"id": "esphome", "name": "ESPHome", "icon": "esphome"},
    "koenkk/zigbee2mqtt": {"id": "zigbee2mqtt", "name": "Zigbee2MQTT", "icon": "zigbee2mqtt"},
    "eclipse-mosquitto": {"id": "mosquitto", "name": "Mosquitto", "icon": "mqtt"},
    "n8nio/n8n": {"id": "n8n", "name": "n8n", "icon": "n8n"},
    "ghcr.io/actualbudget/actual-server": {"id": "actual", "name": "Actual Budget", "icon": "actual-budget"},
    "fireflyiii/core": {"id": "firefly-iii", "name": "Firefly III", "icon": "firefly-iii"},
    "bookstackapp/bookstack": {"id": "bookstack", "name": "BookStack", "icon": "bookstack"},
    "linuxserver/bookstack": {"id": "bookstack", "name": "BookStack", "icon": "bookstack"},
    "filebrowser/filebrowser": {"id": "filebrowser", "name": "File Browser", "icon": "filebrowser"},
    "linuxserver/qbittorrent": {"id": "qbittorrent", "name": "qBittorrent", "icon": "qbittorrent"},
    "linuxserver/transmission": {"id": "transmission", "name": "Transmission", "icon": "transmission"},
    "binhex/arch-sabnzbd": {"id": "sabnzbd", "name": "SABnzbd", "icon": "sabnzbd"},
    "linuxserver/sabnzbd": {"id": "sabnzbd", "name": "SABnzbd", "icon": "sabnzbd"},
    "ghcr.io/meeb/tubearchivist": {"id": "tubearchivist", "name": "Tube Archivist", "icon": "tubearchivist"},
    "alexta69/metube": {"id": "metube", "name": "MeTube", "icon": "youtube"},
    "deluan/navidrome": {"id": "navidrome", "name": "Navidrome", "icon": "navidrome"},
    "ghcr.io/advplyr/audiobookshelf": {"id": "audiobookshelf", "name": "Audiobookshelf", "icon": "audiobookshelf"},
    "ghcr.io/calcom/cal.com": {"id": "calcom", "name": "Cal.com", "icon": "calcom"},
    "matrixdotorg/synapse": {"id": "matrix-synapse", "name": "Matrix Synapse", "icon": "matrix"},
    "vectorim/element-web": {"id": "element", "name": "Element", "icon": "element"},
    "ghcr.io/binwiederhier/ntfy": {"id": "ntfy", "name": "ntfy", "icon": "ntfy"},
    "minio/minio": {"id": "minio", "name": "MinIO", "icon": "minio"},
    "emby/embyserver": {"id": "emby", "name": "Emby", "icon": "emby"},
    "lscr.io/linuxserver/emby": {"id": "emby", "name": "Emby", "icon": "emby"},
    "ghcr.io/getumbrel/umbrel": {"id": "umbrel", "name": "Umbrel", "icon": "umbrel"},
    "dockurr/windows": {"id": "windows", "name": "Windows", "icon": "windows"},
    "qmcgaw/gluetun": {"id": "gluetun", "name": "Gluetun", "icon": "gluetun"},
    "ghcr.io/dockur/casaos": {"id": "casaos", "name": "CasaOS", "icon": "casaos"},
    "containrrr/watchtower": {"id": "watchtower", "name": "Watchtower", "icon": "watchtower"},
    "ghcr.io/wzshiming/dockge": {"id": "dockge", "name": "Dockge", "icon": "dockge"},
    "louislam/dockge": {"id": "dockge", "name": "Dockge", "icon": "dockge"},
    "ghcr.io/ajnart/homarr": {"id": "homarr", "name": "Homarr", "icon": "homarr"},
    "ghcr.io/gethomepage/homepage": {"id": "homepage", "name": "Homepage", "icon": "homepage"},
    "ghcr.io/flame-dashboard/flame": {"id": "flame", "name": "Flame", "icon": "flame"},
    "linuxserver/heimdall": {"id": "heimdall", "name": "Heimdall", "icon": "heimdall"},
    "ghcr.io/dani-garcia/vaultwarden": {"id": "vaultwarden", "name": "Vaultwarden", "icon": "vaultwarden"},
    "searxng/searxng": {"id": "searxng", "name": "SearXNG", "icon": "searxng"},
    "ghcr.io/librespeed/speedtest": {"id": "librespeed", "name": "LibreSpeed", "icon": "speedtest"},
    "speedtest": {"id": "librespeed", "name": "LibreSpeed", "icon": "speedtest"},
    "ghcr.io/openfoodfacts": {"id": "openfoodfacts", "name": "Open Food Facts", "icon": "docker"},
    "wordpress": {"id": "wordpress", "name": "WordPress", "icon": "wordpress"},
    "ghost": {"id": "ghost", "name": "Ghost", "icon": "ghost"},
    "gitlab/gitlab-ce": {"id": "gitlab", "name": "GitLab", "icon": "gitlab"},
    "jenkins/jenkins": {"id": "jenkins", "name": "Jenkins", "icon": "jenkins"},
    "sonatype/nexus3": {"id": "nexus", "name": "Nexus", "icon": "sonatype"},
    "registry:": {"id": "docker-registry", "name": "Docker Registry", "icon": "docker"},
    "registry:2": {"id": "docker-registry", "name": "Docker Registry", "icon": "docker"},
    "cloudflare/cloudflared": {"id": "cloudflared", "name": "Cloudflared", "icon": "cloudflare"},
    "tailscale/tailscale": {"id": "tailscale", "name": "Tailscale", "icon": "tailscale"},
    "zerotier/zerotier": {"id": "zerotier", "name": "ZeroTier", "icon": "zerotier"},
    "netbirdio/netbird": {"id": "netbird", "name": "NetBird", "icon": "netbird"},
    "wg-easy/wg-easy": {"id": "wg-easy", "name": "WireGuard Easy", "icon": "wireguard"},
    "linuxserver/wireguard": {"id": "wireguard", "name": "WireGuard", "icon": "wireguard"},
    "ghcr.io/headscale/headscale": {"id": "headscale", "name": "Headscale", "icon": "headscale"},
    "photoprism/photoprism": {"id": "photoprism", "name": "PhotoPrism", "icon": "photoprism"},
    "lycheeorg/lychee": {"id": "lychee", "name": "Lychee", "icon": "lychee"},
    "ghcr.io/usememos/memos": {"id": "memos", "name": "Memos", "icon": "memos"},
    "outlinewiki/outline": {"id": "outline", "name": "Outline", "icon": "outline"},
    "ghcr.io/requarks/wiki": {"id": "wikijs", "name": "Wiki.js", "icon": "wikijs"},
    "ghcr.io/corentinth/it-tools": {"id": "it-tools", "name": "IT Tools", "icon": "it-tools"},
    "ghcr.io/stirling-tools/stirling-pdf": {"id": "stirling-pdf", "name": "Stirling PDF", "icon": "stirling-pdf"},
    "changedetection/changedetection.io": {"id": "changedetection", "name": "Changedetection", "icon": "changedetection"},
    "rsshub/rsshub": {"id": "rsshub", "name": "RSSHub", "icon": "rsshub"},
    "freshrss/freshrss": {"id": "freshrss", "name": "FreshRSS", "icon": "freshrss"},
    "miniflux/miniflux": {"id": "miniflux", "name": "Miniflux", "icon": "miniflux"},
    "theotherp/nzbhydra2": {"id": "nzbhydra2", "name": "NZBHydra2", "icon": "nzbhydra2"},
    "fallenbagel/jellyseerr": {"id": "jellyseerr", "name": "Jellyseerr", "icon": "jellyseerr"},
    "sctx/overseerr": {"id": "overseerr", "name": "Overseerr", "icon": "overseerr"},
    "tautulli/tautulli": {"id": "tautulli", "name": "Tautulli", "icon": "tautulli"},
    "ghcr.io/flaresolverr/flaresolverr": {"id": "flaresolverr", "name": "Flaresolverr", "icon": "flaresolverr"},
    "ghcr.io/homebridge/homebridge": {"id": "homebridge", "name": "Homebridge", "icon": "homebridge"},
    "zwavejs/zwave-js-ui": {"id": "zwavejsui", "name": "Z-Wave JS UI", "icon": "zwavejs"},
    "ghcr.io/mealie-recipes/mealie": {"id": "mealie", "name": "Mealie", "icon": "mealie"},
    "ghcr.io/tandoorrecipes/recipes": {"id": "tandoor", "name": "Tandoor", "icon": "tandoor"},
    "grocy/grocy": {"id": "grocy", "name": "Grocy", "icon": "grocy"},
    "ghcr.io/vikunja/vikunja": {"id": "vikunja", "name": "Vikunja", "icon": "vikunja"},
    "planka": {"id": "planka", "name": "Planka", "icon": "planka"},
    "wekan/wekan": {"id": "wekan", "name": "Wekan", "icon": "wekan"},
    "mattermost/mattermost-team-edition": {"id": "mattermost", "name": "Mattermost", "icon": "mattermost"},
    "rocket.chat": {"id": "rocketchat", "name": "Rocket.Chat", "icon": "rocket-dot-chat"},
    "discord": {"id": "discord", "name": "Discord", "icon": "discord"},
    "mailcow/mailcow": {"id": "mailcow", "name": "Mailcow", "icon": "mailcow"},
    "mailserver/docker-mailserver": {"id": "docker-mailserver", "name": "Docker Mailserver", "icon": "mail"},
    "roundcube/roundcubemail": {"id": "roundcube", "name": "Roundcube", "icon": "roundcube"},
    "ghcr.io/gotify/server": {"id": "gotify", "name": "Gotify", "icon": "gotify"},
    "ghcr.io/karakeep-app/karakeep": {"id": "karakeep", "name": "Karakeep", "icon": "karakeep"},
    "linkding": {"id": "linkding", "name": "linkding", "icon": "linkding"},
    "wallabag/wallabag": {"id": "wallabag", "name": "Wallabag", "icon": "wallabag"},
    "shlinkio/shlink": {"id": "shlink", "name": "Shlink", "icon": "shlink"},
    "yourls/yourls": {"id": "yourls", "name": "YOURLS", "icon": "yourls"},
    "umami": {"id": "umami", "name": "Umami", "icon": "umami"},
    "matomo": {"id": "matomo", "name": "Matomo", "icon": "matomo"},
    "plausible/analytics": {"id": "plausible", "name": "Plausible", "icon": "plausible"},
    "ghcr.io/cortexprojects/dashdot": {"id": "dashdot", "name": "Dashdot", "icon": "dashdot"},
    "netdata/netdata": {"id": "netdata", "name": "Netdata", "icon": "netdata"},
    "glances": {"id": "glances", "name": "Glances", "icon": "glances"},
    "ghcr.io/analogj/scrutiny": {"id": "scrutiny", "name": "Scrutiny", "icon": "scrutiny"},
    "duplicati/duplicati": {"id": "duplicati", "name": "Duplicati", "icon": "duplicati"},
    "restic/rest-server": {"id": "restic", "name": "Restic REST Server", "icon": "restic"},
    "kopia/kopia": {"id": "kopia", "name": "Kopia", "icon": "kopia"},
    "borgmatic": {"id": "borgmatic", "name": "Borgmatic", "icon": "borg"},
    "syncthing/syncthing": {"id": "syncthing", "name": "Syncthing", "icon": "syncthing"},
    "linuxserver/syncthing": {"id": "syncthing", "name": "Syncthing", "icon": "syncthing"},
    "seafileltd/seafile": {"id": "seafile", "name": "Seafile", "icon": "seafile"},
    "owncloud/server": {"id": "owncloud", "name": "ownCloud", "icon": "owncloud"},
    "onlyoffice/documentserver": {"id": "onlyoffice", "name": "ONLYOFFICE", "icon": "onlyoffice"},
    "collabora/code": {"id": "collabora", "name": "Collabora", "icon": "collabora"},
    "jitsi/web": {"id": "jitsi", "name": "Jitsi", "icon": "jitsi"},
    "bigbluebutton": {"id": "bigbluebutton", "name": "BigBlueButton", "icon": "bigbluebutton"},
    "guacamole/guacamole": {"id": "guacamole", "name": "Apache Guacamole", "icon": "apacheguacamole"},
    "linuxserver/code-server": {"id": "code-server", "name": "code-server", "icon": "coder"},
    "codercom/code-server": {"id": "code-server", "name": "code-server", "icon": "coder"},
    "gitpod/openvscode-server": {"id": "openvscode", "name": "OpenVSCode", "icon": "visualstudiocode"},
    "ghcr.io/coder/code-server": {"id": "code-server", "name": "code-server", "icon": "coder"},
    "theiaide/theia": {"id": "theia", "name": "Eclipse Theia", "icon": "eclipse"},
    "portainer/agent": {"id": "portainer-agent", "name": "Portainer Agent", "icon": "portainer"},
    "dozzle": {"id": "dozzle", "name": "Dozzle", "icon": "dozzle"},
    "amir20/dozzle": {"id": "dozzle", "name": "Dozzle", "icon": "dozzle"},
    "ghcr.io/corentinth/homarr": {"id": "homarr", "name": "Homarr", "icon": "homarr"},
    "mongo-express": {"id": "mongo-express", "name": "Mongo Express", "icon": "mongodb"},
    "adminer": {"id": "adminer", "name": "Adminer", "icon": "adminer"},
    "phpmyadmin": {"id": "phpmyadmin", "name": "phpMyAdmin", "icon": "phpmyadmin"},
    "dpage/pgadmin4": {"id": "pgadmin", "name": "pgAdmin", "icon": "pgadmin"},
    "cloudflare/cloudflared:": {"id": "cloudflared", "name": "Cloudflared", "icon": "cloudflare"},
    "otf-agent": {"id": "opentofu", "name": "OpenTofu Agent", "icon": "opentofu"},
    "ghcr.io/opentofu": {"id": "opentofu", "name": "OpenTofu", "icon": "opentofu"},
    "hashicorp/terraform": {"id": "terraform", "name": "Terraform", "icon": "terraform"},
    "hashicorp/vault": {"id": "vault", "name": "HashiCorp Vault", "icon": "vault"},
    "hashicorp/consul": {"id": "consul", "name": "Consul", "icon": "consul"},
    "hashicorp/nomad": {"id": "nomad", "name": "Nomad", "icon": "nomad"},
    "rabbitmq": {"id": "rabbitmq", "name": "RabbitMQ", "icon": "rabbitmq"},
    "apache/kafka": {"id": "kafka", "name": "Kafka", "icon": "apachekafka"},
    "nats": {"id": "nats", "name": "NATS", "icon": "nats"},
    "elasticsearch": {"id": "elasticsearch", "name": "Elasticsearch", "icon": "elasticsearch"},
    "opensearchproject/opensearch": {"id": "opensearch", "name": "OpenSearch", "icon": "opensearch"},
    "kibana": {"id": "kibana", "name": "Kibana", "icon": "kibana"},
    "graylog/graylog": {"id": "graylog", "name": "Graylog", "icon": "graylog"},
    "grafana/loki": {"id": "loki", "name": "Loki", "icon": "grafana"},
    "grafana/promtail": {"id": "promtail", "name": "Promtail", "icon": "grafana"},
    "jaegertracing/all-in-one": {"id": "jaeger", "name": "Jaeger", "icon": "jaeger"},
    "otel/opentelemetry-collector": {"id": "otel-collector", "name": "OpenTelemetry Collector", "icon": "opentelemetry"},
    "selenium/standalone": {"id": "selenium", "name": "Selenium", "icon": "selenium"},
    "browserless/chrome": {"id": "browserless", "name": "Browserless", "icon": "chrome"},
    "gotenberg/gotenberg": {"id": "gotenberg", "name": "Gotenberg", "icon": "gotenberg"},
    "tianon/true-myth": {"id": "docker", "name": "Docker", "icon": "docker"},
}


def slugify(name: str) -> str:
    s = name.lower().strip()
    s = s.replace("+", "plus")
    s = re.sub(r"[^a-z0-9]+", "-", s)
    return s.strip("-")


def title_from_slug(slug: str) -> str:
    parts = slug.replace("_", "-").split("-")
    special = {
        "js": "JS",
        "ui": "UI",
        "api": "API",
        "ml": "ML",
        "ai": "AI",
        "tv": "TV",
        "vpn": "VPN",
        "dns": "DNS",
        "ssh": "SSH",
        "pdf": "PDF",
        "rss": "RSS",
        "mqtt": "MQTT",
        "ngx": "ngx",
        "db": "DB",
    }
    out = []
    for p in parts:
        out.append(special.get(p, p.capitalize()))
    return " ".join(out)


def load_icons() -> set[str]:
    path = os.environ.get("TEMP") or os.environ.get("TMP") or "/tmp"
    tree_path = Path(path) / "di-tree.json"
    icons: set[str] = set()
    if tree_path.exists():
        tree = json.loads(tree_path.read_text(encoding="utf-8"))
        for t in tree.get("tree", []):
            p = t.get("path", "")
            if p.startswith("svg/") and p.endswith(".svg"):
                icons.add(Path(p).stem.lower())
    return icons


def load_survey() -> list[str]:
    survey = Path(
        r"C:\Users\SSAS\.cursor\projects\c-Data-add-code-dash\agent-tools\9bda2a6d-fe0f-4eae-a983-bf1ba0a9b6b1.txt"
    )
    if not survey.exists():
        return []
    apps = []
    for m in re.finditer(r"^## (.+)$", survey.read_text(encoding="utf-8"), re.M):
        name = m.group(1).strip()
        if name.lower() in {"mvp rankings"}:
            continue
        apps.append(name)
    return apps


def linuxserver_apps() -> list[str]:
    # Popular linuxserver image names (homelab survey + common)
    return [
        "plex", "sonarr", "radarr", "lidarr", "bazarr", "prowlarr", "jellyfin", "emby",
        "qbittorrent", "transmission", "sabnzbd", "nzbget", "deluge", "rutorrent",
        "nextcloud", "heimdall", "organizr", "ombi", "lazylibrarian", "mylar3",
        "readarr", "whisparr", "calibre", "calibre-web", "bookstack", "wikijs",
        "pihole", "unbound", "wireguard", "openvpn-as", "swag", "nginx", "duckdns",
        "letsencrypt", "mariadb", "mysql", "postgres", "redis", "mongodb",
        "homeassistant", "code-server", "docker-compose", "duplicati", "syncthing",
        "resilio-sync", "tt-rss", "freshrss", "minisatip", "tvheadend", "oscam",
        "sickchill", "medusa", "headphones", "beets", "airsonic", "navidrome",
        "lychee", "piwigo", "photoprism", "digikam", "doublecommander", "krusader",
        "filebrowser", "projectsend", "pydio", "seafile", "owncloud", "snapdrop",
        "pairdrop", "vaultwarden", "pwndrop", "dashy", "homarr", "flame",
        "uptime-kuma", "speedtest-tracker", "smokeping", "netdata", "scrutiny",
        "glances", "portainer", "yacht", "dockge", "watchtower", "diun",
        "adguardhome-sync", "cloudcmd", "filezilla", "putty", "remmina",
        "guacd", "guacamole", "webtop", "rdesktop", "kasmweb", "desktop",
        "chromium", "firefox", "opera", "brave", "edge", "tor", "privoxy",
        "sabnzbd", "nzbhydra2", "jackett", "flaresolverr", "tdarr", "unmanic",
        "handbrake", "ffmpeg", "makemkv", "makemkvcon", "mkvtoolnix",
        "obsidian", "joplin", "standardnotes", "trilium", "appflowy",
        "affine", "outline", "notion", "siyuan", "logseq", "anytype",
        "immich", "ente", "librephotos", "pigallery2", "photoview",
        "mealie", "tandoor", "grocy", "barassistant", "karakeep",
        "linkwarden", "linkding", "wallabag", "shiori", "omnivore",
        "paperless-ngx", "papermerge", "docspell", "mayan", "stirling-pdf",
        "it-tools", "cyberchef", "excalidraw", "drawio", "penpot",
        "plausible", "umami", "matomo", "goatcounter", "ackee",
        "ghost", "wordpress", "ghostfolio", "actual", "fireflyiii",
        "invoiceninja", "akaunting", "kimai", "solidtime",
        "vikunja", "vikunja-api", "wekan", "planka", "kanboard", "focalboard",
        "mattermost", "rocket.chat", "zulip", "revolt", "element", "synapse",
        "dendrite", "conduit", "gotosocial", "mastodon", "pixelfed", "lemmy",
        "peertube", "owncast", "livekit", "jitsi", "bigbluebutton",
        "mailcow", "mailu", "docker-mailserver", "postalserver", "stalwart",
        "roundcube", "snappymail", "rainloop", "postfixadmin",
        "minio", "garage", "seaweedfs", "ceph", "longhorn",
        "prometheus", "grafana", "loki", "tempo", "mimir", "alertmanager",
        "thanos", "victoriametrics", "influxdb", "telegraf", "chronograf",
        "elasticsearch", "opensearch", "kibana", "graylog", "vector",
        "traefik", "caddy", "haproxy", "nginx-proxy-manager", "pangolin",
        "authentik", "authelia", "keycloak", "oauth2-proxy", "pocket-id",
        "lldap", "freeipa", "samba", "webdav", "rclone",
        "frigate", "scrypted", "motioneye", "zoneminder", "shinobi",
        "esphome", "zigbee2mqtt", "zwavejs2mqtt", "deconz", "homebridge",
        "node-red", "n8n", "activepieces", "windmill", "huginn",
        "changedetection", "rsshub", "miniflux", "freshrss", "stringer",
        "audiobookshelf", "kavita", "komga", "lazylibrarian", "readarr",
        "calibre-web", "bookstack", "wikijs", "dokuwiki", "mediawiki",
        "outline", "docmost", "affine", "siyuan", "anytype",
        "searxng", "whoogle", "4get", "libreddit", "redlib", "nitter",
        "invidious", "piped", "tubearchivist", "metube", "pinchflat",
        "ollama", "open-webui", "localai", "text-generation-webui", "invokeai",
        "stable-diffusion", "comfyui", "automatic1111", "fooocus",
        "qbittorrent", "deluge", "rtorrent", "flood", "transmission",
        "jdownloader", "pyload", "aria2", "youtubedl-material",
        "speedtest-tracker", "librespeed", "smokeping", "pingvin-share",
        "pairdrop", "localsend", "snapdrop", "filebrowser", "filestash",
        "projectsend", "pydio", "nextcloud-aio", "owncloud-infinite-scale",
        "collabora", "onlyoffice", "cryptpad", "privatebin", "hastebin",
        "pastefy", "microbin", "wastebin", "flaresolverr", "capsolver",
        "gitea", "forgejo", "gitlab", "gogs", "onedev", "drone", "woodpecker",
        "jenkins", "teamcity", "buildkite", "act-runner", "github-runner",
        "semaphore", "ansible-semaphore", "awx", "rundeck", "tower",
        "terraform", "opentofu", "pulumi", "packer", "vagrant",
        "vault", "consul", "nomad", "boundary", "waypoint",
        "docker-registry", "harbor", "nexus", "artifactory", "verdaccio",
        "portainer-ce", "yacht", "dockge", "lazydocker", "ctop", "dive",
        "cadvisor", "node-exporter", "dcgm-exporter", "process-exporter",
        "blackbox-exporter", "snmp-exporter", "cloudprober",
        "uptime-kuma", "gatus", "healthchecks", "cachet", "status-page",
        "ntfy", "gotify", "apprise", "shoutrrr", "bark", "pushbits",
        "matrix-alertmanager", "grafana-oncall", "opsgenie",
        "home-assistant", "hass", "hassio", "supervisor",
        "mosquitto", "emqx", "vernemq", "nanomq", "rabbitmq", "nats",
        "kafka", "redpanda", "pulsar", "centrifugo",
        "postgres", "pgvector", "pgadmin", "adminer", "phpmyadmin",
        "mysql", "mariadb", "percona", "tidb", "cockroachdb",
        "mongodb", "mongo-express", "redis", "keydb", "dragonfly", "valkey",
        "memcached", "hazelcast", "cassandra", "scylladb", "clickhouse",
        "timescaledb", "questdb", "duckdb", "sqlite", "litestream",
        "meilisearch", "typesense", "sonic", "zincsearch", "quickwit",
        "weaviate", "qdrant", "milvus", "chroma", "lancedb",
        "neo4j", "dgraph", "arangodb", "orientdb",
        "wordpress", "ghost", "hugo", "jekyll", "hexo", "astro",
        "directus", "strapi", "payload", "sanity", "contentful",
        "keycloak", "ory-hydra", "ory-kratos", "supabase", "appwrite",
        "pocketbase", "nocodb", "baserow", "rowy", "appsmith", "tooljet",
        "budibase", "retool", "illuminate", "windmill",
        "metabase", "superset", "redash", "lightdash", "evidence",
        "posthog", "plausible", "umami", "matomo", "ackee", "goatcounter",
        "sentry", "glitchtip", "highlight", "signoz", "openobserve",
        "langfuse", "helicone", "litellm", "one-api", "new-api",
        "dify", "fastgpt", "langchain", "anythingllm", "librechat",
        "sillytavern", "text-generation-webui", "oobabooga",
        "comfyui", "invokeai", "automatic1111", "fooocus", "sd-webui",
        "whisper", "faster-whisper", "coqui", "piper", "xtts",
        "whisper-asr-webservice", "speaches",
        "stirling-pdf", "gotenberg", "paperless-ngx", "papermerge",
        "mayanedms", "docspell", "tagspaces", "tagflow",
        "actualbudget", "firefly-iii", "ghostfolio", "maybe", "wallos",
        "invoiceninja", "crater", "kimai", "solidtime", "everhour",
        "vikunja", "todoist", "ticktick", "habitica", "loop",
        "bookstack", "wikijs", "outline", "docmost", "affine", "notion-like",
        "siyuan", "anytype", "logseq", "obsidian-livesync", "trilium",
        "standardnotes", "joplin-server", "appflowy", "notesnook",
        "karakeep", "linkwarden", "linkding", "shiori", "wallabag", "omnivore",
        "readeck", "cubox", "raindrop",
        "immich", "photoprism", "lychee", "librephotos", "ente", "pigallery2",
        "photoview", "piwigo", "zenphoto", "coppermine",
        "jellyfin", "plex", "emby", "navidrome", "funkwhale", "ampache",
        "airsonic-advanced", "gonic", "koel", "feishin", "finamp",
        "audiobookshelf", "kavita", "komga", "lazylibrarian", "readarr",
        "calibre-web", "ebook2audiobook", "storyteller",
        "sonarr", "radarr", "lidarr", "bazarr", "prowlarr", "whisparr",
        "jellyseerr", "overseerr", "ombi", "requestrr", "notifiarr",
        "tautulli", "maintainerr", "kometa", "pulsarr", "recyclarr",
        "tdarr", "unmanic", "fileflows", "handbrake", "ffmpeg",
        "tubearchivist", "metube", "pinchflat", "youtubedl-material", "meTube",
        "invidious", "piped", "libre-tube", "freetube",
        "qbittorrent", "transmission", "deluge", "rtorrent", "flood", "rutorrent",
        "sabnzbd", "nzbget", "nzbhydra2", "jackett", "prowlarr", "flaresolverr",
        "gluetun", "wireguard", "openvpn", "tailscale", "headscale", "netbird",
        "zerotier", "nebula", "wg-easy", "pivpn", "algo",
        "adguardhome", "pihole", "blocky", "technitium", "unbound", "coredns",
        "powerdns", "bind9", "knot-resolver",
        "caddy", "traefik", "nginx", "nginx-proxy-manager", "haproxy", "envoy",
        "cloudflared", "pangolin", "boringproxy", "frp", "nps", "rathole",
        "authentik", "authelia", "keycloak", "pocket-id", "oauth2-proxy", "lldap",
        "portainer", "dockge", "yacht", "homarr", "homepage", "heimdall", "dashy", "flame",
        "uptime-kuma", "gatus", "healthchecks", "changedetection", "beszel", "glance",
        "netdata", "glances", "scrutiny", "cadvisor", "dozzle", "lazydocker",
        "prometheus", "grafana", "loki", "promtail", "tempo", "mimir", "alloy",
        "vaultwarden", "bitwarden", "passbolt", "infisical", "doppler",
        "gitea", "forgejo", "gitlab", "gogs", "onedev", "woodpecker", "drone",
        "jenkins", "act-runner", "github-runner", "semaphore", "awx",
        "nextcloud", "owncloud", "seafile", "syncthing", "rclone", "duplicati", "kopia", "restic",
        "minio", "garage", "filebrowser", "filestash", "projectsend", "pairdrop", "localsend",
        "paperless-ngx", "stirling-pdf", "gotenberg", "it-tools", "cyberchef", "excalidraw", "drawio",
        "mealie", "tandoor", "grocy", "actual", "firefly-iii", "ghostfolio", "wallos",
        "n8n", "node-red", "activepieces", "windmill", "huginn", "home-assistant", "frigate", "esphome",
        "zigbee2mqtt", "zwave-js-ui", "mosquitto", "homebridge", "scrypted",
        "ollama", "open-webui", "localai", "dify", "anythingllm", "librechat", "open-webui",
        "searxng", "whoogle", "rsshub", "miniflux", "freshrss", "ntfy", "gotify", "apprise",
        "matrix-synapse", "element-web", "gotosocial", "mastodon", "lemmy", "peertube", "owncast",
        "jitsi", "livekit", "bigbluebutton", "guacamole", "code-server", "openvscode-server",
        "wordpress", "ghost", "umami", "plausible", "matomo", "posthog", "sentry", "glitchtip",
        "directus", "strapi", "appwrite", "supabase", "pocketbase", "nocodb", "baserow", "appsmith",
        "metabase", "superset", "redash", "langfuse", "signoz", "openobserve",
        "postgres", "mariadb", "mysql", "mongodb", "redis", "rabbitmq", "elasticsearch", "opensearch",
        "meilisearch", "typesense", "qdrant", "weaviate", "milvus", "neo4j", "clickhouse", "timescaledb",
        "mailcow", "docker-mailserver", "stalwart", "roundcube", "snappymail",
        "watchtower", "diun", "containrrr", "regclient", "skopeo",
        "speedtest-tracker", "librespeed", "smokeping", "cloudprober", "blackbox-exporter",
        "node-exporter", "cadvisor", "dcgm-exporter", "process-exporter",
        "gitlab-runner", "gitea-act-runner", "woodpecker-agent", "drone-runner-docker",
        "terraform", "opentofu", "packer", "ansible", "semaphoreui",
        "vault", "consul", "nomad", "boundary",
        "registry", "harbor", "nexus", "verdaccio", "gitea-registry",
        "nginx", "caddy", "traefik", "haproxy", "envoy", "cloudflared",
        "pihole", "adguardhome", "blocky", "unbound", "coredns", "technitium",
        "wireguard", "gluetun", "tailscale", "headscale", "netbird", "zerotier", "wg-easy",
        "homeassistant", "frigate", "esphome", "zigbee2mqtt", "mosquitto", "nodered", "n8n",
        "plex", "jellyfin", "emby", "immich", "photoprism", "navidrome", "audiobookshelf",
        "sonarr", "radarr", "lidarr", "prowlarr", "bazarr", "jellyseerr", "overseerr", "tautulli",
        "qbittorrent", "transmission", "sabnzbd", "nzbhydra2", "flaresolverr", "jackett",
        "nextcloud", "syncthing", "filebrowser", "minio", "seafile",
        "vaultwarden", "authentik", "authelia", "keycloak", "lldap",
        "portainer", "dockge", "homarr", "homepage", "heimdall", "uptime-kuma", "gatus",
        "grafana", "prometheus", "loki", "netdata", "glances", "scrutiny", "dozzle",
        "gitea", "forgejo", "gitlab", "jenkins", "woodpecker", "act-runner",
        "paperless-ngx", "stirling-pdf", "mealie", "tandoor", "actual", "firefly-iii",
        "ollama", "open-webui", "searxng", "rsshub", "miniflux", "freshrss", "ntfy", "gotify",
        "wordpress", "ghost", "umami", "plausible", "matomo", "posthog",
        "postgres", "mariadb", "mysql", "mongodb", "redis", "rabbitmq", "elasticsearch",
        "mailcow", "docker-mailserver", "roundcube",
        "watchtower", "diun", "cloudflared", "caddy", "nginx", "traefik",
        "code-server", "openvscode-server", "guacamole", "webtop",
        "kavita", "komga", "calibre-web", "bookstack", "wikijs", "outline", "memos", "linkding",
        "karakeep", "wallabag", "shiori", "changedetection", "it-tools", "excalidraw", "drawio",
        "vikunja", "planka", "wekan", "mattermost", "element", "synapse", "gotosocial", "mastodon",
        "peertube", "owncast", "jitsi", "livekit",
        "metabase", "nocodb", "baserow", "appsmith", "directus", "strapi", "appwrite", "supabase", "pocketbase",
        "langfuse", "signoz", "openobserve", "sentry", "glitchtip",
        "meilisearch", "typesense", "qdrant", "weaviate", "clickhouse", "timescaledb", "neo4j",
        "minio", "garage", "duplicati", "kopia", "restic", "borgmatic",
        "speedtest-tracker", "librespeed", "smokeping", "beszel", "glance", "dashdot",
        "homebridge", "scrypted", "zwave-js-ui", "deconz",
        "tubearchivist", "metube", "pinchflat", "invidious", "piped", "redlib", "nitter",
        "whoogle", "4get", "librechat", "anythingllm", "dify", "localai",
        "stirling-pdf", "gotenberg", "cyberchef", "penpot", "affine", "siyuan", "logseq", "trilium", "joplin",
        "ghostfolio", "wallos", "invoiceninja", "kimai", "solidtime",
        "activepieces", "windmill", "huginn", "semaphore", "awx",
        "harbor", "nexus", "verdaccio", "registry",
        "gitlab-runner", "drone", "woodpecker-agent",
        "vault", "consul", "nomad", "opentofu", "terraform",
        "mailu", "stalwart", "snappymail", "postfixadmin",
        "matrix-dendrite", "conduit", "revolt", "zulip", "rocket.chat",
        "lemmy", "pixelfed", "writefreely", "ghost", "hugo",
        "umami", "goatcounter", "ackee", "plausible", "posthog", "matomo",
        "keycloak", "pocket-id", "oauth2-proxy", "lldap", "freeipa",
        "samba", "webdav", "rclone", "restic-rest-server",
        "unbound", "coredns", "technitium", "blocky", "powerdns",
        "wg-easy", "pivpn", "nebula", "netbird", "zerotier", "tailscale", "headscale",
        "pangolin", "frp", "nps", "rathole", "boringproxy",
        "alloy", "vector", "fluent-bit", "promtail", "tempo", "mimir", "thanos", "victoriametrics",
        "alertmanager", "karma", "botkube", "keep",
        "blackbox-exporter", "snmp-exporter", "cloudprober", "smokeping",
        "pgadmin", "adminer", "phpmyadmin", "mongo-express", "redis-commander", "redisinsight",
        "kafka-ui", "kafdrop", "akhq", "redpanda-console",
        "emqx", "vernemq", "nanomq", "centrifugo", "nats-surveyor",
        "jaeger", "zipkin", "opentelemetry-collector", "signoz", "openobserve", "highlight",
        "litellm", "one-api", "new-api", "helicone", "langfuse",
        "comfyui", "invokeai", "automatic1111", "fooocus", "whisper", "faster-whisper", "piper",
        "browserless", "selenium", "playwright", "puppeteer",
        "gotenberg", "unoserver", "collabora", "onlyoffice", "cryptpad",
        "privatebin", "hastebin", "microbin", "wastebin", "pastefy",
        "pairdrop", "localsend", "snapdrop", "pingvin-share", "projectsend", "pydio",
        "owncloud-infinite-scale", "nextcloud-aio", "seafile", "filestash",
        "photoview", "piwigo", "zenphoto", "digikam", "lychee", "librephotos", "ente",
        "koel", "gonic", "ampache", "funkwhale", "feishin", "finamp",
        "kavita", "komga", "lazylibrarian", "readarr", "calibre", "ebook2audiobook",
        "maintainerr", "kometa", "pulsarr", "recyclarr", "notifiarr", "requestrr",
        "tdarr", "unmanic", "fileflows", "handbrake", "makemkv",
        "youtubedl-material", "pinchflat", "tubearchivist", "metube",
        "freetube", "libretube", "piped", "invidious",
        "deluge", "rtorrent", "flood", "rutorrent", "jdownloader", "pyload", "aria2",
        "nzbget", "sabnzbd", "nzbhydra2", "jackett", "prowlarr", "flaresolverr",
        "gluetun", "openvpn-as", "wireguard", "wg-easy",
        "adguardhome", "pihole", "blocky", "technitium", "unbound",
        "caddy", "traefik", "nginx", "nginx-proxy-manager", "haproxy", "cloudflared",
        "authentik", "authelia", "keycloak", "pocket-id", "lldap",
        "portainer", "dockge", "yacht", "homarr", "homepage", "heimdall", "dashy", "flame", "glance", "beszel",
        "uptime-kuma", "gatus", "healthchecks", "changedetection", "speedtest-tracker", "librespeed",
        "netdata", "glances", "scrutiny", "cadvisor", "dozzle", "dashdot",
        "grafana", "prometheus", "loki", "promtail", "alloy", "tempo",
        "vaultwarden", "gitea", "forgejo", "gitlab", "jenkins", "woodpecker", "act-runner", "github-runner",
        "nextcloud", "syncthing", "filebrowser", "minio", "duplicati", "kopia", "restic",
        "paperless-ngx", "stirling-pdf", "it-tools", "mealie", "tandoor", "actual", "firefly-iii",
        "n8n", "node-red", "home-assistant", "frigate", "esphome", "zigbee2mqtt", "mosquitto", "homebridge",
        "ollama", "open-webui", "searxng", "rsshub", "miniflux", "freshrss", "ntfy", "gotify",
        "jellyfin", "plex", "emby", "immich", "photoprism", "navidrome", "audiobookshelf",
        "sonarr", "radarr", "lidarr", "prowlarr", "bazarr", "jellyseerr", "overseerr", "tautulli",
        "qbittorrent", "transmission", "sabnzbd", "wordpress", "ghost", "umami", "plausible", "matomo",
        "postgres", "mariadb", "mysql", "mongodb", "redis", "rabbitmq", "elasticsearch", "meilisearch",
        "mailcow", "docker-mailserver", "roundcube", "watchtower", "registry", "code-server", "guacamole",
        "bookstack", "wikijs", "outline", "memos", "linkding", "karakeep", "wallabag", "vikunja", "planka",
        "mattermost", "element", "synapse", "jitsi", "peertube", "owncast", "gotosocial", "mastodon",
        "metabase", "nocodb", "directus", "strapi", "appwrite", "supabase", "pocketbase", "langfuse", "signoz",
        "qdrant", "weaviate", "clickhouse", "timescaledb", "neo4j", "pgadmin", "adminer", "phpmyadmin",
        "keycloak", "cloudflared", "tailscale", "headscale", "netbird", "wg-easy", "pihole", "adguardhome",
        "caddy", "nginx", "traefik", "portainer", "dockge", "homarr", "uptime-kuma", "grafana", "prometheus",
    ]


def main() -> None:
    icons = load_icons()
    survey = load_survey()
    by_id: dict[str, dict] = {}

    def upsert(entry: dict) -> None:
        eid = entry["id"]
        cur = by_id.get(eid)
        if not cur:
            by_id[eid] = entry
            return
        # merge images/names
        imgs = list(dict.fromkeys((cur.get("images") or []) + (entry.get("images") or [])))
        names = list(dict.fromkeys((cur.get("names") or []) + (entry.get("names") or [])))
        cur["images"] = imgs
        cur["names"] = names
        if not cur.get("icon") and entry.get("icon"):
            cur["icon"] = entry["icon"]
        if entry.get("name") and len(entry["name"]) > len(cur.get("name") or ""):
            cur["name"] = entry["name"]

    # Explicit image aliases first (highest quality)
    for img, meta in IMAGE_ALIASES.items():
        upsert(
            {
                "id": meta["id"],
                "name": meta["name"],
                "icon": meta["icon"],
                "images": [img],
                "names": [meta["id"], slugify(meta["name"])],
            }
        )

    # Survey apps
    for name in survey:
        sid = slugify(name)
        icon = sid if sid in icons else sid.replace(" ", "-")
        if icon not in icons:
            # try common variants
            for cand in (sid, sid.replace("-", ""), name.lower().replace(" ", "-"), name.lower().replace(" ", "")):
                if cand in icons:
                    icon = cand
                    break
            else:
                icon = sid
        upsert(
            {
                "id": sid,
                "name": name,
                "icon": icon,
                "images": [sid, sid.replace("-", "")],
                "names": [sid, slugify(name)],
            }
        )

    # Dashboard icons as discoverable apps (fills toward 1000+)
    for icon in sorted(icons):
        upsert(
            {
                "id": icon,
                "name": title_from_slug(icon),
                "icon": icon,
                "images": [icon, icon.replace("-", "")],
                "names": [icon],
            }
        )

    # linuxserver / common names
    for name in linuxserver_apps():
        sid = slugify(name)
        icon = sid if sid in icons else sid
        upsert(
            {
                "id": sid,
                "name": title_from_slug(sid),
                "icon": icon if icon in icons else sid,
                "images": [
                    sid,
                    f"linuxserver/{sid}",
                    f"lscr.io/linuxserver/{sid}",
                    f"ghcr.io/linuxserver/{sid}",
                ],
                "names": [sid],
            }
        )

    items = sorted(by_id.values(), key=lambda x: x["id"])
    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(json.dumps(items, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {len(items)} apps -> {OUT}")


if __name__ == "__main__":
    main()
