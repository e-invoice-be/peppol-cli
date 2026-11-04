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

## Document lifecycle

```bash
peppol document lifecycle <document-id> --json
```

Returns the statuses stated about an invoice or credit note after creation, oldest first: Peppol Message Level Responses (MLR/MLS), Invoice Responses (IMR), and partner status events. Each event has:

- `canonical_status`: route-agnostic status (`deposited`, `emitted`, `delivered`, `rejected`, `acknowledged`, `approved`, `partially_approved`, `disputed`, `on_hold`, `refused`, `payment_sent`, `paid`), or `null` when the native code has no canonical equivalent
- `native_code`: the untouched code from the source
- `level`: `transport` or `business`
- `source`: `peppol:mlr`, `peppol:mls`, `peppol:imr`, `fr:superpdp`, or `local`
- `direction`: `in` or `out`
- `reason_code`, `reason`: why, when given (for example on a rejection)
- `occurred_at`: timestamp

Use `lifecycle` to see whether the buyer accepted, disputed, or paid an invoice; use `timeline` for the processing steps on our side.

**Exit code 4** if the document is not found.

## Create documents

### From JSON (recommended)

```bash
peppol document create json invoice.json --json
peppol document create json invoice.json --construct-pdf --json
```

- `--construct-pdf`: auto-generates a PDF representation of the invoice
- Returns the created document with its ID
- Credit notes: set `preceding_invoice_id` (BT-25) and `preceding_invoice_date` (BT-26, `YYYY-MM-DD`) to reference the invoice that the credit note corrects
- References: `purchase_order` (BT-13), `sales_order` (BT-14), and `buyer_reference` (BT-10, e.g. a cost centre the buyer asked for)
- Contacts: `vendor_phone` (BT-42) and `customer_phone` (BT-57). `vendor_phone` is required when vendor and customer are both in Germany (DE-R-006). `customer_address_recipient` (BT-56) / `vendor_address_recipient` (BT-41) also become the contact name in the UBL; the vendor contact name is required in that German case too (DE-R-005)
- VAT in another currency: set `tax_currency` (BT-6, must differ from `currency`) together with `total_tax_in_tax_currency` (BT-111); it does not add to `invoice_total`
- Rounding: `rounding_amount` (BT-114, between -5.00 and 5.00) is added to `invoice_total` to get the amount to pay, for example to round to whole kronor
- Payment details: `swift` is the BIC of the payee's bank, or `SE:BANKGIRO` / `SE:PLUSGIRO` with the bankgiro/plusgiro number in `bank_account_number` and `iban` left empty
- Line items: `name` (BT-153) is the item name; `description` (BT-154) is the longer item description. When you omit `name`, the API uses `description` as the item name. `price_base_quantity` (BT-149, default `1`) is the number of units that `unit_price` applies to, for example `100` for a price per 100 units

In text output, `document get` shows the preceding invoice, buyer reference, purchase and sales order, contact phones, VAT in the tax currency, and rounding amount when they are set. With `--full`, the line-item table adds a `Name` column when an item has a name and a `Base Qty` column when an item has a price base quantity other than 1.

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
- Check the `success` field in the response -- `false` means the conversion failed and the document may need manual review; `error_type` and `error_message` give the reason, and `items` can be empty

## Send document

```bash
peppol document send <document-id> --json
```

Optional override flags:
- `--sender-peppol-id`: override sender Peppol ID
- `--sender-peppol-scheme`: override sender scheme
- `--receiver-peppol-id`: override receiver Peppol ID
- `--receiver-peppol-scheme`: override receiver scheme
- `--email`: send email notification (deprecated by the API -- still accepted, prints a warning, do not use in new scripts)

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
