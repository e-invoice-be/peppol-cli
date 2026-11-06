# peppol-cli

Go CLI for the [e-invoice.be](https://e-invoice.be) Peppol Access Point API.

## Build & Test

```bash
go build -o peppol ./cmd/peppol   # Build binary
go test ./...                       # Run all tests
go vet ./...                        # Static analysis
make lint                           # golangci-lint v2.15.0 (config: .golangci.yml)
make fmt                            # gofumpt + goimports via golangci-lint
make vuln                           # govulncheck
make cover                          # tests + coverage, fails below 85% total
make check                          # everything CI runs (tidy, lint, vuln, cover)
```

## Code Quality

- `.golangci.yml` is strict (gosec, errorlint, noctx, gocritic, revive, ...). Fix findings rather than suppressing them; `//nolint` needs a specific linter and a `// reason` (enforced by nolintlint).
- HTTP requests must carry the command context (`http.NewRequestWithContext(cmd.Context(), ...)`) so Ctrl-C cancels them; compare errors with `errors.Is`/`errors.As`.
- CI enforces `go mod tidy`, lint, govulncheck, race tests and the coverage floor; run `make check` before pushing.
- Tests must never touch the real config dir, keyring or network: use `t.TempDir()`, `t.Setenv`, and the fake-API helpers in `internal/cli/harness_extra_test.go`.

## Project Structure

```
cmd/peppol/main.go          # Entry point → cli.Execute()
internal/
  cli/                       # Cobra commands (one file per command)
    root.go                  # Root command, global flags, Execute()
    auth.go                  # auth, auth status, auth logout
    me.go                    # me command
    stats.go                 # stats command
    mailbox.go               # mailbox list, get, attachment, reprocess
    completion.go            # completion bash|zsh|fish|powershell
  client/
    client.go                # HTTP client with Bearer auth
    generated.go             # Types from OpenAPI spec (manually maintained)
  config/
    config.go                # Config file + keyring credential storage
  version/
    version.go               # Build-time version info (ldflags)
api/
  openapi.json               # API specification
```

## Command Pattern

Every command follows this pattern (see `me.go` as reference):

1. `newXxxCmd() *cobra.Command` — creates the command with flags
2. `runXxx(cmd, args) error` — handler: `resolveKey()` → `client.NewClient()` → API call → render
3. JSON output: check `flags.JSON`, use `json.NewEncoder(cmd.OutOrStdout())`
4. Text output: `fmt.Fprintf(cmd.OutOrStdout(), ...)`
5. Errors: wrap in `&ExitError{Err: ..., Code: N}` (code 1 = general, code 2 = auth)
6. Register in `root.go`: `cmd.AddCommand(newXxxCmd())`

## Test Pattern

- **Client tests** (`client_test.go`): `httptest.NewServer` → `NewClient("key", WithBaseURL(srv.URL))` → assert
- **CLI tests** (`cli_test.go`): test client layer via httptest, test cobra wiring via `cmd.SetArgs([]string{"--help"})`, test rendering via `renderXxx` functions

## Adding a New Command

1. Create `internal/cli/<command>.go` with `newXxxCmd()` and `runXxx()`
2. Add API method to `internal/client/client.go` (follow `GetMe`/`GetStats` pattern)
3. Add response types to `internal/client/generated.go` if needed
4. Register command in `internal/cli/root.go`
5. Add tests in both `client_test.go` and `cli_test.go`

## Authentication

- API key resolution is `config.ResolveAPIKeySource` (non-empty `PEPPOL_API_KEY` first, else file keyring). `resolveKey()` and `auth status` both call `resolveKeySource`, so reporting stays aligned with usage.
- Config stored at `~/.config/peppol-cli/config.yaml`
- Client auto-injects `Authorization: Bearer <key>` via `authTransport`
