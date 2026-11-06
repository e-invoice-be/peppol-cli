package cli

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- document attachments ---

func attachmentJSON(api *fakeAPI, id, name string) map[string]any {
	return map[string]any{
		"id": id, "file_name": name, "file_size": 2048, "file_type": "application/pdf",
		"file_url": api.fileURL(name),
	}
}

func TestAttachmentList_TextJSONEmptyAndErrors(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/documents/doc-1/attachments", http.StatusOK, []any{attachmentJSON(api, "att-1", "scan.pdf")})
	api.json("GET /api/documents/none/attachments", http.StatusOK, []any{})
	api.json("GET /api/documents/gone/attachments", http.StatusNotFound, map[string]string{"detail": "x"})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "document", "attachment", "list", "doc-1")
	wantOK(t, res)
	wantContains(t, res.Out, "att-1", "scan.pdf", "application/pdf", "2.0 KB")

	res = runCLI(t, "", "document", "attachment", "list", "doc-1", "--json")
	wantOK(t, res)
	if !strings.HasPrefix(strings.TrimSpace(res.Out), "[") || !strings.Contains(res.Out, `"att-1"`) {
		t.Errorf("expected a JSON array, got %s", res.Out)
	}

	res = runCLI(t, "", "document", "attachment", "list", "none")
	wantOK(t, res)
	wantContains(t, res.Out, "No attachments.")

	wantCode(t, runCLI(t, "", "document", "attachment", "list", "gone"), 4)
	t.Setenv("PEPPOL_API_KEY", "bad")
	wantCode(t, runCLI(t, "", "document", "attachment", "list", "doc-1"), 2)
}

func TestAttachmentGet_DetailsAndDownload(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/documents/doc-1/attachments/att-1", http.StatusOK, attachmentJSON(api, "att-1", "scan.pdf"))
	api.handle("GET /files/scan.pdf", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "%PDF-data") })
	api.json("GET /api/documents/doc-1/attachments/nourl", http.StatusOK, map[string]any{"id": "nourl", "file_name": "x", "file_size": 1})
	api.json("GET /api/documents/doc-1/attachments/expired", http.StatusOK, attachmentJSON(api, "expired", "expired.pdf"))
	api.handle("GET /files/expired.pdf", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) })
	api.json("GET /api/documents/doc-1/attachments/gone", http.StatusNotFound, map[string]string{"detail": "x"})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "document", "attachment", "get", "doc-1", "att-1")
	wantOK(t, res)
	wantContains(t, res.Out, "att-1", "scan.pdf", "2.0 KB")

	res = runCLI(t, "", "document", "attachment", "get", "doc-1", "att-1", "--json")
	wantOK(t, res)
	if decodeJSON(t, res.Out)["file_name"] != "scan.pdf" {
		t.Errorf("unexpected JSON: %s", res.Out)
	}

	out := filepath.Join(t.TempDir(), "scan.pdf")
	res = runCLI(t, "", "document", "attachment", "get", "doc-1", "att-1", "-o", out)
	wantOK(t, res)
	wantContains(t, res.Out, "Attachment written to "+out+" (9 B)")
	res = runCLI(t, "", "document", "attachment", "get", "doc-1", "att-1", "-o", out, "--json")
	wantOK(t, res)
	if m := decodeJSON(t, res.Out); m["output"] != out || m["size"] != float64(9) {
		t.Errorf("unexpected JSON: %s", res.Out)
	}
	res = runCLI(t, "", "document", "attachment", "get", "doc-1", "att-1", "-o", out, "-q")
	wantOK(t, res)
	if res.Out != "" {
		t.Errorf("--quiet download printed %q", res.Out)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "%PDF-data" {
		t.Errorf("downloaded %q", data)
	}

	res = runCLI(t, "", "document", "attachment", "get", "doc-1", "nourl", "-o", out)
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "no download URL")

	// A failed signed-URL download must fail and must not write the error body.
	expiredOut := filepath.Join(t.TempDir(), "expired.pdf")
	res = runCLI(t, "", "document", "attachment", "get", "doc-1", "expired", "-o", expiredOut)
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "HTTP 403")
	if _, err := os.Stat(expiredOut); !os.IsNotExist(err) {
		t.Errorf("output file should not exist after a failed download, stat err = %v", err)
	}

	res = runCLI(t, "", "document", "attachment", "get", "doc-1", "att-1", "-o", filepath.Join(t.TempDir(), "nodir", "f"))
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "creating output file")

	wantCode(t, runCLI(t, "", "document", "attachment", "get", "doc-1", "gone"), 4)
}

