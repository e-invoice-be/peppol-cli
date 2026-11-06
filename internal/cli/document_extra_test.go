package cli

import (
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- document get ---

func TestDocumentGet_TextAndFull(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/documents/doc-1", http.StatusOK, sampleDoc("doc-1"))
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "document", "get", "doc-1")
	wantOK(t, res)
	wantContains(t, res.Out, "doc-1", "INV-doc-1", "Buyer BV", "Seller NV", "121.00 EUR", "2026-01-15")
	if strings.Contains(res.Out, "Widget") {
		t.Error("line items should only be shown with --full")
	}

	res = runCLI(t, "", "doc", "get", "doc-1", "--full")
	wantOK(t, res)
	wantContains(t, res.Out, "Widget", "50.00")
}

func TestDocumentGet_JSON(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/documents/doc-1", http.StatusOK, sampleDoc("doc-1"))
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "document", "get", "doc-1", "--json")
	wantOK(t, res)
	m := decodeJSON(t, res.Out)
	if m["id"] != "doc-1" || m["state"] != "DRAFT" {
		t.Errorf("unexpected JSON: %v", m)
	}
}

func TestDocumentGet_ErrorCodes(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/documents/missing", http.StatusNotFound, map[string]string{"detail": "Document not found"})
	api.json("GET /api/documents/boom", http.StatusBadGateway, map[string]string{"detail": "upstream"})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "document", "get", "missing")
	wantCode(t, res, 4)
	wantContains(t, res.Err.Error(), "peppol inbox list")

	res = runCLI(t, "", "document", "get", "boom")
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "502", "upstream")

	t.Setenv("PEPPOL_API_KEY", "bad")
	wantCode(t, runCLI(t, "", "document", "get", "doc-1"), 2)
}

func TestDocumentGet_RequiresExactlyOneArg(t *testing.T) {
	useEnvKey(t, goodKey)
	if res := runCLI(t, "", "document", "get"); res.Err == nil {
		t.Error("expected error without document id")
	}
	if res := runCLI(t, "", "document", "get", "a", "b"); res.Err == nil {
		t.Error("expected error with two ids")
	}
}

// --- timeline / lifecycle ---

func TestDocumentTimeline_TextJSONAndErrors(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/documents/doc-1/timeline", http.StatusOK, map[string]any{
		"document_id": "doc-1",
		"events": []map[string]any{
			{"event_type": "DOCUMENT_CREATED", "timestamp": "2026-01-15T10:00:00Z"},
			{"event_type": "PEPPOL_SENT", "timestamp": "2026-01-15T10:05:30Z"},
		},
	})
	api.json("GET /api/documents/empty/timeline", http.StatusOK, map[string]any{"document_id": "empty", "events": []any{}})
	api.json("GET /api/documents/gone/timeline", http.StatusNotFound, map[string]string{"detail": "nope"})
	api.json("GET /api/documents/err/timeline", http.StatusInternalServerError, map[string]string{"detail": "x"})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "document", "timeline", "doc-1")
	wantOK(t, res)
	wantContains(t, res.Out, "2026-01-15 10:00:00  DOCUMENT CREATED", "2026-01-15 10:05:30  PEPPOL SENT")

	res = runCLI(t, "", "document", "timeline", "doc-1", "--json")
	wantOK(t, res)
	if ev, ok := decodeJSON(t, res.Out)["events"].([]any); !ok || len(ev) != 2 {
		t.Errorf("expected 2 events in JSON, got %s", res.Out)
	}

	res = runCLI(t, "", "document", "timeline", "empty")
	wantOK(t, res)
	wantContains(t, res.Out, "No timeline events.")

	wantCode(t, runCLI(t, "", "document", "timeline", "gone"), 4)
	wantCode(t, runCLI(t, "", "document", "timeline", "err"), 1)
	t.Setenv("PEPPOL_API_KEY", "bad")
	wantCode(t, runCLI(t, "", "document", "timeline", "doc-1"), 2)
}

