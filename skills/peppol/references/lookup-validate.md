# Lookup & Validate

## Look up a Peppol participant

```bash
peppol lookup 0208:0123456789 --json
```

Performs DNS lookup, SMP query, and business card retrieval. Returns:
- Registration status
- SMP hostname
- Business card entities (name, country, identifiers)
- Supported document types

## Search participants by name

```bash
peppol lookup search "Company Name" --json
peppol lookup search "Company" --country BE --json
```

- `--country <code>`: filter by ISO country code (e.g. `BE`, `NL`, `DE`)

Returns a list of matching participants with their Peppol IDs, names, countries, and supported document type counts.

## Validate a Peppol ID

```bash
peppol validate peppol-id 0208:0123456789 --json
```

Checks both format validity and network registration:
- `is_valid`: overall validity
- `dns_valid`: DNS record exists
- `business_card_valid`: business card is registered
- `business_card`: entity name, country, registration date
- `supported_document_types`: list of Peppol document type URNs

## Validate a JSON document

```bash
peppol validate json invoice.json --json
```

Validates a JSON invoice file against Peppol BIS Billing 3.0 rules without creating a document. Supports stdin:

```bash
cat invoice.json | peppol validate json --file - --json
```

Response:
- `is_valid`: boolean
- `issues`: array of `{type, message, rule_id, location}` where type is `error` or `warning`

**Exit code 3** when validation fails.

## Validate a UBL/XML document

```bash
peppol validate ubl invoice.xml --json
```

Same validation rules and response format as JSON validation.

## Peppol ID format

Peppol IDs use the format `scheme:identifier`:
- `0208:0123456789` -- Belgian enterprise (KBO/BCE number)
- `0106:12345678` -- Dutch enterprise (KvK number)
- `9925:BE0123456789` -- Belgian VAT number

The scheme prefix identifies the identifier type. Common schemes:
- `0208` -- Belgian KBO/BCE
- `0106` -- Dutch KvK
- `9925` -- Belgian VAT
- `9944` -- Dutch VAT
