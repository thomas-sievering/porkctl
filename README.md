# t-porkbun

[![CI](https://github.com/thomas-sievering/t-porkbun/actions/workflows/ci.yml/badge.svg)](https://github.com/thomas-sievering/t-porkbun/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.23%2B-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/thomas-sievering/t-porkbun?display_name=tag)](https://github.com/thomas-sievering/t-porkbun/releases)
[![Platforms](https://img.shields.io/badge/platforms-windows%20%7C%20linux%20%7C%20macOS-6f42c1)](#install)

Go CLI for Porkbun domain and DNS operations.

## Quick Start

```powershell
# Check credentials
t-porkbun ping

# Check one domain
t-porkbun check nex.us

# Check several domains
t-porkbun check-bulk nex.us nexus.io nexus.dev

# Register a domain (after confirming check output)
t-porkbun register nex.us

# View cheapest TLD pricing
t-porkbun pricing

# List DNS records
t-porkbun dns list example.com

# Create a DNS record
t-porkbun dns create --type A --name www --content 1.2.3.4 example.com

# Delete a DNS record by ID
t-porkbun dns delete --id 12345 example.com
```

## Install

### Option A: Download Binary (recommended for users)

Use the GitHub Release asset for your OS and run `t-porkbun` directly.

### Option B: Build from source (dev)

```powershell
go build -o t-porkbun.exe .
```

End users do **not** need Go if you ship the binary.

## Commands

```powershell
t-porkbun version
t-porkbun ping [--json]
t-porkbun check <domain> [--json]
t-porkbun check-bulk <d1> <d2> ... [--json]
t-porkbun register <domain> [--json]
t-porkbun pricing [--json]
t-porkbun auth setup [--json]
t-porkbun auth login [--json]
t-porkbun auth status [--json]
t-porkbun auth logout [--json]
t-porkbun dns list [--type TYPE] [--name SUB] [--filter TEXT] [--first] [--id-only] [--json] <domain>
t-porkbun dns get --id N [--id-only] [--json] <domain>
t-porkbun dns create --type TYPE --content VAL [--name SUB] [--ttl N] [--prio N] [--notes TXT] [--id-only] [--json] <domain>
t-porkbun dns edit {--id N | --type TYPE [--name SUB]} --content VAL [--ttl N] [--prio N] [--notes TXT] [--json] <domain>
t-porkbun dns delete {--id N | --type TYPE [--name SUB]} [--json] <domain>
```

Global flags:

- `--json` for machine-readable output envelope.

## Credentials

The quickest way to set up credentials:

```powershell
t-porkbun auth setup    # shows step-by-step instructions
t-porkbun auth login    # prompts for API key + secret, saves to config
t-porkbun auth status   # verify credentials are configured
```

Credential sources (highest priority first):

1. Env vars: `PORKBUN_API_KEY` + `PORKBUN_SECRET_KEY` (or `T_PORKBUN_API_KEY` + `T_PORKBUN_SECRET_KEY`)
2. `T_PORKBUN_ENV_FILE` (explicit path)
3. Config file: `<os.UserConfigDir>/t-porkbun/config.json` (written by `auth login`)
4. `./porkbun.env`
5. `./.env`

To remove stored credentials:

```powershell
t-porkbun auth logout
```

## JSON Output

- Success: `{"ok":true,"data":...}` (when `T_PORKBUN_JSON_ENVELOPE=1`)
- Error: `{"ok":false,"error":{"code":"...","message":"..."}}`
- Set `T_PORKBUN_JSON_PRETTY=1` for indented output.
- Set `T_PORKBUN_JSON_ENVELOPE=1` to wrap output in `{"ok":true,"data":...}`.

Required keys:

```env
PORKBUN_API_KEY=pk1_...
PORKBUN_SECRET_KEY=sk1_...
```

## Troubleshooting

- Missing keys: Run `t-porkbun auth setup` for instructions, or set env vars directly.
- API error responses:
  Re-run with a known valid domain and confirm API keys via `t-porkbun auth status`.

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
