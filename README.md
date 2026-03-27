# peppol-cli

CLI for the [e-invoice.be](https://e-invoice.be) Peppol Access Point API.

## Installation

```bash
go install github.com/e-invoicebe/peppol-cli/cmd/peppol@latest
```

## Authentication

```bash
# Interactive authentication (opens browser)
peppol auth

# Or set an environment variable
export PEPPOL_API_KEY=your-api-key

# Check authentication status
peppol auth status

# Remove stored credentials
peppol auth logout
```

## Commands

```bash
# Display current tenant account information
peppol me

# Display usage statistics
peppol stats
peppol stats --from 2026-01-01 --to 2026-03-01 --aggregation MONTH

# JSON output (available for all commands)
peppol me --json
peppol stats --json
```

## Shell Completions

Generate shell completion scripts for tab-completion support.

### Bash

```bash
# Load completions in current session
source <(peppol completion bash)

# Install permanently (Linux)
peppol completion bash > /etc/bash_completion.d/peppol

# Install permanently (macOS with Homebrew)
peppol completion bash > $(brew --prefix)/etc/bash_completion.d/peppol
```

### Zsh

```bash
# Load completions in current session
source <(peppol completion zsh)

# Install permanently (add to your .zshrc)
peppol completion zsh > "${fpath[1]}/_peppol"
```

### Fish

```bash
# Load completions in current session
peppol completion fish | source

# Install permanently
peppol completion fish > ~/.config/fish/completions/peppol.fish
```

### PowerShell

```powershell
# Load completions in current session
peppol completion powershell | Out-String | Invoke-Expression

# Install permanently (add to your PowerShell profile)
peppol completion powershell > peppol.ps1
```

## Development

```bash
# Build
go build -o peppol ./cmd/peppol

# Test
go test ./...

# Vet
go vet ./...
```
