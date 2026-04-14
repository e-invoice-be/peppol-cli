# peppol

**Send and receive e-invoices over the Peppol network from your terminal.**

CLI for the [e-invoice.be](https://e-invoice.be) Peppol Access Point API.

[![Release](https://img.shields.io/github/v/release/e-invoice-be/peppol-cli)](https://github.com/e-invoice-be/peppol-cli/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/e-invoicebe/peppol-cli)](https://goreportcard.com/report/github.com/e-invoicebe/peppol-cli)

## Quick Start

```bash
# 1. Install (macOS/Linux)
brew tap e-invoice-be/tap && brew install e-invoice-be/tap/peppol-cli

# 2. Authenticate
peppol auth

# 3. Check your account
peppol me
```

## Installation

### macOS

```bash
brew tap e-invoice-be/tap
brew install e-invoice-be/tap/peppol-cli
```

### Linux

**Homebrew:**

```bash
brew tap e-invoice-be/tap
brew install e-invoice-be/tap/peppol-cli
```

**Binary download:**

```bash
# Download latest release (amd64)
curl -sL "https://github.com/e-invoice-be/peppol-cli/releases/latest/download/peppol-cli_$(curl -sL https://api.github.com/repos/e-invoice-be/peppol-cli/releases/latest | grep tag_name | cut -d '"' -f4 | sed 's/^v//')_linux_amd64.tar.gz" | tar xz
sudo mv peppol /usr/local/bin/
```

For ARM64, replace `amd64` with `arm64` in the URL above.

### Windows

Download the latest `.zip` for Windows from [GitHub Releases](https://github.com/e-invoice-be/peppol-cli/releases), extract it, and add `peppol.exe` to your `PATH`.

### Go install (cross-platform)

```bash
go install github.com/e-invoicebe/peppol-cli/cmd/peppol@latest
```

## Authentication

Authenticate with your e-invoice.be account:

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

| Command | Description |
|---------|-------------|
| `peppol me` | Display current account information |
| `peppol auth` | Authenticate with the e-invoice.be API |
| `peppol workspace` | Manage workspaces (`list`, `add`, `use`, `remove`) |
| `peppol inbox` | Browse received documents and invoices |
| `peppol outbox` | Browse sent documents |
| `peppol drafts` | Browse draft documents |
| `peppol document` | Get, create, send, validate, and delete documents |
| `peppol lookup` | Look up Peppol participants by ID or name |
| `peppol validate` | Validate Peppol IDs, JSON invoices, and UBL documents |
| `peppol stats` | Display usage statistics |

### Examples

```bash
# Look up a Peppol participant
peppol lookup 0208:0123456789

# Search participants by name
peppol lookup search "Company Name" --country BE

# Create a document from a JSON file
peppol document create json invoice.json

# Send a document via Peppol
peppol document send <document-id>

# List received invoices
peppol inbox invoices

# Validate a UBL document
peppol validate ubl invoice.xml

# Usage stats for a specific period
peppol stats --from 2026-01-01 --to 2026-03-01 --aggregation MONTH

# JSON output (available for all commands)
peppol me --json
```

### Global Flags

| Flag | Description |
|------|-------------|
| `--json, -j` | Output as JSON |
| `--quiet, -q` | Suppress non-essential output |
| `--verbose, -v` | Enable verbose output |
| `--no-color` | Disable colored output |
| `--workspace, -w` | Override active workspace |

<details>
<summary><h2>Shell Completions</h2></summary>

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

</details>

## Contributing

Contributions are welcome! Please open an issue first to discuss what you'd like to change.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/my-feature`)
3. Commit your changes
4. Push to the branch and open a Pull Request

<details>
<summary><h2>Development</h2></summary>

```bash
# Build
make build

# Run tests
make test

# Lint
make lint

# Test release (snapshot)
make release-dry
```

</details>

## License

[MIT](LICENSE)
