# Inbox & Outbox

## Inbox (received documents)

```bash
peppol inbox list --json                          # All received documents
peppol inbox invoices --json                      # Received invoices only
peppol inbox credit-notes --json                  # Received credit notes only
```

### Inbox filters

- `--sender <peppol-id>`: filter by sender Peppol ID
- `--type <type>`: filter by document type (`invoice`, `credit-note`)
- `--from <yyyy-mm-dd>`: start date
- `--to <yyyy-mm-dd>`: end date
- `--search <term>`: free text search
- `--sort-by <field>`: sort field (see below)
- `--sort-order <asc|desc>`: sort direction

### Example: recent invoices from a specific sender

```bash
peppol inbox invoices --sender 0208:0123456789 --from 2025-01-01 --sort-by invoice_date --sort-order desc --json
```

## Outbox (sent documents)

```bash
peppol outbox list --json                         # All sent documents
peppol outbox drafts --json                       # Drafts only
```

### Outbox filters

- `--receiver <peppol-id>`: filter by receiver Peppol ID
- All common filters (`--type`, `--from`, `--to`, `--search`, `--sort-by`, `--sort-order`)

## Drafts

```bash
peppol drafts list --json
peppol drafts list --state DRAFT --json
```

- `--state <state>`: filter by document state

## Pagination

All list commands support:
- `--page <n>`: page number (default: 1)
- `--page-size <n>`: results per page (default: 20)

JSON response includes pagination metadata:

```json
{
  "items": [...],
  "total": 142,
  "page": 1,
  "page_size": 20
}
```

To iterate through all results:

```bash
# Page 1
peppol inbox list --json --page 1 --page-size 50
# Page 2
peppol inbox list --json --page 2 --page-size 50
# Continue until items array is empty or page * page_size >= total
```

## Valid sort fields

`created_at`, `invoice_date`, `due_date`, `invoice_total`, `customer_name`, `vendor_name`, `invoice_id`