func TestDocumentLifecycle_JSONAndAuthError(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/documents/doc-1/lifecycle", http.StatusOK, map[string]any{
		"document_id": "doc-1",
		"events": []map[string]any{{
			"level": "MLR", "canonical_status": "delivered", "source": "PEPPOL",
			"direction": "INBOUND", "native_code": "AP", "occurred_at": "2026-01-16T08:00:00Z",
		}},
	})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "document", "lifecycle", "doc-1", "--json")
	wantOK(t, res)
	if decodeJSON(t, res.Out)["document_id"] != "doc-1" {
		t.Errorf("unexpected JSON: %s", res.Out)
	}

	t.Setenv("PEPPOL_API_KEY", "bad")
	wantCode(t, runCLI(t, "", "document", "lifecycle", "doc-1"), 2)
}

// --- create ---

func TestDocumentCreate_MissingFileFailsBeforeAuth(t *testing.T) {
	api := newFakeAPI(t)
	isolateConfig(t) // no key: a missing file must win over the auth error
	missing := filepath.Join(t.TempDir(), "nope")
	for _, kind := range []string{"json", "ubl", "pdf"} {
		t.Run(kind, func(t *testing.T) {
			res := runCLI(t, "", "document", "create", kind, missing)
			wantCode(t, res, 1)
			wantContains(t, res.Err.Error(), "file not found")
		})
	}
	if n := len(api.requests()); n != 0 {
		t.Errorf("expected no API calls, got %d", n)
	}
}

func TestDocumentCreateJSON_PostsFileAndRendersResult(t *testing.T) {
	api := newFakeAPI(t)
	api.json("POST /api/documents/", http.StatusCreated, sampleDoc("new-1"))
	useEnvKey(t, goodKey)
	path := writeTempFile(t, "inv.json", `{"invoice_id":"X1"}`)

	res := runCLI(t, "", "document", "create", "json", path, "--construct-pdf")
	wantOK(t, res)
	wantContains(t, res.Out, "Document created successfully.", "new-1")

	req := api.last("/api/documents/")
	if string(req.Body) != `{"invoice_id":"X1"}` {
		t.Errorf("body = %q, want the file content", req.Body)
	}
	if req.Query.Get("construct_pdf") != "true" {
		t.Errorf("construct_pdf = %q, want true", req.Query.Get("construct_pdf"))
	}
	if ct := req.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}

	res = runCLI(t, "", "document", "create", "json", path, "--json")
	wantOK(t, res)
	if decodeJSON(t, res.Out)["id"] != "new-1" {
		t.Errorf("unexpected JSON: %s", res.Out)
	}
	if q := api.last("/api/documents/").Query; q.Has("construct_pdf") {
		t.Errorf("construct_pdf should be absent without the flag, query %v", q)
	}
}

func TestDocumentCreateJSON_ValidationErrorDetailIsShown(t *testing.T) {
	api := newFakeAPI(t)
	api.json("POST /api/documents/", http.StatusUnprocessableEntity, map[string]any{
		"detail": []map[string]any{{"loc": []any{"body", "invoice_id"}, "msg": "field required", "type": "missing"}},
	})
	useEnvKey(t, goodKey)
	path := writeTempFile(t, "inv.json", `{}`)

	res := runCLI(t, "", "document", "create", "json", path)
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "422", "body.invoice_id: field required")
}

