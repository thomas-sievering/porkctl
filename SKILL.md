---
name: t-porkbun
version: "1.2"
description: Use when user says "check domain", "register domain", "domain pricing", "is this domain available", "manage DNS", "add DNS record", "update DNS", "list DNS records", "delete DNS record", or asks for Porkbun domain/DNS operations.
user-invocable: true
argument-hint: "[check|register|ping|pricing|dns] [domain or keyword]"
allowed-tools: Read, Bash
---

# t-porkbun Skill

Agent workflow for using `t-porkbun.exe` for Porkbun domain and DNS operations.

## Arguments

Parse `$ARGUMENTS` into:
- `mode`: `check`, `register`, `ping`, `pricing`, or `dns`
- `target`: domain or keyword (when relevant)
- `extra`: remaining flags/tokens

If mode is missing, default to `check`. For DNS operations, also parse a submode: `list`, `get`, `create`, `edit`, `delete`.

## Examples

- User says: "check nex.us"
  - Run: `t-porkbun.exe check nex.us --json`
- User says: "check these domains" + list
  - Run: `t-porkbun.exe check-bulk <domain1> <domain2> ... --json`
- User says: "register nex.us"
  - Run: `t-porkbun.exe register nex.us --json`
- User says: "is Porkbun auth working?"
  - Run: `t-porkbun.exe ping --json`
- User says: "show cheapest TLDs"
  - Run: `t-porkbun.exe pricing --json`
- User says: "list DNS records for example.com"
  - Run: `t-porkbun.exe dns list --json example.com`
- User says: "add an A record for www pointing to 1.2.3.4"
  - Run: `t-porkbun.exe dns create --type A --name www --content 1.2.3.4 --json example.com`
- User says: "delete DNS record 12345 from example.com"
  - Run: `t-porkbun.exe dns delete --id 12345 --json example.com`

## Runtime Context (Optional)

Use these quick checks when troubleshooting local setup:

!`ls -la`
!`ls -la ../_env/secrets`

## Recommended Flow

### 1) Validate credentials (optional quick check)

```powershell
t-porkbun.exe ping --json
```

### 2) Check availability

Single domain:

```powershell
t-porkbun.exe check <domain> --json
```

Bulk:

```powershell
t-porkbun.exe check-bulk <d1> <d2> ... --json
```

### 3) Register (high-impact action)

Always show the domain and price from `check` output first, then confirm with user before executing:

```powershell
t-porkbun.exe register <domain> --json
```

### 4) Pricing view

```powershell
t-porkbun.exe pricing --json
```

### 5) DNS management

List all records:

```powershell
t-porkbun.exe dns list --json example.com
```

Filter by type:

```powershell
t-porkbun.exe dns list --type A --json example.com
```

Get a single record:

```powershell
t-porkbun.exe dns get --id 12345 --json example.com
```

Create a record:

```powershell
t-porkbun.exe dns create --type A --name www --content 1.2.3.4 --json example.com
```

Edit a record (by ID or by name+type):

```powershell
t-porkbun.exe dns edit --id 12345 --type A --content 5.6.7.8 --json example.com
t-porkbun.exe dns edit --type A --name www --content 5.6.7.8 --json example.com
```

Delete a record (confirm with user before executing):

```powershell
t-porkbun.exe dns delete --id 12345 --json example.com
t-porkbun.exe dns delete --type A --name www --json example.com
```

### 6) Return a concise result summary

After running commands, report:
- command executed
- key result (available/unavailable, price, or registration result)
- any actionable next step

## Credentials

`t-porkbun.exe` resolves credentials in this order:
1. `PORKBUN_API_KEY` + `PORKBUN_SECRET_KEY` (or `T_PORKBUN_API_KEY` + `T_PORKBUN_SECRET_KEY`)
2. `T_PORKBUN_ENV_FILE`
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

- If keys are missing, instruct user to set `../_env/secrets/porkbun.env` (or `T_PORKBUN_ENV_FILE`).
- If API returns an error, show the exact message and do not retry registration automatically.
- If a domain is unavailable, suggest alternatives and run `check-bulk`.
- If registration is requested without explicit user confirmation in conversation, stop and ask before running `register`.


