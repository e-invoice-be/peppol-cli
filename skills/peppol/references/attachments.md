# Attachments

Manage file attachments on documents. Alias: `peppol doc att`.

## List attachments

```bash
peppol document attachment list <document-id> --json
```

Returns an array of `{id, file_name, file_type, file_size}`.

## Get attachment details

```bash
peppol document attachment get <document-id> <attachment-id> --json
```

## Download attachment

```bash
peppol document attachment get <document-id> <attachment-id> -o output.pdf
```

The `-o` flag downloads the file to the specified path. Without it, only metadata is shown.

## Upload attachment

```bash
peppol document attachment add <document-id> invoice.pdf --json
```

Uploads a file as an attachment to the document.

## Delete attachment

```bash
peppol document attachment delete <document-id> <attachment-id> --yes --json
```

**Always pass `--yes`** to skip the interactive confirmation prompt.