func TestAttachmentAdd_UploadsAndReports(t *testing.T) {
	api := newFakeAPI(t)
	var gotName, gotContent string
	api.handle("POST /api/documents/doc-1/attachments", func(w http.ResponseWriter, r *http.Request) {
		gotName, _, gotContent = readMultipartFile(t, r)
		writeJSON(w, http.StatusOK, map[string]any{"id": "att-9", "file_name": gotName, "file_size": 1536, "file_type": "application/pdf"})
	})
	api.json("POST /api/documents/gone/attachments", http.StatusNotFound, map[string]string{"detail": "x"})
	useEnvKey(t, goodKey)
	path := writeTempFile(t, "extra.pdf", "pdfbytes")

	res := runCLI(t, "", "document", "attachment", "add", "doc-1", path)
	wantOK(t, res)
	wantContains(t, res.Out, "Uploaded extra.pdf (1.5 KB)")
	if gotName != "extra.pdf" || gotContent != "pdfbytes" {
		t.Errorf("server got %q / %q", gotName, gotContent)
	}

	res = runCLI(t, "", "document", "attachment", "add", "doc-1", path, "--json")
	wantOK(t, res)
	if decodeJSON(t, res.Out)["id"] != "att-9" {
		t.Errorf("unexpected JSON: %s", res.Out)
	}

	wantCode(t, runCLI(t, "", "document", "attachment", "add", "gone", path), 4)

	// A missing file is reported before credentials are needed.
	isolateConfig(t)
	res = runCLI(t, "", "document", "attachment", "add", "doc-1", filepath.Join(t.TempDir(), "missing"))
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "file not found")
}

func TestAttachmentDelete_PromptYesAndJSON(t *testing.T) {
	api := newFakeAPI(t)
	api.json("DELETE /api/documents/doc-1/attachments/att-1", http.StatusOK, map[string]any{"is_deleted": true})
	api.json("DELETE /api/documents/doc-1/attachments/gone", http.StatusNotFound, map[string]string{"detail": "x"})
	useEnvKey(t, goodKey)

	res := runCLI(t, "no\n", "document", "attachment", "delete", "doc-1", "att-1")
	wantOK(t, res)
	wantContains(t, res.Out, "Delete attachment att-1 from document doc-1? [y/N]", "Aborted.")
	if n := len(api.requests()); n != 0 {
		t.Errorf("aborted delete made %d API calls", n)
	}

	res = runCLI(t, "y\n", "document", "attachment", "delete", "doc-1", "att-1")
	wantOK(t, res)

	wantContains(t, res.Out, "Attachment deleted.")

	res = runCLI(t, "", "document", "attachment", "delete", "doc-1", "att-1", "--yes", "--json")
	wantOK(t, res)
	// Same shape as "document delete --json": the API's is_deleted.
	if m := decodeJSON(t, res.Out); m["is_deleted"] != true || len(m) != 1 {
		t.Errorf("unexpected JSON: %s", res.Out)
	}

	wantCode(t, runCLI(t, "", "document", "attachment", "delete", "doc-1", "gone", "--yes"), 4)
}

// Same behavior as "document delete": is_deleted: false is reported and
// exits 1, and --json still prints the API's answer on stdout.
func TestAttachmentDelete_NotDeleted(t *testing.T) {
	api := newFakeAPI(t)
	api.json("DELETE /api/documents/doc-1/attachments/att-1", http.StatusOK, map[string]any{"is_deleted": false})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "document", "attachment", "delete", "doc-1", "att-1", "--yes")
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "attachment was not deleted")
	if strings.Contains(res.Out, "Attachment deleted.") {
		t.Errorf("reported success for is_deleted=false:\n%s", res.Out)
	}

	stdout, _, err := runCLISplit(t, "", "document", "attachment", "delete", "doc-1", "att-1", "--yes", "--json")
	wantCode(t, cliResult{Out: stdout, Err: err}, 1)
	if m := decodeJSON(t, stdout); m["is_deleted"] != false || len(m) != 1 {
		t.Errorf("unexpected JSON: %s", stdout)
	}
}