func TestDocumentCreateUBL_UploadsMultipart(t *testing.T) {
	api := newFakeAPI(t)
	var gotName, gotContent, gotType string
	api.handle("POST /api/documents/ubl", func(w http.ResponseWriter, r *http.Request) {
		gotName, gotType, gotContent = readMultipartFile(t, r)
		writeJSON(w, http.StatusCreated, sampleDoc("ubl-1"))
	})
	useEnvKey(t, goodKey)
	path := writeTempFile(t, "invoice.xml", "<Invoice/>")

	res := runCLI(t, "", "document", "create", "ubl", path)
	wantOK(t, res)
	wantContains(t, res.Out, "Document created from UBL successfully.", "ubl-1")
	if gotName != "invoice.xml" || gotContent != "<Invoice/>" {
		t.Errorf("uploaded %q with %q", gotName, gotContent)
	}
	if !strings.Contains(gotType, "xml") {
		t.Errorf("part Content-Type = %q, want an XML type", gotType)
	}

	t.Setenv("PEPPOL_API_KEY", "bad")
	wantCode(t, runCLI(t, "", "document", "create", "ubl", path), 2)
}

// readMultipartFile returns the filename, Content-Type and content of the
// "file" part of a multipart request.
func readMultipartFile(t *testing.T, r *http.Request) (name, contentType, content string) {
	t.Helper()
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		t.Errorf("parse Content-Type: %v", err)
		return "", "", ""
	}
	mr := multipart.NewReader(r.Body, params["boundary"])
	part, err := mr.NextPart()
	if err != nil {
		t.Errorf("next part: %v", err)
		return "", "", ""
	}
	data, err := io.ReadAll(part)
	if err != nil {
		t.Errorf("read part: %v", err)
	}
	if part.FormName() != "file" {
		t.Errorf("form field = %q, want file", part.FormName())
	}
	return part.FileName(), part.Header.Get("Content-Type"), string(data)
}

func TestDocumentCreatePDF_PassesTaxIDsAndReportsFailure(t *testing.T) {
	api := newFakeAPI(t)
	resp := sampleDoc("pdf-1")
	resp["success"] = false
	resp["error_type"] = "EXTRACTION"
	resp["error_message"] = "unreadable scan"
	api.json("POST /api/documents/pdf", http.StatusCreated, resp)
	useEnvKey(t, goodKey)
	path := writeTempFile(t, "inv.pdf", "%PDF-1.4")

	res := runCLI(t, "", "document", "create", "pdf", path, "--vendor-tax-id", "BE0111", "--customer-tax-id", "BE0222")
	wantOK(t, res)
	wantContains(t, res.Out, "PDF conversion failed", "EXTRACTION", "unreadable scan", "pdf-1")
	q := api.last("/api/documents/pdf").Query
	if q.Get("vendor_tax_id") != "BE0111" || q.Get("customer_tax_id") != "BE0222" {
		t.Errorf("query = %v", q)
	}

	res = runCLI(t, "", "document", "create", "pdf", path, "--json")
	wantOK(t, res)
	if decodeJSON(t, res.Out)["success"] != false {
		t.Errorf("unexpected JSON: %s", res.Out)
	}
}

// Tax IDs containing "&" must not inject extra query parameters.
func TestDocumentCreatePDF_EscapesTaxIDs(t *testing.T) {
	api := newFakeAPI(t)
	api.json("POST /api/documents/pdf", http.StatusCreated, sampleDoc("pdf-1"))
	useEnvKey(t, goodKey)
	path := writeTempFile(t, "inv.pdf", "%PDF-1.4")

	wantOK(t, runCLI(t, "", "document", "create", "pdf", path, "--vendor-tax-id", "BE01&customer_tax_id=EVIL"))
	q := api.last("/api/documents/pdf").Query
	if q.Get("vendor_tax_id") != "BE01&customer_tax_id=EVIL" || q.Has("customer_tax_id") {
		t.Errorf("tax id was not escaped, server saw %v", q)
	}
}

// --- send ---

