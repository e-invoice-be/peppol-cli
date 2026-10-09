# Mailbox

Inbound emails received on the tenant's e-invoice.be mailbox. Each email can carry attachments (PDF, UBL) that the platform converts into a document.

## List inbound emails

```bash
peppol mailbox list --json
peppol mailbox list --status failed --json
```

### Filters

- `--status <pending|success|failed>`: filter by processing status; takes precedence over `--processed`
- `--processed` / `--processed=false`: filter by processed state (ignored by the API when `--status` is set)
- `--from <date-time>`: received on or after (RFC 3339, e.g. `2026-01-01T00:00:00Z`)
- `--to <date-time>`: received on or before (RFC 3339, e.g. `2026-01-31T23:59:59Z`)
- `--search <term>`: searches sender email, subject, message ID, and attachment filenames
- `--sort-by <received_at|created_at>`: sort field (default: `received_at`)
- `--sort-order <asc|desc>`: sort direction (default: `desc`)
- `--page <n>`, `--page-size <n>`: pagination (default: page 1, 20 per page, max 100)

JSON response includes pagination metadata:

```json
{
  "items": [...],
  "total": 41,
  "page": 1,
  "page_size": 20,
  "pages": 3,
  "has_next_page": true
}
```

## Get inbound email

```bash
peppol mailbox get <email-id> --json
```

Returns `{id, message_id, sender_email, sender_name, to_addresses, cc_addresses, bcc_addresses, subject, attachments, attachment_count, processed, error_message, received_at, created_at, processed_at, document_id}`.

- Keys with a null value are omitted: only `id`, `message_id`, `sender_email`, `attachments`, `attachment_count`, `processed`, and `created_at` are always present
- `attachments` is an array of `{filename, size, content_type}` (`size` and `content_type` are omitted when null)
- `document_id` is set when the email produced a document -- use it with `peppol document get`
- There is no status field: `--status` filters on the server. Read `processed` and `error_message` (the processing error, when present) instead

## Download attachment

```bash
peppol mailbox attachment <email-id> <filename> -o output.pdf
peppol mailbox attachment <email-id> <filename> > output.pdf
```

- `<filename>` is the `filename` value from the email's `attachments` array
- Without `-o`, the raw file content goes to stdout (also with `--json`)
- With `-o` and `--json`, returns `{output, content_type, size}`

Alias: `peppol mailbox att`.

## Reprocess a failed email

```bash
peppol mailbox reprocess <email-id> --json
```

Retries processing of a failed inbound email. Processing is asynchronous: the command returns the email at once -- poll `peppol mailbox get <email-id> --json` for the result.

Errors:
- Exit code 4: email not found
- `API error 409`: the email is not in a reprocessable state
- `API error 422`: the original payload is no longer available