// --- document lists ---

func TestDocumentLists_RouteToEndpointAndShowCounterparty(t *testing.T) {
	cases := []struct {
		args     []string
		path     string
		party    string // name shown in the COUNTERPARTY column
		notParty string
	}{
		{[]string{"inbox", "list"}, "/api/inbox/", "Seller NV", "Buyer BV"},
		{[]string{"inbox", "invoices"}, "/api/inbox/invoices", "Seller NV", "Buyer BV"},
		{[]string{"inbox", "credit-notes"}, "/api/inbox/credit-notes", "Seller NV", "Buyer BV"},
		{[]string{"outbox", "list"}, "/api/outbox/", "Buyer BV", "Seller NV"},
		{[]string{"outbox", "drafts"}, "/api/outbox/drafts", "Buyer BV", "Seller NV"},
		{[]string{"drafts", "list"}, "/api/drafts/", "Buyer BV", "Seller NV"},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			api := newFakeAPI(t)
			api.json("GET "+tc.path, http.StatusOK, docPage(sampleDoc("d1")))
			useEnvKey(t, goodKey)

			res := runCLI(t, "", tc.args...)
			wantOK(t, res)
			wantContains(t, res.Out, "d1", "INV-d1", tc.party, "121.00")
			if strings.Contains(res.Out, tc.notParty) {
				t.Errorf("counterparty column shows %q, want %q", tc.notParty, tc.party)
			}

			res = runCLI(t, "", append(tc.args, "--json")...)
			wantOK(t, res)
			if decodeJSON(t, res.Out)["total"] != float64(1) {
				t.Errorf("unexpected JSON: %s", res.Out)
			}

			t.Setenv("PEPPOL_API_KEY", "bad")
			wantCode(t, runCLI(t, "", tc.args...), 2)
		})
	}
}

func TestDocumentLists_FiltersReachQuery(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/inbox/", http.StatusOK, docPage())
	api.json("GET /api/outbox/", http.StatusOK, docPage())
	api.json("GET /api/drafts/", http.StatusOK, docPage())
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "inbox", "list", "--type", "INVOICE", "--sender", "0208:1", "--from", "2026-01-01",
		"--to", "2026-01-31", "--search", "acme", "--sort-by", "invoice_total", "--sort-order", "desc",
		"--page", "3", "--page-size", "50")
	wantOK(t, res)
	wantContains(t, res.Out, "No documents found.")
	want := map[string]string{
		"type": "INVOICE", "sender": "0208:1", "date_from": "2026-01-01", "date_to": "2026-01-31",
		"search": "acme", "sort_by": "invoice_total", "sort_order": "desc", "page": "3", "page_size": "50",
	}
	q := api.last("/api/inbox/").Query
	for k, v := range want {
		if q.Get(k) != v {
			t.Errorf("inbox %s = %q, want %q", k, q.Get(k), v)
		}
	}

	wantOK(t, runCLI(t, "", "outbox", "list", "--receiver", "0208:2"))
	if got := api.last("/api/outbox/").Query.Get("receiver"); got != "0208:2" {
		t.Errorf("outbox receiver = %q", got)
	}
	wantOK(t, runCLI(t, "", "drafts", "list", "--state", "DRAFT"))
	if got := api.last("/api/drafts/").Query.Get("state"); got != "DRAFT" {
		t.Errorf("drafts state = %q", got)
	}

	// Flags are package-level variables; a fresh root must reset them.
	wantOK(t, runCLI(t, "", "inbox", "list"))
	if q := api.last("/api/inbox/").Query; q.Has("sender") || q.Get("page") != "1" {
		t.Errorf("flags leaked between invocations: %v", q)
	}
}

