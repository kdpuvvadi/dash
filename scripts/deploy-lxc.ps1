# Build the multi-stage image on the LXC (UI + Go inside Docker) and recreate dash.
# Usage: .\scripts\deploy-lxc.ps1 [-Host root@10.20.20.24]
param(
  [string]$DeployHost = "root@10.20.20.24",
  [string]$RemoteDir = "/root/dash",
  [string]$Image = "ghcr.io/kdpuvvadi/dash:latest"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$Tar = Join-Path $env:TEMP "dash-src.tgz"

Write-Host "Packing source from $Root"
if (Test-Path $Tar) { Remove-Item $Tar -Force }
tar -czf $Tar `
  --exclude=.git `
  --exclude=web/node_modules `
  --exclude=web/dist `
  --exclude=internal/embedui/dist/assets `
  --exclude=bin `
  --exclude=dist `
  --exclude=caddy-data `
  --exclude=caddy-config `
  --exclude=config/data `
  --exclude=config/ssh `
  --exclude=config/app.yaml `
  --exclude=config/secrets.yaml `
  --exclude=.env `
  -C $Root .

Write-Host "Upload + docker build on $DeployHost"
scp -o BatchMode=yes $Tar "${DeployHost}:/tmp/dash-src.tgz"
ssh -o BatchMode=yes $DeployHost @"
set -e
mkdir -p $RemoteDir
# keep runtime config / caddy state
tar -xzf /tmp/dash-src.tgz -C $RemoteDir
rm -f /tmp/dash-src.tgz
cd $RemoteDir
docker build -t $Image .
docker compose up -d --force-recreate dash
docker image prune -f >/dev/null
echo DONE
"@
Remove-Item $Tar -Force -ErrorAction SilentlyContinue
Write-Host "Done → $Image"