func TestDocumentSend_FlagsReachQuery(t *testing.T) {
	api := newFakeAPI(t)
	sent := sampleDoc("doc-1")
	sent["state"] = "TRANSIT"
	api.json("POST /api/documents/doc-1/send", http.StatusOK, sent)
	api.json("POST /api/documents/gone/send", http.StatusNotFound, map[string]string{"detail": "x"})
	api.json("POST /api/documents/bad/send", http.StatusBadRequest, map[string]string{"detail": "Receiver not on Peppol"})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "document", "send", "doc-1",
		"--sender-peppol-id", "0208:1", "--sender-peppol-scheme", "iso6523-actorid-upis",
		"--receiver-peppol-id", "0208:2", "--receiver-peppol-scheme", "iso6523-actorid-upis")
	wantOK(t, res)
	wantContains(t, res.Out, "Document sent successfully.", "TRANSIT")
	q := api.last("/api/documents/doc-1/send").Query
	for k, v := range map[string]string{
		"sender_peppol_id": "0208:1", "sender_peppol_scheme": "iso6523-actorid-upis",
		"receiver_peppol_id": "0208:2", "receiver_peppol_scheme": "iso6523-actorid-upis",
	} {
		if q.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, q.Get(k), v)
		}
	}
	if q.Has("email") {
		t.Error("email should not be sent when unset")
	}

	res = runCLI(t, "", "document", "send", "doc-1", "--json")
	wantOK(t, res)
	if decodeJSON(t, res.Out)["state"] != "TRANSIT" {
		t.Errorf("unexpected JSON: %s", res.Out)
	}

	wantCode(t, runCLI(t, "", "document", "send", "gone"), 4)
	res = runCLI(t, "", "document", "send", "bad")
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "Receiver not on Peppol")
}

// --- validate (stored document) ---

func TestDocumentValidate_ValidInvalidAndErrors(t *testing.T) {
	api := newFakeAPI(t)
	api.json("POST /api/documents/ok/validate", http.StatusOK, map[string]any{"id": "ok", "is_valid": true, "issues": []any{}})
	api.json("POST /api/documents/bad/validate", http.StatusOK, map[string]any{
		"id": "bad", "is_valid": false,
		"issues": []map[string]any{{"type": "error", "message": "Missing buyer", "rule_id": "BR-07"}},
	})
	api.json("POST /api/documents/gone/validate", http.StatusNotFound, map[string]string{"detail": "x"})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "document", "validate", "ok")
	wantOK(t, res)
	wantContains(t, res.Out, "Document is valid.")

	// An invalid stored document is reported, not turned into an error.
	res = runCLI(t, "", "document", "validate", "bad")
	wantOK(t, res)
	wantContains(t, res.Out, "Document is not valid.", "Missing buyer", "BR-07")

	res = runCLI(t, "", "document", "validate", "bad", "--json")
	wantOK(t, res)
	if decodeJSON(t, res.Out)["is_valid"] != false {
		t.Errorf("unexpected JSON: %s", res.Out)
	}

	wantCode(t, runCLI(t, "", "document", "validate", "gone"), 4)
	t.Setenv("PEPPOL_API_KEY", "bad")
	wantCode(t, runCLI(t, "", "document", "validate", "ok"), 2)
}

// --- delete ---

func TestDocumentDelete_ConfirmationAndResults(t *testing.T) {
	api := newFakeAPI(t)
	api.json("DELETE /api/documents/doc-1", http.StatusOK, map[string]any{"is_deleted": true})
	api.json("DELETE /api/documents/gone", http.StatusNotFound, map[string]string{"detail": "x"})
	useEnvKey(t, goodKey)

	res := runCLI(t, "YES\n", "document", "delete", "doc-1")
	wantOK(t, res)
	wantContains(t, res.Out, "Delete document doc-1? [y/N]", "Document deleted.")

	res = runCLI(t, "", "document", "delete", "doc-1")
	wantOK(t, res)
	wantContains(t, res.Out, "Aborted.")

	res = runCLI(t, "", "document", "delete", "doc-1", "-y")
	wantOK(t, res)
	if strings.Contains(res.Out, "[y/N]") {
		t.Error("--yes should skip the prompt")
	}

	res = runCLI(t, "", "document", "delete", "doc-1", "--yes", "--json")
	wantOK(t, res)
	if decodeJSON(t, res.Out)["is_deleted"] != true {
		t.Errorf("unexpected JSON: %s", res.Out)
	}

	wantCode(t, runCLI(t, "", "document", "delete", "gone", "-y"), 4)

	deletes := 0
	for _, r := range api.requests() {
		if r.Method == http.MethodDelete && r.Path == "/api/documents/doc-1" {
			deletes++
		}
	}
	if deletes != 3 {
		t.Errorf("DELETE calls = %d, want 3 (the aborted run must not delete)", deletes)
	}
}