func TestDocumentLists_InvalidSortFailsWithoutAPICall(t *testing.T) {
	api := newFakeAPI(t)
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "inbox", "list", "--sort-by", "price")
	if res.Err == nil || !strings.Contains(res.Err.Error(), `invalid --sort-by "price"`) {
		t.Errorf("err = %v", res.Err)
	}
	res = runCLI(t, "", "outbox", "drafts", "--sort-order", "up")
	if res.Err == nil || !strings.Contains(res.Err.Error(), `invalid --sort-order "up"`) {
		t.Errorf("err = %v", res.Err)
	}
	if n := len(api.requests()); n != 0 {
		t.Errorf("invalid flags made %d API calls", n)
	}
}

func TestDocumentLists_ServerErrorIsCode1(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/outbox/drafts", http.StatusServiceUnavailable, map[string]string{"detail": "maintenance"})
	useEnvKey(t, goodKey)
	res := runCLI(t, "", "outbox", "drafts")
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "maintenance")
}

// --- mailbox ---

func sampleMail(id string) map[string]any {
	return map[string]any{
		"id": id, "message_id": "<m-" + id + "@x>", "sender_email": "ap@vendor.be", "sender_name": "Vendor AP",
		"to_addresses": "invoices@us.be", "subject": "Invoice 42", "processed": true, "attachment_count": 1,
		"created_at": "2026-02-01T09:00:00Z", "received_at": "2026-02-01T08:59:00Z",
		"processed_at": "2026-02-01T09:01:00Z", "document_id": "doc-77", "error_message": "partial",
		"attachments": []map[string]any{{"filename": "inv.pdf", "content_type": "application/pdf", "size": 4096}},
	}
}

func TestMailboxList_QueryTextAndJSON(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/mailbox/", http.StatusOK, map[string]any{
		"items": []any{sampleMail("m1")}, "total": 1, "page": 1, "page_size": 20, "pages": 1,
	})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "mailbox", "list", "--status", "failed", "--processed=false", "--from", "2026-01-01T00:00:00Z",
		"--to", "2026-02-01T00:00:00Z", "--search", "inv", "--sort-by", "received_at", "--sort-order", "asc",
		"--page", "2", "--page-size", "10")
	wantOK(t, res)
	wantContains(t, res.Out, "m1", "ap@vendor.be", "Invoice 42", "Yes", "2026-02-01 08:59")
	q := api.last("/api/mailbox/").Query
	for k, v := range map[string]string{
		"status": "failed", "processed": "false", "received_from": "2026-01-01T00:00:00Z",
		"received_to": "2026-02-01T00:00:00Z", "search": "inv", "sort_by": "received_at",
		"sort_order": "asc", "page": "2", "page_size": "10",
	} {
		if q.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, q.Get(k), v)
		}
	}

	wantOK(t, runCLI(t, "", "mailbox", "list"))
	if q := api.last("/api/mailbox/").Query; q.Has("processed") || q.Has("status") {
		t.Errorf("unset filters were sent: %v", q)
	}

	res = runCLI(t, "", "mailbox", "list", "--json")
	wantOK(t, res)
	if decodeJSON(t, res.Out)["total"] != float64(1) {
		t.Errorf("unexpected JSON: %s", res.Out)
	}
}

func TestMailboxList_EmptyAndInvalidFlags(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/mailbox/", http.StatusOK, map[string]any{"items": []any{}, "total": 0, "page": 1, "page_size": 20, "pages": 0})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "mailbox", "list")
	wantOK(t, res)
	wantContains(t, res.Out, "No inbound emails found.")

	before := len(api.requests())
	for _, args := range [][]string{
		{"--status", "done"}, {"--sort-by", "subject"}, {"--sort-order", "sideways"},
	} {
		res := runCLI(t, "", append([]string{"mailbox", "list"}, args...)...)
		if res.Err == nil || !strings.Contains(res.Err.Error(), "invalid "+args[0]) {
			t.Errorf("%v: err = %v", args, res.Err)
		}
	}
	if n := len(api.requests()); n != before {
		t.Errorf("invalid flags made %d API calls", n-before)
	}

	t.Setenv("PEPPOL_API_KEY", "bad")
	wantCode(t, runCLI(t, "", "mailbox", "list"), 2)
}

