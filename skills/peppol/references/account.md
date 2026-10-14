# Account

## Tenant info

```bash
peppol me --json
```

Returns current tenant details:
- `name`: tenant/company name
- `company_name`: legal company name
- `tax_id`: tax identification number
- `peppol_identifiers`: list of registered Peppol IDs
- `plan`: current subscription plan

## Usage statistics

```bash
peppol stats --json
peppol stats --from 2025-01-01 --to 2025-03-31 --json
peppol stats --aggregation MONTH --json
```

Flags:
- `--from <yyyy-mm-dd>`: start date
- `--to <yyyy-mm-dd>`: end date
- `--aggregation <level>`: `DAY`, `WEEK`, or `MONTH` (default varies)

Returns usage metrics (documents sent, received, etc.) aggregated by the specified period.
