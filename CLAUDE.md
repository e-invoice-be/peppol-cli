# peppol-cli

Go CLI for the [e-invoice.be](https://e-invoice.be) Peppol Access Point API.

## Build & Test

```bash
go build -o peppol ./cmd/peppol   # Build binary
go test ./...                       # Run all tests
go vet ./...                        # Static analysis
```

## Project Structure

```
cmd/peppol/main.go          # Entry point → cli.Execute()
internal/
  cli/                       # Cobra commands (one file per command)
    root.go                  # Root command, global flags, Execute()
    auth.go                  # auth, auth status, auth logout
    me.go                    # me command
    stats.go                 # stats command
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

- `resolveKey()` checks `PEPPOL_API_KEY` env var first, then file-based keyring
- Config stored at `~/.config/peppol-cli/config.yaml`
- Client auto-injects `Authorization: Bearer <key>` via `authTransport`
