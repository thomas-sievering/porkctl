---
name: porkctl
version: "1.1"
description: Use when user says "check domain", "register domain", "domain pricing", "is this domain available", or asks for Porkbun domain operations.
user-invocable: true
argument-hint: "[check|register|ping|pricing] [domain or keyword]"
allowed-tools: Read, Bash
---

# porkctl Skill

Agent workflow for using `porkctl` for Porkbun domain operations.

## Arguments

Parse `$ARGUMENTS` into:
- `mode`: `check`, `register`, `ping`, or `pricing`
- `target`: domain or keyword (when relevant)
- `extra`: remaining flags/tokens

If mode is missing, default to `check`.

## Examples

- User says: "check nex.us"
  - Run: `porkctl check nex.us --json`
- User says: "check these domains" + list
  - Run: `porkctl check-bulk <domain1> <domain2> ... --json`
- User says: "register nex.us"
  - Run: `porkctl register nex.us --json`
- User says: "is Porkbun auth working?"
  - Run: `porkctl ping --json`
- User says: "show cheapest TLDs"
  - Run: `porkctl pricing --json`

## Runtime Context (Optional)

Use these quick checks when troubleshooting local setup:

!`ls -la`
!`ls -la ../_env/secrets`

## Recommended Flow

### 1) Validate credentials (optional quick check)

```powershell
porkctl ping --json
```

### 2) Check availability

Single domain:

```powershell
porkctl check <domain> --json
```

Bulk:

```powershell
porkctl check-bulk <d1> <d2> ... --json
```

### 3) Register (high-impact action)

Always show the domain and price from `check` output first, then confirm with user before executing:

```powershell
porkctl register <domain> --json
```

### 4) Pricing view

```powershell
porkctl pricing --json
```

### 5) Return a concise result summary

After running commands, report:
- command executed
- key result (available/unavailable, price, or registration result)
- any actionable next step

## Credentials

`porkctl` resolves credentials in this order:
1. `PORKBUN_API_KEY` + `PORKBUN_SECRET_KEY` (or `PORKCTL_API_KEY` + `PORKCTL_SECRET_KEY`)
2. `PORKCTL_ENV_FILE`
3. `../_env/secrets/porkbun.env`
4. `./porkbun.env`
5. `./.env`
6. `../_skills/porkbun/.env` (legacy fallback)

Required keys:

```env
PORKBUN_API_KEY=pk1_...
PORKBUN_SECRET_KEY=sk1_...
```

## Error Handling

- If keys are missing, instruct user to set `../_env/secrets/porkbun.env` (or `PORKCTL_ENV_FILE`).
- If API returns an error, show the exact message and do not retry registration automatically.
- If a domain is unavailable, suggest alternatives and run `check-bulk`.
- If registration is requested without explicit user confirmation in conversation, stop and ask before running `register`.
