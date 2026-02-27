# porkctl

[![CI](https://github.com/thomas-sievering/porkctl/actions/workflows/ci.yml/badge.svg)](https://github.com/thomas-sievering/porkctl/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.23%2B-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/thomas-sievering/porkctl?display_name=tag)](https://github.com/thomas-sievering/porkctl/releases)
[![Platforms](https://img.shields.io/badge/platforms-windows%20%7C%20linux%20%7C%20macOS-6f42c1)](#install)

Go CLI for Porkbun domain and DNS operations.

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

# List DNS records
porkctl dns list example.com

# Create a DNS record
porkctl dns create --type A --name www --content 1.2.3.4 example.com

# Delete a DNS record by ID
porkctl dns delete --id 12345 example.com
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
porkctl auth setup [--json]
porkctl auth login [--json]
porkctl auth status [--json]
porkctl auth logout [--json]
porkctl dns list [--type TYPE] [--name SUB] [--filter TEXT] [--first] [--id-only] [--json] <domain>
porkctl dns get --id N [--id-only] [--json] <domain>
porkctl dns create --type TYPE --content VAL [--name SUB] [--ttl N] [--prio N] [--notes TXT] [--id-only] [--json] <domain>
porkctl dns edit {--id N | --type TYPE [--name SUB]} --content VAL [--ttl N] [--prio N] [--notes TXT] [--json] <domain>
porkctl dns delete {--id N | --type TYPE [--name SUB]} [--json] <domain>
```

Global flags:

- `--json` for machine-readable output envelope.

## Credentials

The quickest way to set up credentials:

```powershell
porkctl auth setup    # shows step-by-step instructions
porkctl auth login    # prompts for API key + secret, saves to config
porkctl auth status   # verify credentials are configured
```

Credential sources (highest priority first):

1. Env vars: `PORKBUN_API_KEY` + `PORKBUN_SECRET_KEY` (or `PORKCTL_API_KEY` + `PORKCTL_SECRET_KEY`)
2. `PORKCTL_ENV_FILE` (explicit path)
3. Config file: `<os.UserConfigDir>/porkctl/config.json` (written by `auth login`)
4. `./porkbun.env`
5. `./.env`

To remove stored credentials:

```powershell
porkctl auth logout
```

## JSON Output

- Success: `{"ok":true,"data":...}` (when `PORKCTL_JSON_ENVELOPE=1`)
- Error: `{"ok":false,"error":{"code":"...","message":"..."}}`
- Set `PORKCTL_JSON_PRETTY=1` for indented output.
- Set `PORKCTL_JSON_ENVELOPE=1` to wrap output in `{"ok":true,"data":...}`.

Required keys:

```env
PORKBUN_API_KEY=pk1_...
PORKBUN_SECRET_KEY=sk1_...
```

## Troubleshooting

- Missing keys: Run `porkctl auth setup` for instructions, or set env vars directly.
- API error responses:
  Re-run with a known valid domain and confirm API keys via `porkctl auth status`.

## Automated Releases

This repo includes `.github/workflows/release.yml`.

On tag push (`v*`), GitHub Actions will build versioned releases. On main push, prereleases are published automatically.

- Build binaries for Windows/Linux/macOS (amd64 + arm64)
- Package assets (`.zip` for Windows, `.tar.gz` for Linux/macOS)
- Publish them to the GitHub Release

Publish a release:

```powershell
git tag v0.1.0
git push origin v0.1.0
```
