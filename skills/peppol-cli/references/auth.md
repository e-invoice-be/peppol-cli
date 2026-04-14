# Authentication & Workspaces

## Authentication

### Interactive login

```bash
peppol auth
```

Opens `https://app.e-invoice.be/api-settings` in the browser. Paste the API key when prompted. The key is validated against the API before being stored.

**Note:** This command reads from stdin, so it cannot be used non-interactively by an AI agent. Use the `PEPPOL_API_KEY` environment variable instead.

### Environment variable (preferred for automation)

```bash
export PEPPOL_API_KEY=your-api-key
peppol me --json  # Uses the env var automatically
```

The CLI checks `PEPPOL_API_KEY` first, then falls back to stored credentials.

### Check status

```bash
peppol auth status
# Output: Authenticated as <name> (workspace: <ws>, key: sk-...****)
```

```bash
peppol auth status --json
# Not supported -- text output only
```

### Logout

```bash
peppol auth logout              # Logout from active workspace
peppol auth logout -w production  # Logout from specific workspace
```

Removes stored credentials and workspace config.

## Workspaces

Workspaces allow managing multiple e-invoice.be tenants. Each workspace has its own API key.

### List workspaces

```bash
peppol workspace list --json
# Returns: [{"name": "default", "tenant": "Company Name", "active": true}]
```

Alias: `peppol ws list`

### Add a workspace

```bash
peppol workspace add production
```

**Note:** This command reads the API key from stdin interactively. For automation, use `PEPPOL_API_KEY` with the `-w` flag instead.

### Switch workspace

```bash
peppol workspace use production
```

### Remove a workspace

```bash
peppol workspace remove staging
```

### Per-command workspace override

Use `-w` on any command to target a specific workspace without switching:

```bash
peppol me -w production --json
peppol inbox list -w staging --json
```

## Credential storage

- Config file: `~/.config/peppol-cli/config.yaml`
- Credentials: stored in file-based keyring per workspace
- The `PEPPOL_API_KEY` env var always takes precedence over stored credentials