func TestMailboxGet_RendersAllFields(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/mailbox/m1", http.StatusOK, sampleMail("m1"))
	api.json("GET /api/mailbox/gone", http.StatusNotFound, map[string]string{"detail": "x"})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "mailbox", "get", "m1")
	wantOK(t, res)
	wantContains(t, res.Out, "<m-m1@x>", "Vendor AP <ap@vendor.be>", "invoices@us.be", "Invoice 42",
		"2026-02-01 08:59:00", "2026-02-01 09:01:00", "doc-77", "partial", "inv.pdf", "4.0 KB")

	res = runCLI(t, "", "mailbox", "get", "m1", "--json")
	wantOK(t, res)
	if decodeJSON(t, res.Out)["document_id"] != "doc-77" {
		t.Errorf("unexpected JSON: %s", res.Out)
	}

	res = runCLI(t, "", "mailbox", "get", "gone")
	wantCode(t, res, 4)
	wantContains(t, res.Err.Error(), "peppol mailbox list")
}

func TestMailboxGet_EscapesID(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/mailbox/a/b", http.StatusOK, sampleMail("a/b"))
	useEnvKey(t, goodKey)
	wantOK(t, runCLI(t, "", "mailbox", "get", "a/b"))
	// The server decodes %2F back to "/" in r.URL.Path; RawPath shows escaping
	// happened because the request matched the single route above.
	if n := len(api.requests()); n != 1 {
		t.Errorf("requests = %d", n)
	}
}

func TestMailboxAttachment_StdoutFileAndJSON(t *testing.T) {
	api := newFakeAPI(t)
	api.handle("GET /api/mailbox/m1/attachments/inv.pdf", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = io.WriteString(w, "PDFBYTES")
	})
	api.json("GET /api/mailbox/m1/attachments/none.pdf", http.StatusNotFound, map[string]string{"detail": "x"})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "mailbox", "attachment", "m1", "inv.pdf")
	wantOK(t, res)
	if res.Out != "PDFBYTES" {
		t.Errorf("stdout = %q", res.Out)
	}

	// Without -o the raw bytes are written even in JSON mode.
	res = runCLI(t, "", "mailbox", "att", "m1", "inv.pdf", "--json")
	wantOK(t, res)
	if res.Out != "PDFBYTES" {
		t.Errorf("stdout = %q", res.Out)
	}

	out := filepath.Join(t.TempDir(), "inv.pdf")
	res = runCLI(t, "", "mailbox", "attachment", "m1", "inv.pdf", "-o", out)
	wantOK(t, res)
	wantContains(t, res.Out, "Attachment written to "+out)
	if data, err := os.ReadFile(out); err != nil || string(data) != "PDFBYTES" {
		t.Errorf("file = %q, %v", data, err)
	}

	res = runCLI(t, "", "mailbox", "attachment", "m1", "inv.pdf", "-o", out, "--json")
	wantOK(t, res)
	m := decodeJSON(t, res.Out)
	if m["output"] != out || m["content_type"] != "application/pdf" || m["size"] != float64(8) {
		t.Errorf("unexpected JSON: %v", m)
	}

	res = runCLI(t, "", "mailbox", "attachment", "m1", "inv.pdf", "-o", filepath.Join(t.TempDir(), "x", "y"))
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "writing output file")

	res = runCLI(t, "", "mailbox", "attachment", "m1", "none.pdf")
	wantCode(t, res, 4)
	wantContains(t, res.Err.Error(), "inbound email or attachment not found")
}

func TestMailboxReprocess(t *testing.T) {
	api := newFakeAPI(t)
	api.json("POST /api/mailbox/m1/reprocess", http.StatusAccepted, sampleMail("m1"))
	api.json("POST /api/mailbox/err/reprocess", http.StatusConflict, map[string]string{"detail": "already processed"})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "mailbox", "reprocess", "m1")
	wantOK(t, res)
	wantContains(t, res.Out, "Reprocessing started.")

	res = runCLI(t, "", "mailbox", "reprocess", "m1", "--json")
	wantOK(t, res)
	if decodeJSON(t, res.Out)["id"] != "m1" {
		t.Errorf("unexpected JSON: %s", res.Out)
	}

	res = runCLI(t, "", "mailbox", "reprocess", "err")
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "already processed")
}

// --- lookup ---

