package backup_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/e-invoicebe/peppol-cli/internal/backup"
	"github.com/e-invoicebe/peppol-cli/internal/client"
)

// fakeBackend builds a stub e-invoice.be backend with the minimum endpoints
// the backup engine consumes. Pass a set of seed documents per direction; the
// backend wires up list pagination, detail/UBL/timeline/attachment endpoints,
// and a signed-URL storage server.
type seedDoc struct {
	id          string
	createdAt   string // RFC3339
	state       client.DocumentState
	direction   client.DocumentDirection
	ublXML      string
	attachments map[string]string // filename -> body
}

type fakeBackend struct {
	api     *httptest.Server
	storage *httptest.Server

	hitsMu sync.Mutex
	hits   map[string]int // path -> count

	inbox   []seedDoc
	outbox  []seedDoc
	drafts  []seedDoc
	allByID map[string]seedDoc
}

func (fb *fakeBackend) recordHit(path string) {
	fb.hitsMu.Lock()
	fb.hits[path]++
	fb.hitsMu.Unlock()
}

func (fb *fakeBackend) hitCount(path string) int {
	fb.hitsMu.Lock()
	defer fb.hitsMu.Unlock()
	return fb.hits[path]
}

func (fb *fakeBackend) hitsSnapshot() map[string]int {
	fb.hitsMu.Lock()
	defer fb.hitsMu.Unlock()
	out := make(map[string]int, len(fb.hits))
	for k, v := range fb.hits {
		out[k] = v
	}
	return out
}

