# porkctl

[![CI](https://github.com/thomas-sievering/porkctl/actions/workflows/ci.yml/badge.svg)](https://github.com/thomas-sievering/porkctl/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.23%2B-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/thomas-sievering/porkctl?display_name=tag)](https://github.com/thomas-sievering/porkctl/releases)
[![Platforms](https://img.shields.io/badge/platforms-windows%20%7C%20linux%20%7C%20macOS-6f42c1)](#install)

Go CLI for Porkbun domain operations.

## Quick Start

```powershell
# Check credentials
porkctl ping

# Check one domain
porkctl check nex.us

# Check several domains
porkctl check-bulk nex.us nexus.io nexus.dev

# Register a domain (after confirming check output)
porkctl register nex.us

# View cheapest TLD pricing
porkctl pricing
```

## Install

### Option A: Download Binary (recommended for users)

Use the GitHub Release asset for your OS and run `porkctl` directly.

### Option B: Build from source (dev)

```powershell
go build -o porkctl.exe .
```

End users do **not** need Go if you ship the binary.

## Commands

```powershell
porkctl version
porkctl ping [--json]
porkctl check <domain> [--json]
porkctl check-bulk <d1> <d2> ... [--json]
porkctl register <domain> [--json]
porkctl pricing [--json]
```

Global flags:

- `--json` for machine-readable output envelope.

## Credentials

Credential sources (highest priority first):

1. Env vars: `PORKBUN_API_KEY` + `PORKBUN_SECRET_KEY` (or `PORKCTL_API_KEY` + `PORKCTL_SECRET_KEY`)
2. `PORKCTL_ENV_FILE` (explicit path)
3. `C:/dev/_env/secrets/porkbun.env`
4. `./porkbun.env`
5. `./.env`
6. `C:/dev/_skills/porkbun/.env` (legacy fallback)

## JSON Output

- Success: `{"ok":true,"data":...}`
- Error: `{"ok":false,"error":{"code":"...","message":"..."}}`
- Set `PORKCTL_JSON_PRETTY=1` for indented output.

Required keys:

```env
PORKBUN_API_KEY=pk1_...
PORKBUN_SECRET_KEY=sk1_...
```

## Troubleshooting

- Missing keys / env file:
  Set env vars directly (`PORKBUN_API_KEY`, `PORKBUN_SECRET_KEY`) or set `PORKCTL_ENV_FILE`.
- API error responses:
  Re-run with a known valid domain and confirm API keys.

## Automated Releases

This repo includes `.github/workflows/release.yml`.

On tag push (`v*`), GitHub Actions will:

- Build binaries for Windows/Linux/macOS (amd64 + arm64)
- Package assets (`.zip` for Windows, `.tar.gz` for Linux/macOS)
- Publish them to the GitHub Release for that tag

Publish a release:

```powershell
git tag v0.1.0
git push origin v0.1.0
```
