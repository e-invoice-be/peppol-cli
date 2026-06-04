# Backup

Archive every document (with attachments, UBL XML, and timeline) for the active tenant to a local directory.

```bash
peppol backup <dir> --json
```

The action taken depends entirely on the state of `<dir>` — there is no `--resume` or `--since` flag.

## Directory state machine

| State of `<dir>` | What the run does |
|---|---|
| empty / missing | **Fresh** — paginate inbox + outbox + drafts and archive every document |
| `manifest.json` present, last run has no `completed_at` | **Resume** — continue from where a killed prior run left off; documents already on disk are not re-fetched |
| `manifest.json` present, last run completed | **Top-up** — fetch documents created after the prior run's high-water mark, and refresh DRAFT/TRANSIT documents (see refresh policy below) |

Re-running against the same directory is always safe.

## Refresh policy (top-up only)

| State | Behaviour on top-up |
|---|---|
| `DRAFT` | Re-fetched (state may still change) |
| `TRANSIT` | Re-fetched (still in flight) |
| `SENT` | Frozen — never re-fetched |
| `FAILED` | Frozen — never re-fetched |
| `RECEIVED` | Frozen — never re-fetched |

The backup is **append-only**: server-side deletions are NOT detected and do not remove local files.

## Layouts

`--layout=flat` (default):

```
<dir>/
├── manifest.json
├── documents.jsonl          # one compacted envelope JSON per line
├── ubl/<doc-id>.xml
└── attachments/<doc-id>/<filename>
```

`--layout=tree` — per-document directories, better for browsing:

```
<dir>/
├── manifest.json
└── documents/<doc-id>/
    ├── document.json
    ├── timeline.json
    ├── ubl.xml
    └── attachments/<filename>
```

The layout is recorded in `manifest.json` on the first run. Re-running with a different `--layout` is rejected.

## Flags

| Flag | Default | Notes |
|---|---|---|
| `--layout` | `flat` | `flat` or `tree`; must match the existing manifest on resume/top-up |
| `--concurrency` | `8` | Parallel per-document workers (detail + UBL + timeline + attachments) |

## Guardrails

- **Tenant fingerprint** — the manifest records a SHA256 prefix of the API key. Running against a directory that belongs to a different tenant aborts before any API call.
- **Layout mismatch** — same: aborts before any API call.
- **Atomic writes** — every file is written via temp file + `fsync` + rename. A `kill -9` at any point leaves no half-written file visible under its final name.
- **Path traversal** — attachment filenames and document IDs containing `..`, path separators, or absolute paths are rejected with a clear error naming the offending value.
- **Retries** — transient errors (429, 5xx) retry up to 4 times with exponential backoff + jitter; non-transient errors propagate immediately.

## JSON output

```bash
peppol backup ./peppol-archive --json
```

```json
{
  "documents_fetched": 142,
  "documents_refreshed": 3,
  "documents_skipped": 0,
  "started_at": "2026-06-04T12:00:00Z",
  "completed_at": "2026-06-04T12:01:23Z"
}
```

## Common patterns

**Nightly cron** — re-run against the same directory; the state machine handles the rest:

```bash
peppol backup /var/peppol-archive --json >> /var/log/peppol-backup.jsonl
```

**Large tenants** — bump concurrency for faster initial fetch (the engine streams bodies and JSONL rebuild, so memory stays flat):

```bash
peppol backup ./peppol-archive --concurrency=32 --json
```

**Per-tenant directories in multi-tenant automation** — combine with `-w` to target a workspace; each tenant gets its own directory:

```bash
peppol -w acme backup ./archives/acme --json
peppol -w widgets backup ./archives/widgets --json
```