func newFakeBackend(t *testing.T, inbox, outbox, drafts []seedDoc) *fakeBackend {
	t.Helper()
	fb := &fakeBackend{
		inbox:   inbox,
		outbox:  outbox,
		drafts:  drafts,
		allByID: map[string]seedDoc{},
		hits:    map[string]int{},
	}
	for _, d := range inbox {
		fb.allByID[d.id] = d
	}
	for _, d := range outbox {
		fb.allByID[d.id] = d
	}
	for _, d := range drafts {
		fb.allByID[d.id] = d
	}

	fb.storage = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fb.recordHit(r.URL.Path)
		// /storage/<doc_id>/ubl.xml
		// /storage/<doc_id>/att/<filename>
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 2 || parts[0] != "storage" {
			http.NotFound(w, r)
			return
		}
		docID := parts[1]
		doc, ok := fb.allByID[docID]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if len(parts) == 3 && parts[2] == "ubl.xml" {
			_, _ = w.Write([]byte(doc.ublXML))
			return
		}
		if len(parts) == 4 && parts[2] == "att" {
			body, ok := doc.attachments[parts[3]]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))

	mux := http.NewServeMux()
	listHandler := func(docs []seedDoc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			fb.recordHit(r.URL.Path)
			// honour ?date_from for top-up
			from := r.URL.Query().Get("date_from")
			filtered := docs
			if from != "" {
				cut, _ := time.Parse(time.RFC3339, from)
				out := []seedDoc{}
				for _, d := range docs {
					t, _ := time.Parse(time.RFC3339, d.createdAt)
					if !t.Before(cut) {
						out = append(out, d)
					}
				}
				filtered = out
			}
			items := make([]map[string]any, 0, len(filtered))
			for _, d := range filtered {
				items = append(items, map[string]any{
					"id":         d.id,
					"created_at": d.createdAt,
					"state":      string(d.state),
					"direction":  string(d.direction),
				})
			}
			resp := map[string]any{
				"items":         items,
				"total":         len(filtered),
				"page":          1,
				"page_size":     100,
				"pages":         1,
				"has_next_page": false,
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}
	mux.HandleFunc("/api/inbox/", listHandler(inbox))
	mux.HandleFunc("/api/outbox/", listHandler(outbox))
	mux.HandleFunc("/api/drafts/", listHandler(drafts))

	mux.HandleFunc("/api/me/", func(w http.ResponseWriter, r *http.Request) {
		fb.recordHit(r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":       "Test Tenant",
			"peppol_ids": []string{"0208:0123456789"},
		})
	})

	mux.HandleFunc("/api/documents/", func(w http.ResponseWriter, r *http.Request) {
		fb.recordHit(r.URL.Path)
		// /api/documents/<id>
		// /api/documents/<id>/ubl
		// /api/documents/<id>/timeline
		// /api/documents/<id>/attachments
		// /api/documents/<id>/attachments/<att_id>
		trim := strings.TrimPrefix(r.URL.Path, "/api/documents/")
		segments := strings.Split(trim, "/")
		docID := segments[0]
		doc, ok := fb.allByID[docID]
		if !ok {
			http.NotFound(w, r)
			return
		}
		switch {
		case len(segments) == 1:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":         doc.id,
				"created_at": doc.createdAt,
				"state":      string(doc.state),
				"direction":  string(doc.direction),
			})
		case len(segments) == 2 && segments[1] == "ubl":
			signed := fb.storage.URL + "/storage/" + doc.id + "/ubl.xml"
			_ = json.NewEncoder(w).Encode(map[string]any{
				"file_name":  doc.id + ".xml",
				"file_size":  len(doc.ublXML),
				"signed_url": signed,
			})
		case len(segments) == 2 && segments[1] == "timeline":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"document_id": doc.id,
				"events": []map[string]any{
					{"event_type": "document_created", "timestamp": doc.createdAt},
				},
			})
		case len(segments) == 2 && segments[1] == "attachments":
			out := []map[string]any{}
			for name := range doc.attachments {
				out = append(out, map[string]any{
					"id":        "att-" + name,
					"file_name": name,
					"file_type": "application/pdf",
					"file_size": len(doc.attachments[name]),
				})
			}
			_ = json.NewEncoder(w).Encode(out)
		case len(segments) == 3 && segments[1] == "attachments":
			// /api/documents/<id>/attachments/<att_id>
			attID := segments[2]
			name := strings.TrimPrefix(attID, "att-")
			body, ok := doc.attachments[name]
			if !ok {
				http.NotFound(w, r)
				return
			}
			signed := fb.storage.URL + "/storage/" + doc.id + "/att/" + name
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":        attID,
				"file_name": name,
				"file_type": "application/pdf",
				"file_size": len(body),
				"file_url":  signed,
			})
		default:
			http.NotFound(w, r)
		}
	})
	fb.api = httptest.NewServer(mux)
	return fb
}

func (fb *fakeBackend) close() {
	fb.api.Close()
	fb.storage.Close()
}

func (fb *fakeBackend) client() *client.Client {
	return client.NewClient("test-key", client.WithBaseURL(fb.api.URL))
}

// --- Tracer bullet: fresh run, flat layout, end-to-end ---