func TestLookup_NoArgsShowsHelpWithoutAuth(t *testing.T) {
	isolateConfig(t)
	res := runCLI(t, "", "lookup")
	wantOK(t, res)
	wantContains(t, res.Out, "Usage:")
}

func TestLookup_RendersResult(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/lookup", http.StatusOK, map[string]any{
		"status":          "success",
		"executionTimeMs": 42.4,
		"queryMetadata":   map[string]any{"identifierScheme": "iso6523", "identifierValue": "0208:0123"},
		"dnsInfo":         map[string]any{"status": "success", "smpHostname": "smp.example", "smlHostname": "sml"},
		"businessCard": map[string]any{"status": "success", "entities": []map[string]any{{
			"name": "ACME NV", "countryCode": "BE", "identifiers": []map[string]any{{"scheme": "0208", "value": "0123"}},
		}}},
		"errors": []string{"certificate expires soon"},
	})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "lookup", "0208:0123")
	wantOK(t, res)
	wantContains(t, res.Out, "0208:0123", "smp.example", "42ms", "ACME NV", "BE", "0208: 0123", "certificate expires soon")
	if got := api.last("/api/lookup").Query.Get("peppol_id"); got != "0208:0123" {
		t.Errorf("peppol_id = %q", got)
	}

	res = runCLI(t, "", "lookup", "0208:0123", "--json")
	wantOK(t, res)
	if decodeJSON(t, res.Out)["status"] != "success" {
		t.Errorf("unexpected JSON: %s", res.Out)
	}

	t.Setenv("PEPPOL_API_KEY", "bad")
	wantCode(t, runCLI(t, "", "lookup", "0208:0123"), 2)
}

func TestLookupSearch_CountryAndRendering(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/lookup/participants", http.StatusOK, map[string]any{
		"total_count": 2,
		"participants": []map[string]any{
			{
				"peppol_id": "0208:1", "entities": []map[string]any{{"name": "ACME", "country_code": "BE"}},
				"document_types": []map[string]any{{"scheme": "s", "value": "v"}},
			},
			{"peppol_id": "0208:2"},
		},
	})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "lookup", "search", "acme", "--country", "BE")
	wantOK(t, res)
	wantContains(t, res.Out, "Found 2 participants", "0208:1", "ACME", "0208:2")
	q := api.last("/api/lookup/participants").Query
	if q.Get("query") != "acme" || q.Get("country_code") != "BE" {
		t.Errorf("query = %v", q)
	}

	wantOK(t, runCLI(t, "", "lookup", "search", "acme"))
	if q := api.last("/api/lookup/participants").Query; q.Has("country_code") {
		t.Errorf("country_code sent without --country: %v", q)
	}

	res = runCLI(t, "", "lookup", "search", "acme", "--json")
	wantOK(t, res)
	if decodeJSON(t, res.Out)["total_count"] != float64(2) {
		t.Errorf("unexpected JSON: %s", res.Out)
	}
}

// --- validate ---

func TestValidatePeppolID(t *testing.T) {
	api := newFakeAPI(t)
	api.handle("GET /api/validate/peppol-id", func(w http.ResponseWriter, r *http.Request) {
		valid := r.URL.Query().Get("peppol_id") == "0208:good"
		writeJSON(w, http.StatusOK, map[string]any{
			"is_valid": valid, "dns_valid": valid, "business_card_valid": false,
			"business_card": map[string]any{"name": "ACME", "country_code": "BE", "registration_date": "2020-01-01"},
			"supported_document_types": []string{
				"urn:oasis:names:specification:ubl:schema:xsd:Invoice-2::Invoice##urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0::2.1",
			},
		})
	})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "validate", "peppol-id", "0208:good")
	wantOK(t, res)
	wantContains(t, res.Out, "VALID", "ACME", "2020-01-01", "Supported Document Types (1)", "Invoice")

	// An invalid Peppol ID is a result, not an error.
	res = runCLI(t, "", "validate", "peppol-id", "0208:bad")
	wantOK(t, res)
	wantContains(t, res.Out, "INVALID")

	res = runCLI(t, "", "validate", "peppol-id", "0208:good", "--json")
	wantOK(t, res)
	if decodeJSON(t, res.Out)["is_valid"] != true {
		t.Errorf("unexpected JSON: %s", res.Out)
	}

	t.Setenv("PEPPOL_API_KEY", "bad")
	wantCode(t, runCLI(t, "", "validate", "peppol-id", "x"), 2)
}