func TestDocumentDelete_AbortNeedsNoCredentials(t *testing.T) {
	isolateConfig(t)
	res := runCLI(t, "n\n", "document", "delete", "doc-1")
	wantOK(t, res)
	wantContains(t, res.Out, "Aborted.")
}

// With --json and no --yes the confirmation prompt goes to stderr, so stdout
// is exactly the JSON document.
func TestDocumentDelete_JSONWithPromptIsParseable(t *testing.T) {
	api := newFakeAPI(t)
	api.json("DELETE /api/documents/doc-1", http.StatusOK, map[string]any{"is_deleted": true})
	useEnvKey(t, goodKey)
	stdout, stderr, err := runCLISplit(t, "y\n", "document", "delete", "doc-1", "--json")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	decodeJSON(t, stdout)
	wantContains(t, stderr, "Delete document doc-1? [y/N]")
}

// --- ubl ---

func TestDocumentUBL_DownloadsToStdoutAndFile(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/documents/doc-1/ubl", http.StatusOK, map[string]any{
		"id": "doc-1", "file_name": "doc-1.xml", "file_size": 10, "signed_url": api.fileURL("doc-1.xml"),
	})
	api.handle("GET /files/doc-1.xml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "<Invoice>1</Invoice>")
	})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "document", "ubl", "doc-1")
	wantOK(t, res)
	if res.Out != "<Invoice>1</Invoice>" {
		t.Errorf("stdout = %q, want raw XML only", res.Out)
	}

	out := filepath.Join(t.TempDir(), "out.xml")
	res = runCLI(t, "", "document", "ubl", "doc-1", "-o", out)
	wantOK(t, res)
	wantContains(t, res.Out, "UBL XML written to "+out)
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "<Invoice>1</Invoice>" {
		t.Errorf("file = %q", data)
	}

	// The signed URL is fetched without the API key.
	if auth := api.last("/files/doc-1.xml").Auth; auth != "" {
		t.Errorf("signed URL request carried Authorization %q", auth)
	}
}

func TestDocumentUBL_Errors(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/documents/nourl/ubl", http.StatusOK, map[string]any{"id": "nourl", "file_name": "x.xml", "file_size": 0})
	api.json("GET /api/documents/expired/ubl", http.StatusOK, map[string]any{
		"id": "expired", "file_name": "x.xml", "file_size": 1, "signed_url": api.fileURL("expired.xml"),
	})
	api.handle("GET /files/expired.xml", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	api.json("GET /api/documents/gone/ubl", http.StatusNotFound, map[string]string{"detail": "x"})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "document", "ubl", "nourl")
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "no UBL file available")

	res = runCLI(t, "", "document", "ubl", "expired")
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "HTTP 403")

	wantCode(t, runCLI(t, "", "document", "ubl", "gone"), 4)
}

func TestDocumentUBL_UnwritableOutputIsExitCode1(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/documents/doc-1/ubl", http.StatusOK, map[string]any{
		"id": "doc-1", "file_name": "doc-1.xml", "file_size": 1, "signed_url": api.fileURL("doc-1.xml"),
	})
	api.handle("GET /files/doc-1.xml", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "<x/>") })
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "document", "ubl", "doc-1", "-o", filepath.Join(t.TempDir(), "missing-dir", "f.xml"))
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "creating output file")
}