func TestRun_FreshFlat_WritesAllArtifacts(t *testing.T) {
	inboxDoc := seedDoc{
		id:        "inb-1",
		createdAt: "2026-01-10T10:00:00Z",
		state:     client.DocumentStateReceived,
		direction: client.DocumentDirectionInbound,
		ublXML:    `<Invoice id="inb-1"/>`,
	}
	outboxDoc := seedDoc{
		id:        "out-1",
		createdAt: "2026-02-15T12:00:00Z",
		state:     client.DocumentStateSent,
		direction: client.DocumentDirectionOutbound,
		ublXML:    `<Invoice id="out-1"/>`,
		attachments: map[string]string{
			"receipt.pdf": "fake-pdf-bytes",
		},
	}
	fb := newFakeBackend(t, []seedDoc{inboxDoc}, []seedDoc{outboxDoc}, nil)
	defer fb.close()

	dir := t.TempDir()
	opts := backup.Options{
		Dir:         dir,
		Layout:      backup.LayoutFlat,
		Concurrency: 2,
		Quiet:       true,
		APIKey:      "test-key",
		Client:      fb.client(),
	}
	res, err := backup.Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.DocumentsFetched != 2 {
		t.Errorf("DocumentsFetched = %d, want 2", res.DocumentsFetched)
	}

	// manifest.json exists with tenant + layout + HWM
	mfPath := filepath.Join(dir, "manifest.json")
	raw, err := os.ReadFile(mfPath)
	if err != nil {
		t.Fatalf("manifest.json: %v", err)
	}
	var mf map[string]any
	if err := json.Unmarshal(raw, &mf); err != nil {
		t.Fatalf("manifest unmarshal: %v", err)
	}
	if mf["layout"] != "flat" {
		t.Errorf("layout = %v, want flat", mf["layout"])
	}
	if mf["tenant_fingerprint"] == "" || mf["tenant_fingerprint"] == nil {
		t.Errorf("tenant_fingerprint missing")
	}
	if mf["high_water_mark"] != "2026-02-15T12:00:00Z" {
		t.Errorf("high_water_mark = %v, want 2026-02-15T12:00:00Z", mf["high_water_mark"])
	}

	// state index records both docs
	stateIdx, ok := mf["document_states"].(map[string]any)
	if !ok {
		t.Fatalf("document_states missing or wrong shape: %v", mf["document_states"])
	}
	if stateIdx["inb-1"] != "RECEIVED" {
		t.Errorf("inb-1 state = %v, want RECEIVED", stateIdx["inb-1"])
	}
	if stateIdx["out-1"] != "SENT" {
		t.Errorf("out-1 state = %v, want SENT", stateIdx["out-1"])
	}

	// documents.jsonl: one line per doc
	jsonlPath := filepath.Join(dir, "documents.jsonl")
	body, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("documents.jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("documents.jsonl has %d lines, want 2:\n%s", len(lines), body)
	}

	// ubl/<id>.xml
	for _, id := range []string{"inb-1", "out-1"} {
		p := filepath.Join(dir, "ubl", id+".xml")
		b, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("ubl %s: %v", id, err)
			continue
		}
		want := fmt.Sprintf(`<Invoice id="%s"/>`, id)
		if string(b) != want {
			t.Errorf("ubl %s body = %q, want %q", id, b, want)
		}
	}

	// attachments/out-1/receipt.pdf
	attPath := filepath.Join(dir, "attachments", "out-1", "receipt.pdf")
	att, err := os.ReadFile(attPath)
	if err != nil {
		t.Fatalf("attachment: %v", err)
	}
	if string(att) != "fake-pdf-bytes" {
		t.Errorf("attachment body = %q", att)
	}

	// no stray temp files left behind
	if leftovers := findTempFiles(t, dir); len(leftovers) > 0 {
		t.Errorf("temp files left behind: %v", leftovers)
	}
}

func findTempFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.Contains(filepath.Base(p), ".tmp") {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// --- Resume after mid-run interrupt ---

func TestRun_Resume_SkipsAlreadyArchivedDocs(t *testing.T) {
	docA := seedDoc{
		id:        "doc-a",
		createdAt: "2026-01-10T10:00:00Z",
		state:     client.DocumentStateReceived,
		direction: client.DocumentDirectionInbound,
		ublXML:    `<Invoice id="doc-a"/>`,
	}
	docB := seedDoc{
		id:        "doc-b",
		createdAt: "2026-01-11T10:00:00Z",
		state:     client.DocumentStateSent,
		direction: client.DocumentDirectionOutbound,
		ublXML:    `<Invoice id="doc-b"/>`,
	}
	fb := newFakeBackend(t, []seedDoc{docA}, []seedDoc{docB}, nil)
	defer fb.close()

	dir := t.TempDir()

	// Simulate a prior partial run: docA is on disk; manifest has open run.
	stagingDir := filepath.Join(dir, ".staging")
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// pre-stage docA envelope
	preEnv := map[string]any{
		"document": map[string]any{
			"id":         "doc-a",
			"created_at": "2026-01-10T10:00:00Z",
			"state":      "RECEIVED",
			"direction":  "INBOUND",
		},
		"timeline":    map[string]any{"document_id": "doc-a", "events": []any{}},
		"attachments": []any{},
	}
	preRaw, _ := json.Marshal(preEnv)
	if err := os.WriteFile(filepath.Join(stagingDir, "doc-a.json"), preRaw, 0o644); err != nil {
		t.Fatal(err)
	}

	// Pre-existing manifest with open run record.
	fingerprint := backup.FingerprintForTesting("test-key")
	prevMf := map[string]any{
		"schema_version":     1,
		"tool_version":       "test",
		"tenant_fingerprint": fingerprint,
		"layout":             "flat",
		"document_count":     1,
		"high_water_mark":    "2026-01-10T10:00:00Z",
		"document_states":    map[string]any{"doc-a": "RECEIVED"},
		"run_history": []any{
			map[string]any{
				"started_at": "2026-01-10T09:00:00Z",
				// no completed_at -> open run
			},
		},
	}
	prevRaw, _ := json.MarshalIndent(prevMf, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), prevRaw, 0o644); err != nil {
		t.Fatal(err)
	}

	// Run.
	res, err := backup.Run(context.Background(), backup.Options{
		Dir:         dir,
		Layout:      backup.LayoutFlat,
		Concurrency: 2,
		Quiet:       true,
		APIKey:      "test-key",
		Client:      fb.client(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if res.DocumentsFetched != 1 {
		t.Errorf("DocumentsFetched = %d, want 1 (only doc-b)", res.DocumentsFetched)
	}

	// API for doc-a detail must NOT have been called.
	if fb.hits["/api/documents/doc-a"] > 0 {
		t.Errorf("doc-a was re-fetched during resume; hits = %d", fb.hits["/api/documents/doc-a"])
	}
	if fb.hits["/api/documents/doc-b"] == 0 {
		t.Errorf("doc-b was NOT fetched; hits map: %v", fb.hits)
	}

	// documents.jsonl has exactly 2 lines, no duplicates.
	body, err := os.ReadFile(filepath.Join(dir, "documents.jsonl"))
	if err != nil {
		t.Fatalf("documents.jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("documents.jsonl has %d lines, want 2:\n%s", len(lines), body)
	}
	for _, want := range []string{"doc-a", "doc-b"} {
		if !strings.Contains(string(body), `"id":"`+want+`"`) {
			t.Errorf("documents.jsonl missing id %q\n%s", want, body)
		}
	}
}

// --- Top-up against completed dir ---

func TestRun_TopUp_RefreshesInFlightAndFetchesNew(t *testing.T) {
	docOldSent := seedDoc{id: "old-sent", createdAt: "2026-01-01T00:00:00Z", state: client.DocumentStateSent, direction: client.DocumentDirectionOutbound, ublXML: `<x id="old-sent"/>`}
	docOldFailed := seedDoc{id: "old-failed", createdAt: "2026-01-02T00:00:00Z", state: client.DocumentStateFailed, direction: client.DocumentDirectionOutbound, ublXML: `<x id="old-failed"/>`}
	docOldReceived := seedDoc{id: "old-received", createdAt: "2026-01-03T00:00:00Z", state: client.DocumentStateReceived, direction: client.DocumentDirectionInbound, ublXML: `<x id="old-received"/>`}
	docOldTransit := seedDoc{id: "old-transit", createdAt: "2026-01-04T00:00:00Z", state: client.DocumentStateTransit, direction: client.DocumentDirectionOutbound, ublXML: `<x id="old-transit"/>`}
	docOldDraft := seedDoc{id: "old-draft", createdAt: "2026-01-05T00:00:00Z", state: client.DocumentStateDraft, direction: client.DocumentDirectionOutbound, ublXML: `<x id="old-draft"/>`}
	docNew := seedDoc{id: "new-1", createdAt: "2026-03-01T00:00:00Z", state: client.DocumentStateSent, direction: client.DocumentDirectionOutbound, ublXML: `<x id="new-1"/>`}

	fb := newFakeBackend(t,
		[]seedDoc{docOldReceived},
		[]seedDoc{docOldSent, docOldFailed, docOldTransit, docNew},
		[]seedDoc{docOldDraft},
	)
	defer fb.close()

	dir := t.TempDir()

	// Seed a completed prior run: all five "old" docs already on disk + staging.
	stagingDir := filepath.Join(dir, ".staging")
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, d := range []seedDoc{docOldSent, docOldFailed, docOldReceived, docOldTransit, docOldDraft} {
		env := map[string]any{
			"document": map[string]any{
				"id":         d.id,
				"created_at": d.createdAt,
				"state":      string(d.state),
				"direction":  string(d.direction),
			},
			"timeline":    map[string]any{"document_id": d.id, "events": []any{}},
			"attachments": []any{},
		}
		raw, _ := json.Marshal(env)
		_ = os.WriteFile(filepath.Join(stagingDir, d.id+".json"), raw, 0o644)
	}

	completedAt := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	fingerprint := backup.FingerprintForTesting("test-key")
	prevMf := map[string]any{
		"schema_version":     1,
		"tool_version":       "test",
		"tenant_fingerprint": fingerprint,
		"layout":             "flat",
		"document_count":     5,
		"high_water_mark":    "2026-01-05T00:00:00Z",
		"document_states": map[string]any{
			"old-sent":     "SENT",
			"old-failed":   "FAILED",
			"old-received": "RECEIVED",
			"old-transit":  "TRANSIT",
			"old-draft":    "DRAFT",
		},
		"run_history": []any{
			map[string]any{
				"started_at":   "2026-01-31T00:00:00Z",
				"completed_at": completedAt.Format(time.RFC3339),
			},
		},
	}
	prevRaw, _ := json.MarshalIndent(prevMf, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), prevRaw, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := backup.Run(context.Background(), backup.Options{
		Dir:         dir,
		Layout:      backup.LayoutFlat,
		Concurrency: 2,
		Quiet:       true,
		APIKey:      "test-key",
		Client:      fb.client(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if res.DocumentsFetched != 1 {
		t.Errorf("DocumentsFetched = %d, want 1 (new-1)", res.DocumentsFetched)
	}
	if res.DocumentsRefreshed != 2 {
		t.Errorf("DocumentsRefreshed = %d, want 2 (old-transit, old-draft)", res.DocumentsRefreshed)
	}

	// Frozen docs must NOT have been re-fetched.
	for _, frozenID := range []string{"old-sent", "old-failed", "old-received"} {
		if fb.hits["/api/documents/"+frozenID] > 0 {
			t.Errorf("frozen doc %s was re-fetched (hits = %d)", frozenID, fb.hits["/api/documents/"+frozenID])
		}
	}
	// Refreshable docs must have been re-fetched.
	for _, liveID := range []string{"old-transit", "old-draft"} {
		if fb.hits["/api/documents/"+liveID] == 0 {
			t.Errorf("in-flight doc %s was NOT refreshed", liveID)
		}
	}
	// New doc fetched.
	if fb.hits["/api/documents/new-1"] == 0 {
		t.Errorf("new doc was NOT fetched")
	}

	// Manifest: HWM advanced; document_count grew by 1; new run record closed.
	raw, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
	var mf map[string]any
	_ = json.Unmarshal(raw, &mf)
	if mf["high_water_mark"] != "2026-03-01T00:00:00Z" {
		t.Errorf("HWM = %v, want 2026-03-01T00:00:00Z", mf["high_water_mark"])
	}
	if mf["document_count"] != float64(6) {
		t.Errorf("document_count = %v, want 6", mf["document_count"])
	}

	// documents.jsonl: 6 lines (5 old + 1 new), no duplicates.
	body, _ := os.ReadFile(filepath.Join(dir, "documents.jsonl"))
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 6 {
		t.Errorf("documents.jsonl has %d lines, want 6", len(lines))
	}
}

// --- Cross-tenant refusal ---

func TestRun_RefusesCrossTenant(t *testing.T) {
	fb := newFakeBackend(t, nil, nil, nil)
	defer fb.close()

	dir := t.TempDir()
	otherFingerprint := backup.FingerprintForTesting("some-other-key")

	prevMf := map[string]any{
		"schema_version":     1,
		"tool_version":       "test",
		"tenant_fingerprint": otherFingerprint,
		"layout":             "flat",
		"document_count":     0,
		"high_water_mark":    "",
		"document_states":    map[string]any{},
		"run_history": []any{
			map[string]any{"started_at": "2026-01-01T00:00:00Z", "completed_at": "2026-01-01T01:00:00Z"},
		},
	}
	raw, _ := json.MarshalIndent(prevMf, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o644)

	_, err := backup.Run(context.Background(), backup.Options{
		Dir:    dir,
		Layout: backup.LayoutFlat,
		APIKey: "current-key",
		Client: fb.client(),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, otherFingerprint) {
		t.Errorf("error must name the existing tenant fingerprint %q; got: %s", otherFingerprint, msg)
	}
	currentFingerprint := backup.FingerprintForTesting("current-key")
	if !strings.Contains(msg, currentFingerprint) {
		t.Errorf("error must name the current API key fingerprint %q; got: %s", currentFingerprint, msg)
	}

	// Strict: no API calls were made.
	if len(fb.hits) != 0 {
		t.Errorf("expected zero API hits before cross-tenant abort, got: %v", fb.hits)
	}
}

// --- Layout-mismatch refusal ---

func TestRun_RefusesLayoutMismatch(t *testing.T) {
	fb := newFakeBackend(t, nil, nil, nil)
	defer fb.close()

	dir := t.TempDir()
	fingerprint := backup.FingerprintForTesting("test-key")
	prevMf := map[string]any{
		"schema_version":     1,
		"tool_version":       "test",
		"tenant_fingerprint": fingerprint,
		"layout":             "flat",
		"document_count":     0,
		"high_water_mark":    "",
		"document_states":    map[string]any{},
		"run_history": []any{
			map[string]any{"started_at": "2026-01-01T00:00:00Z", "completed_at": "2026-01-01T01:00:00Z"},
		},
	}
	raw, _ := json.MarshalIndent(prevMf, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o644)

	_, err := backup.Run(context.Background(), backup.Options{
		Dir:    dir,
		Layout: backup.LayoutTree, // mismatch!
		APIKey: "test-key",
		Client: fb.client(),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	for _, want := range []string{"flat", "tree"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error must name layout %q; got: %s", want, err.Error())
		}
	}
	if len(fb.hits) != 0 {
		t.Errorf("expected zero API hits before layout-mismatch abort, got: %v", fb.hits)
	}
}

// --- Tree layout ---

func TestRun_FreshTree_PerDocDirectories(t *testing.T) {
	doc := seedDoc{
		id:        "tree-1",
		createdAt: "2026-01-10T10:00:00Z",
		state:     client.DocumentStateSent,
		direction: client.DocumentDirectionOutbound,
		ublXML:    `<Invoice id="tree-1"/>`,
		attachments: map[string]string{
			"receipt.pdf": "fake-pdf",
		},
	}
	fb := newFakeBackend(t, nil, []seedDoc{doc}, nil)
	defer fb.close()

	dir := t.TempDir()
	_, err := backup.Run(context.Background(), backup.Options{
		Dir:         dir,
		Layout:      backup.LayoutTree,
		Concurrency: 1,
		Quiet:       true,
		APIKey:      "test-key",
		Client:      fb.client(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Tree layout: per-doc directory.
	base := filepath.Join(dir, "documents", "tree-1")
	for _, want := range []string{"document.json", "ubl.xml", "timeline.json"} {
		p := filepath.Join(base, want)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s, got %v", p, err)
		}
	}
	attBody, err := os.ReadFile(filepath.Join(base, "attachments", "receipt.pdf"))
	if err != nil {
		t.Errorf("attachment missing: %v", err)
	}
	if string(attBody) != "fake-pdf" {
		t.Errorf("attachment body = %q", attBody)
	}

	// Tree layout DOES NOT produce documents.jsonl at the root.
	if _, err := os.Stat(filepath.Join(dir, "documents.jsonl")); !os.IsNotExist(err) {
		t.Errorf("documents.jsonl must not exist in tree layout; stat err = %v", err)
	}

	// Manifest records layout=tree.
	raw, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
	var mf map[string]any
	_ = json.Unmarshal(raw, &mf)
	if mf["layout"] != "tree" {
		t.Errorf("manifest layout = %v, want tree", mf["layout"])
	}
}

// --- Retry on 429 ---

func TestRun_RetriesTransientFailures(t *testing.T) {
	// /api/documents/doc-1 returns 429 once, then 200.
	var hits int
	var hitsMu sync.Mutex

	mux := http.NewServeMux()
	mux.HandleFunc("/api/inbox/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{{
				"id":         "doc-1",
				"created_at": "2026-01-10T10:00:00Z",
				"state":      "RECEIVED",
				"direction":  "INBOUND",
			}},
			"total": 1, "page": 1, "page_size": 100, "pages": 1, "has_next_page": false,
		})
	})
	mux.HandleFunc("/api/outbox/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "has_next_page": false})
	})
	mux.HandleFunc("/api/drafts/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "has_next_page": false})
	})
	mux.HandleFunc("/api/documents/", func(w http.ResponseWriter, r *http.Request) {
		// fail once on /api/documents/doc-1 (the detail endpoint, no trailing path)
		if r.URL.Path == "/api/documents/doc-1" {
			hitsMu.Lock()
			hits++
			n := hits
			hitsMu.Unlock()
			if n == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]any{"detail": "rate limited"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":         "doc-1",
				"created_at": "2026-01-10T10:00:00Z",
				"state":      "RECEIVED",
				"direction":  "INBOUND",
			})
			return
		}
		// timeline, ubl, attachments all succeed
		switch {
		case strings.HasSuffix(r.URL.Path, "/timeline"):
			_ = json.NewEncoder(w).Encode(map[string]any{"document_id": "doc-1", "events": []any{}})
		case strings.HasSuffix(r.URL.Path, "/ubl"):
			_ = json.NewEncoder(w).Encode(map[string]any{"file_name": "doc-1.xml", "file_size": 0})
		case strings.HasSuffix(r.URL.Path, "/attachments"):
			_ = json.NewEncoder(w).Encode([]any{})
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := client.NewClient("test-key", client.WithBaseURL(srv.URL))

	dir := t.TempDir()
	res, err := backup.Run(context.Background(), backup.Options{
		Dir:            dir,
		Layout:         backup.LayoutFlat,
		Concurrency:    1,
		Quiet:          true,
		APIKey:         "test-key",
		Client:         c,
		MaxRetries:     3,
		RetryBaseDelay: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.DocumentsFetched != 1 {
		t.Errorf("DocumentsFetched = %d, want 1", res.DocumentsFetched)
	}
	hitsMu.Lock()
	defer hitsMu.Unlock()
	if hits < 2 {
		t.Errorf("expected ≥2 hits on /api/documents/doc-1 (1 transient + 1 success), got %d", hits)
	}
}

// silence imports until later tests need them
var _ = io.Discard
