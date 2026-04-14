# Documents

## Get document details

```bash
peppol document get <document-id> --json
peppol document get <document-id> --full --json  # Include line items
```

Without `--full`, the `items` array is omitted from the response. Always use `--full` when you need line-item data.

**Exit code 4** if the document is not found.

## Document timeline

```bash
peppol document timeline <document-id> --json
```

Returns processing events (creation, validation, sending, delivery, errors) with timestamps.

## Create documents

### From JSON (recommended)

```bash
peppol document create json invoice.json --json
peppol document create json invoice.json --construct-pdf --json
```

- `--construct-pdf`: auto-generates a PDF representation of the invoice
- Returns the created document with its ID

### From UBL/XML

```bash
peppol document create ubl invoice.xml --json
```

For pre-built Peppol BIS Billing 3.0 UBL XML files.

### From PDF (OCR)

```bash
peppol document create pdf invoice.pdf --json
peppol document create pdf invoice.pdf --vendor-tax-id BE0123456789 --customer-tax-id BE9876543210 --json
```

- `--vendor-tax-id`: hint for OCR to identify the vendor
- `--customer-tax-id`: hint for OCR to identify the customer
- Check the `success` field in the response -- `false` means the document may need manual review

## Send document

```bash
peppol document send <document-id> --json
```

Optional override flags:
- `--sender-peppol-id`: override sender Peppol ID
- `--sender-peppol-scheme`: override sender scheme
- `--receiver-peppol-id`: override receiver Peppol ID
- `--receiver-peppol-scheme`: override receiver scheme
- `--email`: send email notification

**Important:** Always validate before sending.

## Validate document (server-side)

```bash
peppol document validate <document-id> --json
```

Validates an existing document against Peppol BIS Billing 3.0 rules. The response contains:
- `is_valid`: boolean
- `issues`: array of `{type, message, rule_id, location}` where type is `error` or `warning`

## Delete document

```bash
peppol document delete <document-id> --yes --json
```

**Always pass `--yes`** to skip the interactive confirmation prompt. Only draft documents can be deleted.

## Download UBL XML

```bash
peppol document ubl <document-id> --json          # Get signed URL in JSON
peppol document ubl <document-id>                  # Print XML to stdout
peppol document ubl <document-id> -o invoice.xml   # Save to file
```

## Document types

| Type | Value |
|---|---|
| Invoice | `INVOICE` |
| Credit Note | `CREDIT_NOTE` |
| Debit Note | `DEBIT_NOTE` |
| Self-billing Invoice | `SELFBILLING_INVOICE` |
| Self-billing Credit Note | `SELFBILLING_CREDIT_NOTE` |

## Document states

| State | Description |
|---|---|
| `DRAFT` | Created, not yet sent |
| `TRANSIT` | Being delivered via Peppol |
| `SENT` | Successfully delivered |
| `FAILED` | Delivery failed |
| `RECEIVED` | Received from another participant |
