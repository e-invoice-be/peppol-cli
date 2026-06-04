// Package backup implements the `peppol backup` command's archive engine.
//
// The engine maps the state of a target directory to one of three modes:
//
//   - fresh   — directory empty / missing
//   - resume  — manifest's most recent run has no completed_at
//   - top-up  — manifest's most recent run completed
//
// All file writes use temp-file + rename so a kill -9 never leaves a
// half-written file visible under its final name.
package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/e-invoicebe/peppol-cli/internal/version"
)

// Layout selects the on-disk shape of the backup.
type Layout string

const (
	LayoutFlat Layout = "flat"
	LayoutTree Layout = "tree"
)

const (
	schemaVersion = 1
	manifestName  = "manifest.json"
	stagingDir    = ".staging"
)

// Options configures a backup run.
type Options struct {
	Dir         string
	Layout      Layout
	Concurrency int
	Quiet       bool
	APIKey      string
	Client      *client.Client
	Stdout      io.Writer
	Stderr      io.Writer
	HTTPClient  *http.Client

	// MaxRetries caps per-request retry attempts on 429/5xx. Default 4.
	MaxRetries int
	// RetryBaseDelay is the base exponential backoff. Default 200ms.
	RetryBaseDelay time.Duration
}

// Result summarises a completed run.
type Result struct {
	DocumentsFetched   int
	DocumentsRefreshed int
	DocumentsSkipped   int
	StartedAt          time.Time
	CompletedAt        time.Time
}

// Manifest is the canonical metadata file at the root of a backup directory.
type Manifest struct {
	SchemaVersion     int               `json:"schema_version"`
	ToolVersion       string            `json:"tool_version"`
	TenantFingerprint string            `json:"tenant_fingerprint"`
	TenantName        string            `json:"tenant_name,omitempty"`
	Layout            Layout            `json:"layout"`
	DocumentCount     int               `json:"document_count"`
	HighWaterMark     string            `json:"high_water_mark"`
	DocumentStates    map[string]string `json:"document_states"`
	RunHistory        []RunRecord       `json:"run_history"`
}