func validationRoute(api *fakeAPI, route string) {
	api.handle(route, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "bad") {
			writeJSON(w, http.StatusOK, map[string]any{"is_valid": false, "issues": []map[string]any{
				{"type": "error", "message": "Missing buyer", "rule_id": "BR-07", "location": "/Invoice"},
				{"type": "warning", "message": "Odd date"},
			}})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"is_valid": true, "issues": []any{}})
	})
}

func TestValidateJSON_SourcesAndExitCodes(t *testing.T) {
	api := newFakeAPI(t)
	validationRoute(api, "POST /api/validate/json")
	useEnvKey(t, goodKey)
	good := writeTempFile(t, "good.json", `{"ok":true}`)
	bad := writeTempFile(t, "bad.json", `{"bad":true}`)

	res := runCLI(t, "", "validate", "json", good)
	wantOK(t, res)
	wantContains(t, res.Out, "Validation: PASSED")

	res = runCLI(t, "", "validate", "json", "--file", bad)
	wantCode(t, res, 3)
	wantContains(t, res.Out, "Validation: FAILED (1 errors, 1 warnings)", "BR-07", "Missing buyer", "/Invoice", "WARNING")

	// In JSON mode an invalid document is still printed and exits 0.
	res = runCLI(t, "", "validate", "json", bad, "--json")
	wantOK(t, res)
	if decodeJSON(t, res.Out)["is_valid"] != false {
		t.Errorf("unexpected JSON: %s", res.Out)
	}

	wantOK(t, runCLI(t, `{"ok":"stdin"}`, "validate", "json", "--file", "-"))
	if body := string(api.last("/api/validate/json").Body); body != `{"ok":"stdin"}` {
		t.Errorf("stdin body = %q", body)
	}

	missing := filepath.Join(t.TempDir(), "missing.json")
	for _, args := range [][]string{{missing}, {"--file", missing}} {
		res := runCLI(t, "", append([]string{"validate", "json"}, args...)...)
		wantCode(t, res, 1)
		wantContains(t, res.Err.Error(), "file not found")
	}

	res = runCLI(t, "", "validate", "json")
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "provide a file path")
}

// Regression: validate json resolved credentials before checking its
// arguments, so a user without credentials got an auth error (code 2) for a
// usage mistake.
func TestValidateJSON_UsageErrorBeforeAuth(t *testing.T) {
	isolateConfig(t)
	res := runCLI(t, "", "validate", "json")
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "provide a file path")

	missing := filepath.Join(t.TempDir(), "missing.json")
	for _, args := range [][]string{{"validate", "json", missing}, {"validate", "json", "--file", missing}} {
		res = runCLI(t, "", args...)
		wantCode(t, res, 1)
		wantContains(t, res.Err.Error(), "file not found")
	}

	// With valid input, missing credentials are still an auth error.
	wantCode(t, runCLI(t, "", "validate", "json", writeTempFile(t, "x.json", "{}")), 2)

	// A positional "-" is a filename, so a missing one is a usage error
	// before any credential check (and must not read stdin).
	t.Chdir(t.TempDir())
	res = runCLI(t, `{"from":"stdin"}`, "validate", "json", "-")
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "file not found: -")
}

