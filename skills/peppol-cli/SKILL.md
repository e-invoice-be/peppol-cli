---
name: peppol-cli
description: Manage Peppol e-invoicing from the command line. Create, send, validate, and receive invoices via the Peppol network through the e-invoice.be Access Point API.
allowed-tools: Bash(peppol:*)
---

# Peppol CLI

A CLI to manage e-invoices on the [Peppol](https://peppol.org) network through the [e-invoice.be](https://e-invoice.be) Access Point API.

## Prerequisites

The `peppol` command must be available on PATH:

```bash
peppol version
```

Check authentication status:

```bash
peppol auth status
```

If not authenticated, either:
- Run `peppol auth` (interactive -- opens browser to get API key)
- Set `PEPPOL_API_KEY` environment variable (non-interactive, preferred for automation)

## Best Practices for AI Agents

- **Always use `--json` (`-j`)** for machine-readable output. Every command supports it. Parse JSON instead of formatted tables.
- **Always use `--yes` (`-y`)** on destructive commands (`document delete`, `attachment delete`). Without it, the CLI prompts for confirmation on stdin which will hang in non-interactive contexts.
- **Validate before sending** -- run `peppol document validate <id> --json` and check the `is_valid` field before calling `peppol document send`.
- **Use `--full` for line items** -- `peppol document get <id> --json` omits line items by default. Add `--full` to include them.
- **Check exit codes** -- `0` = success, `1` = general error, `2` = auth error, `3` = validation failed, `4` = not found.
- **Paginate through results** -- default page size is 20. Use `--page` and `--page-size` to iterate. The JSON response includes `total`, `page`, and `page_size` fields.
- **Use `-w <workspace>`** to target a specific tenant without switching global state. Good for multi-tenant automation.
- **Prefer `document create json`** -- JSON creation is the most reliable format. PDF creation uses OCR and may need manual review (check the `success` field). UBL is for pre-built XML.
- **Use `--construct-pdf`** when creating from JSON to auto-generate a PDF representation of the invoice.
- **Peppol ID format** -- always `scheme:identifier`, e.g. `0208:0123456789` (Belgian KBO/BCE number). Use `peppol validate peppol-id` to verify format and registration.

## Available Commands

```
peppol auth                                    # Authenticate (interactive)
peppol auth status                             # Show auth status
peppol auth logout                             # Remove credentials

peppol me                                      # Show account info

peppol workspace list                          # List workspaces (alias: ws)
peppol workspace add <name>                    # Add workspace
peppol workspace use <name>                    # Switch active workspace
peppol workspace remove <name>                 # Remove workspace

peppol stats                                   # Usage statistics

peppol document get <id>                       # Show document (alias: doc)
peppol document get <id> --full                # Include line items
peppol document timeline <id>                  # Processing events
peppol document create json <file>             # Create from JSON
peppol document create ubl <file>              # Create from UBL/XML
peppol document create pdf <file>              # Create from PDF (OCR)
peppol document send <id>                      # Send via Peppol
peppol document validate <id>                  # Validate (BIS Billing 3.0)
peppol document delete <id> --yes              # Delete draft
peppol document ubl <id>                       # Download UBL XML

peppol document attachment list <doc-id>       # List attachments (alias: att)
peppol document attachment get <id> <att-id>   # Show attachment details
peppol document attachment get <id> <att> -o f # Download attachment
peppol document attachment add <id> <file>     # Upload attachment
peppol document attachment delete <id> <att>   # Delete attachment

peppol inbox list                              # All received documents
peppol inbox invoices                          # Received invoices
peppol inbox credit-notes                      # Received credit notes

peppol outbox list                             # All sent documents
peppol outbox drafts                           # Sent drafts

peppol drafts list                             # All drafts

peppol lookup <peppol-id>                      # Look up participant
peppol lookup search <query>                   # Search participants by name

peppol validate peppol-id <id>                 # Validate Peppol ID
peppol validate json <file>                    # Validate JSON document
peppol validate ubl <file>                     # Validate UBL/XML document

peppol version                                 # Show version
peppol completion bash|zsh|fish|powershell     # Shell completions
```

## Common Workflows

**Send an invoice from JSON:**

```bash
# 1. Create the document (returns document ID)
peppol document create json invoice.json --construct-pdf --json

# 2. Validate against Peppol BIS Billing 3.0
peppol document validate <document-id> --json

# 3. Send via the Peppol network
peppol document send <document-id> --json
```

**Process incoming invoices:**

```bash
# 1. List recent inbox items
peppol inbox invoices --json --page-size 50

# 2. Get full document details with line items
peppol document get <document-id> --full --json

# 3. Download the UBL XML
peppol document ubl <document-id> -o invoice.xml
```

**Check a trading partner:**

```bash
# Search by company name
peppol lookup search "Company Name" --country BE --json

# Or validate a known Peppol ID
peppol validate peppol-id 0208:0123456789 --json
```

**Create invoice from PDF (OCR):**

```bash
# Create with tax ID hints for better OCR accuracy
peppol document create pdf invoice.pdf --vendor-tax-id BE0123456789 --customer-tax-id BE9876543210 --json

# Check if OCR succeeded (inspect "success" field)
# Then validate and send as usual
peppol document validate <document-id> --json
peppol document send <document-id> --json
```

## Reference Documentation

- [Authentication & Workspaces](references/auth.md) -- auth flows, workspace management, credentials
- [Documents](references/documents.md) -- create, get, send, validate, delete, download UBL
- [Attachments](references/attachments.md) -- list, get, add, delete attachments
- [Inbox & Outbox](references/inbox-outbox.md) -- browse received/sent documents, filtering, pagination
- [Lookup & Validate](references/lookup-validate.md) -- Peppol ID lookup, search, format validation
- [Account](references/account.md) -- tenant info, usage statistics

## Discovering Options

Use `--help` on any command to see all available flags:

```bash
peppol --help
peppol document --help
peppol document create --help
peppol inbox list --help
```

Notable command aliases for brevity:
- `doc` for `document`
- `att` for `attachment`
- `ws` for `workspace`