// RunRecord captures one run's lifecycle. CompletedAt nil ⇒ run was interrupted.
type RunRecord struct {
	StartedAt          time.Time  `json:"started_at"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	NewDocuments       int        `json:"new_documents"`
	RefreshedDocuments int        `json:"refreshed_documents"`
}

// Mode reports how Run interpreted the directory state.
type Mode string

const (
	ModeFresh  Mode = "fresh"
	ModeResume Mode = "resume"
	ModeTopUp  Mode = "topup"
)

// Run executes a backup against opts.Dir. It is safe to call repeatedly: the
// directory state alone decides whether the run is fresh, a resume, or a top-up.
func Run(ctx context.Context, opts Options) (*Result, error) {
	if err := validateOptions(&opts); err != nil {
		return nil, err
	}

	startedAt := time.Now().UTC()
	fingerprint := fingerprintAPIKey(opts.APIKey)

	existing, mfPath, err := loadManifest(opts.Dir)
	if err != nil {
		return nil, err
	}

	mode := ModeFresh
	mf := &Manifest{
		SchemaVersion:     schemaVersion,
		ToolVersion:       version.Version,
		TenantFingerprint: fingerprint,
		Layout:            opts.Layout,
		DocumentStates:    map[string]string{},
	}
	if existing != nil {
		if existing.TenantFingerprint != fingerprint {
			return nil, fmt.Errorf(
				"refusing to back up: directory %q belongs to tenant %s but current API key is tenant %s",
				opts.Dir, existing.TenantFingerprint, fingerprint,
			)
		}
		if existing.Layout != opts.Layout {
			return nil, fmt.Errorf(
				"refusing to back up: directory %q uses layout %q but requested layout %q",
				opts.Dir, existing.Layout, opts.Layout,
			)
		}
		mf = existing
		mf.ToolVersion = version.Version
		if isLastRunIncomplete(existing) {
			mode = ModeResume
		} else {
			mode = ModeTopUp
		}
	}

	if err := ensureLayout(opts.Dir, opts.Layout); err != nil {
		return nil, err
	}
	mf.RunHistory = append(mf.RunHistory, RunRecord{StartedAt: startedAt})
	if err := writeManifest(mfPath, mf); err != nil {
		return nil, err
	}

	if opts.Client == nil {
		return nil, errors.New("backup: Options.Client is required")
	}
	c := opts.Client.WithContext(ctx)

	listing, err := enumerateDocuments(c, mode, mf)
	if err != nil {
		return nil, err
	}

	type doneEntry struct {
		item     workItem
		envelope archiveEntry
		err      error
	}

	jobs := make(chan workItem, len(listing))
	for _, w := range listing {
		jobs <- w
	}
	close(jobs)
	results := make(chan doneEntry, len(listing))

	var wg sync.WaitGroup
	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				env, err := fetchDocument(ctx, c, opts, job.docID)
				results <- doneEntry{item: job, envelope: env, err: err}
			}
		}()
	}
	wg.Wait()
	close(results)

	fetched := 0
	refreshed := 0
	for r := range results {
		if r.err != nil {
			_ = writeManifest(mfPath, mf)
			return nil, fmt.Errorf("backing up document %s: %w", r.item.docID, r.err)
		}
		if err := persistEnvelope(opts.Dir, opts.Layout, opts.HTTPClient, r.envelope); err != nil {
			_ = writeManifest(mfPath, mf)
			return nil, err
		}
		mf.DocumentStates[r.envelope.Document.ID] = string(r.envelope.Document.State)
		if t := r.envelope.Document.CreatedAt.UTC().Format(time.RFC3339); t > mf.HighWaterMark {
			mf.HighWaterMark = t
		}
		if r.item.refresh {
			refreshed++
		} else {
			fetched++
		}
	}

	if opts.Layout == LayoutFlat {
		if err := rebuildDocumentsJSONL(opts.Dir); err != nil {
			_ = writeManifest(mfPath, mf)
			return nil, err
		}
	}

	completedAt := time.Now().UTC()
	mf.DocumentCount = len(mf.DocumentStates)
	mf.RunHistory[len(mf.RunHistory)-1].CompletedAt = &completedAt
	mf.RunHistory[len(mf.RunHistory)-1].NewDocuments = fetched
	mf.RunHistory[len(mf.RunHistory)-1].RefreshedDocuments = refreshed
	if err := writeManifest(mfPath, mf); err != nil {
		return nil, err
	}

	return &Result{
		DocumentsFetched:   fetched,
		DocumentsRefreshed: refreshed,
		StartedAt:          startedAt,
		CompletedAt:        completedAt,
	}, nil
}

func validateOptions(opts *Options) error {
	if opts.Dir == "" {
		return errors.New("backup: Dir is required")
	}
	if opts.Layout == "" {
		opts.Layout = LayoutFlat
	}
	if opts.Layout != LayoutFlat && opts.Layout != LayoutTree {
		return fmt.Errorf("backup: unknown layout %q", opts.Layout)
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 8
	}
	if opts.MaxRetries <= 0 {
		opts.MaxRetries = 4
	}
	if opts.RetryBaseDelay <= 0 {
		opts.RetryBaseDelay = 200 * time.Millisecond
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}
	if opts.APIKey == "" {
		return errors.New("backup: APIKey is required (used for tenant fingerprint)")
	}
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return fmt.Errorf("creating backup dir: %w", err)
	}
	return nil
}

// fingerprintAPIKey returns a short stable identifier derived from the API key.
// Same key ⇒ same fingerprint ⇒ same tenant for guard purposes.
func fingerprintAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:8])
}

func loadManifest(dir string) (*Manifest, string, error) {
	mfPath := filepath.Join(dir, manifestName)
	raw, err := os.ReadFile(mfPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, mfPath, nil
	}
	if err != nil {
		return nil, mfPath, fmt.Errorf("reading manifest: %w", err)
	}
	if len(raw) == 0 {
		return nil, mfPath, nil
	}
	var mf Manifest
	if err := json.Unmarshal(raw, &mf); err != nil {
		return nil, mfPath, fmt.Errorf("parsing manifest: %w", err)
	}
	if mf.DocumentStates == nil {
		mf.DocumentStates = map[string]string{}
	}
	return &mf, mfPath, nil
}

func writeManifest(path string, mf *Manifest) error {
	raw, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding manifest: %w", err)
	}
	return atomicWrite(path, raw)
}

func isLastRunIncomplete(mf *Manifest) bool {
	if len(mf.RunHistory) == 0 {
		return false
	}
	return mf.RunHistory[len(mf.RunHistory)-1].CompletedAt == nil
}

func ensureLayout(dir string, layout Layout) error {
	subdirs := []string{stagingDir}
	switch layout {
	case LayoutFlat:
		subdirs = append(subdirs, "ubl", "attachments")
	case LayoutTree:
		subdirs = append(subdirs, "documents")
	}
	for _, sd := range subdirs {
		if err := os.MkdirAll(filepath.Join(dir, sd), 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", sd, err)
		}
	}
	return nil
}

// workItem queues one document's worth of work.
type workItem struct {
	docID   string
	refresh bool // top-up refresh of an in-flight document
}

// archiveEntry is what a worker returns to the main loop.
type archiveEntry struct {
	Document    client.DocumentResponse     `json:"document"`
	Timeline    client.DocumentTimeline     `json:"timeline"`
	Attachments []client.DocumentAttachment `json:"attachments"`
	UBL         client.DocumentUBL          `json:"ubl,omitempty"`
}

func enumerateDocuments(c *client.Client, mode Mode, mf *Manifest) ([]workItem, error) {
	seen := map[string]bool{}
	out := []workItem{}

	var fromDate string
	if mode == ModeTopUp && mf.HighWaterMark != "" {
		// API uses yyyy-mm-dd for date_from / date_to. Use the date part of the HWM.
		fromDate = mf.HighWaterMark[:10]
	}

	list := func(fn func(client.DocumentListParams) (*client.PaginatedDocuments, error)) error {
		params := client.DocumentListParams{
			PageSize:  100,
			SortBy:    "created_at",
			SortOrder: "asc",
			FromDate:  fromDate,
		}
		for params.Page = 1; ; params.Page++ {
			page, err := fn(params)
			if err != nil {
				return err
			}
			for _, d := range page.Items {
				if seen[d.ID] {
					continue
				}
				seen[d.ID] = true
				if _, ok := mf.DocumentStates[d.ID]; ok {
					// already archived; covered by refresh pass below if applicable
					continue
				}
				out = append(out, workItem{docID: d.ID})
			}
			if !page.HasNextPage {
				break
			}
		}
		return nil
	}

	if err := list(c.ListInbox); err != nil {
		return nil, fmt.Errorf("listing inbox: %w", err)
	}
	if err := list(c.ListOutbox); err != nil {
		return nil, fmt.Errorf("listing outbox: %w", err)
	}
	if err := list(c.ListDrafts); err != nil {
		return nil, fmt.Errorf("listing drafts: %w", err)
	}

	if mode == ModeTopUp {
		ids := make([]string, 0, len(mf.DocumentStates))
		for id := range mf.DocumentStates {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			if isRefreshable(client.DocumentState(mf.DocumentStates[id])) {
				out = append(out, workItem{docID: id, refresh: true})
			}
		}
	}

	return out, nil
}

// isRefreshable reports whether a document in this state should be re-fetched
// during a top-up. ADR-0001: DRAFT and TRANSIT only.
func isRefreshable(state client.DocumentState) bool {
	return state == client.DocumentStateDraft || state == client.DocumentStateTransit
}

func fetchDocument(ctx context.Context, c *client.Client, opts Options, id string) (archiveEntry, error) {
	var env archiveEntry

	doc, err := withRetry(ctx, opts, func() (*client.DocumentResponse, error) {
		return c.GetDocument(id)
	})
	if err != nil {
		return env, fmt.Errorf("document detail: %w", err)
	}
	env.Document = *doc

	tl, err := withRetry(ctx, opts, func() (*client.DocumentTimeline, error) {
		return c.GetDocumentTimeline(id)
	})
	if err != nil {
		return env, fmt.Errorf("timeline: %w", err)
	}
	env.Timeline = *tl

	atts, err := withRetry(ctx, opts, func() (*[]client.DocumentAttachment, error) {
		xs, err := c.ListAttachments(id)
		if err != nil {
			return nil, err
		}
		return &xs, nil
	})
	if err != nil {
		return env, fmt.Errorf("attachments: %w", err)
	}
	env.Attachments = *atts

	// Fetch per-attachment metadata to resolve signed URLs.
	for i, a := range env.Attachments {
		full, err := withRetry(ctx, opts, func() (*client.DocumentAttachment, error) {
			return c.GetAttachment(id, a.ID)
		})
		if err != nil {
			return env, fmt.Errorf("attachment %s detail: %w", a.ID, err)
		}
		env.Attachments[i] = *full
	}

	// UBL metadata — best-effort. Not all documents have a UBL.
	ubl, err := withRetry(ctx, opts, func() (*client.DocumentUBL, error) {
		return c.GetDocumentUBL(id)
	})
	if err == nil && ubl != nil {
		env.UBL = *ubl
	}
	return env, nil
}

// withRetry retries the function on transient errors (429, 5xx) with
// exponential backoff. Non-transient errors propagate immediately.
func withRetry[T any](ctx context.Context, opts Options, fn func() (*T, error)) (*T, error) {
	var lastErr error
	for attempt := 0; attempt < opts.MaxRetries; attempt++ {
		v, err := fn()
		if err == nil {
			return v, nil
		}
		if !isTransient(err) {
			return nil, err
		}
		lastErr = err
		d := time.Duration(1<<attempt) * opts.RetryBaseDelay
		d += time.Duration(rand.Int63n(int64(opts.RetryBaseDelay)))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(d):
		}
	}
	return nil, fmt.Errorf("exhausted retries: %w", lastErr)
}

func isTransient(err error) bool {
	var api *client.APIError
	if errors.As(err, &api) {
		if api.StatusCode == 429 || (api.StatusCode >= 500 && api.StatusCode < 600) {
			return true
		}
	}
	return false
}

// persistEnvelope writes one document's artifacts (UBL, attachments, envelope)
// to disk, all atomically.
func persistEnvelope(dir string, layout Layout, httpClient *http.Client, env archiveEntry) error {
	id := env.Document.ID

	// UBL XML — fetch the signed URL then write atomically. Best-effort: some
	// documents (e.g. failed drafts) have no UBL.
	ublMeta, err := getUBLMeta(env)
	if err == nil && ublMeta != nil && ublMeta.SignedURL != nil {
		body, err := fetchURL(httpClient, *ublMeta.SignedURL)
		if err != nil {
			return fmt.Errorf("fetching ubl for %s: %w", id, err)
		}
		dest := ublPath(dir, layout, id)
		if err := atomicWrite(dest, body); err != nil {
			return err
		}
	}

	// Attachments — fetch each via its signed file_url.
	for _, att := range env.Attachments {
		if att.FileURL == nil {
			continue
		}
		body, err := fetchURL(httpClient, *att.FileURL)
		if err != nil {
			return fmt.Errorf("fetching attachment %s/%s: %w", id, att.FileName, err)
		}
		dest := attachmentPath(dir, layout, id, att.FileName)
		if err := atomicWrite(dest, body); err != nil {
			return err
		}
	}

	// Envelope JSON.
	raw, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("encoding envelope %s: %w", id, err)
	}
	switch layout {
	case LayoutTree:
		dest := filepath.Join(dir, "documents", id)
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return err
		}
		docOnly, _ := json.MarshalIndent(env.Document, "", "  ")
		if err := atomicWrite(filepath.Join(dest, "document.json"), docOnly); err != nil {
			return err
		}
		tlOnly, _ := json.MarshalIndent(env.Timeline, "", "  ")
		if err := atomicWrite(filepath.Join(dest, "timeline.json"), tlOnly); err != nil {
			return err
		}
	}
	// Staging always — drives resume detection AND documents.jsonl rebuild.
	stagingPath := filepath.Join(dir, stagingDir, id+".json")
	return atomicWrite(stagingPath, raw)
}

// getUBLMeta hits /api/documents/{id}/ubl and returns the metadata. It needs
// the API client, which isn't currently threaded into persistEnvelope. To keep
// the call site small we let the worker pre-fetch UBL meta and attach it to
// the envelope. For now we re-derive: in fetchDocument we'd ideally store it.
// As an interim we treat env.Document.ID + a separate API call is not possible
// here — so we put it onto archiveEntry. Update fetchDocument accordingly.
func getUBLMeta(env archiveEntry) (*client.DocumentUBL, error) {
	if env.UBL.FileName == "" && env.UBL.SignedURL == nil {
		return nil, nil
	}
	return &env.UBL, nil
}

// fetchURL retrieves a URL's body, returning an error on non-2xx.
func fetchURL(httpClient *http.Client, url string) ([]byte, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

func ublPath(dir string, layout Layout, id string) string {
	if layout == LayoutTree {
		return filepath.Join(dir, "documents", id, "ubl.xml")
	}
	return filepath.Join(dir, "ubl", id+".xml")
}

func attachmentPath(dir string, layout Layout, id, name string) string {
	if layout == LayoutTree {
		return filepath.Join(dir, "documents", id, "attachments", name)
	}
	return filepath.Join(dir, "attachments", id, name)
}

// rebuildDocumentsJSONL concatenates every .staging/*.json into documents.jsonl
// in deterministic sorted order, atomically replacing any prior copy.
func rebuildDocumentsJSONL(dir string) error {
	stagingPath := filepath.Join(dir, stagingDir)
	entries, err := os.ReadDir(stagingPath)
	if err != nil {
		return fmt.Errorf("reading staging: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var buf bytes.Buffer
	for _, n := range names {
		raw, err := os.ReadFile(filepath.Join(stagingPath, n))
		if err != nil {
			return err
		}
		if err := json.Compact(&buf, raw); err != nil {
			return fmt.Errorf("compacting %s: %w", n, err)
		}
		buf.WriteByte('\n')
	}
	return atomicWrite(filepath.Join(dir, "documents.jsonl"), buf.Bytes())
}

// atomicWrite writes data to a temp file in the same directory, then renames it
// over path. On POSIX, rename(2) is atomic within a filesystem, guaranteeing no
// reader ever observes a half-written file at the final name.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}
