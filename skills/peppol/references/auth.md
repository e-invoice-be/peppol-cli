# Authentication & Workspaces

## Authentication

### Interactive login

```bash
peppol auth
```

Prints `https://app.e-invoice.be/api-settings` (and opens it in the browser when run in a terminal). Paste the API key when prompted. The key is validated against the API before being stored. Prompts and progress go to stderr.

The key can also be piped in, which skips the browser: `printf '%s\n' "$KEY" | peppol auth`. For one-off automation the `PEPPOL_API_KEY` environment variable is usually simpler.

### Environment variable (preferred for automation)

```bash
export PEPPOL_API_KEY=your-api-key
peppol me --json  # Uses the env var automatically
```

The CLI checks `PEPPOL_API_KEY` first, then falls back to stored credentials.

`peppol auth status` uses the same resolution as commands like `peppol me`, so the masked key and whether it came from the environment or the keyring cannot drift from what the CLI actually uses.

### Check status

```bash
peppol auth status
# Output: Authenticated as <name> (workspace: <ws>, key: ****...abcd)

# With PEPPOL_API_KEY set, the env key is reported instead:
# Authenticated via PEPPOL_API_KEY (key: ****...abcd)
# Note: PEPPOL_API_KEY overrides the stored credentials of workspace "<ws>"
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

The API key is read from stdin (prompt on stderr), so it can be piped: `printf '%s\n' "$KEY" | peppol workspace add production`.

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