// A file literally named "-" is uploaded as that file. --file - is still stdin,
// even when that file exists in the working directory.
func TestValidateJSON_PositionalDashIsFilename(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "-"), []byte(`{"from":"file"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	api := newFakeAPI(t)
	validationRoute(api, "POST /api/validate/json")
	useEnvKey(t, goodKey)

	res := runCLI(t, `{"from":"stdin"}`, "validate", "json", "-")
	wantOK(t, res)
	if body := string(api.last("/api/validate/json").Body); body != `{"from":"file"}` {
		t.Errorf("positional - body = %q, want the file contents", body)
	}

	wantOK(t, runCLI(t, `{"from":"stdin"}`, "validate", "json", "--file", "-"))
	if body := string(api.last("/api/validate/json").Body); body != `{"from":"stdin"}` {
		t.Errorf("--file - body = %q, want stdin", body)
	}
}

func TestValidateUBL(t *testing.T) {
	api := newFakeAPI(t)
	validationRoute(api, "POST /api/validate/ubl")
	useEnvKey(t, goodKey)

	wantOK(t, runCLI(t, "", "validate", "ubl", writeTempFile(t, "good.xml", "<ok/>")))
	res := runCLI(t, "", "validate", "ubl", writeTempFile(t, "bad.xml", "<bad/>"))
	wantCode(t, res, 3)

	isolateConfig(t)
	res = runCLI(t, "", "validate", "ubl", filepath.Join(t.TempDir(), "missing.xml"))
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "file not found")

	t.Setenv("PEPPOL_API_KEY", "bad")
	wantCode(t, runCLI(t, "", "validate", "ubl", writeTempFile(t, "x.xml", "<x/>")), 2)
}

// --- stats ---

func TestStats_QueryTextAndJSON(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/stats", http.StatusOK, map[string]any{
		"period_start": "2026-01-01", "period_end": "2026-01-31", "aggregation": "DAY",
		"total_days": 31, "average_daily_usage": 1.5, "budget_estimation_days": 120.0,
		"actions": []map[string]any{
			{"action": "DOCUMENT_SENT", "stat_date": "2026-01-02", "count": 3},
			{"action": "DOCUMENT_RECEIVED", "stat_date": "2026-01-02", "count": 4},
			{"action": "DOCUMENT_SENT", "stat_date": "2026-01-01", "count": 1},
		},
	})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "stats", "--from", "2026-01-01", "--to", "2026-01-31", "--aggregation", "DAY")
	wantOK(t, res)
	wantContains(t, res.Out, "2026-01-01 to 2026-01-31", "Total Days:          31", "1.50", "120 days", "DATE")
	q := api.last("/api/stats").Query
	if q.Get("start_date") != "2026-01-01" || q.Get("end_date") != "2026-01-31" || q.Get("aggregation") != "DAY" {
		t.Errorf("query = %v", q)
	}
	// Rows are sorted by date and SENT/RECEIVED are pivoted per day.
	i1, i2 := strings.Index(res.Out, "2026-01-01  "), strings.Index(res.Out, "2026-01-02")
	if i1 < 0 || i2 < 0 || i1 > i2 {
		t.Errorf("rows not sorted by date:\n%s", res.Out)
	}
	if !strings.Contains(res.Out, "2026-01-02  3     4") {
		t.Errorf("expected pivoted row for 2026-01-02:\n%s", res.Out)
	}

	res = runCLI(t, "", "stats", "--json")
	wantOK(t, res)
	if decodeJSON(t, res.Out)["total_days"] != float64(31) {
		t.Errorf("unexpected JSON: %s", res.Out)
	}
	if q := api.last("/api/stats").Query; len(q) != 0 {
		t.Errorf("unset flags were sent: %v", q)
	}

	t.Setenv("PEPPOL_API_KEY", "bad")
	wantCode(t, runCLI(t, "", "stats"), 2)
}

// --- misc ---

func TestCompletion_AllShells(t *testing.T) {
	for shell, marker := range map[string]string{
		"bash": "bash completion", "zsh": "#compdef", "fish": "complete -c", "powershell": "Register-ArgumentCompleter",
	} {
		res := runCLI(t, "", "completion", shell)
		wantOK(t, res)
		wantContains(t, res.Out, marker)
	}
	// An unsupported shell is a usage error.
	res := runCLI(t, "", "completion", "tcsh")
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), `unsupported shell "tcsh"`)

	// Without a shell the parent command still prints help.
	res = runCLI(t, "", "completion")
	wantOK(t, res)
	wantContains(t, res.Out, "Usage:")
}

func TestVersionCommand(t *testing.T) {
	res := runCLI(t, "", "version")
	wantOK(t, res)
	wantContains(t, res.Out, "peppol ", "commit:", "built:")
	if res := runCLI(t, "", "version", "extra"); res.Err == nil {
		t.Error("version should reject arguments")
	}
}
