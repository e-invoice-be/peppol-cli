# peppol-cli

Go CLI (Cobra) for the [e-invoice.be](https://e-invoice.be) Peppol Access Point API.

- **Review**: when reviewing a diff, apply `CODING_STANDARDS.md`.
- **Spec sync / release**: to sync the CLI with the live OpenAPI spec or cut a release, follow `docs/sync-and-release.md`.

## Checks

- `make check` runs everything CI runs (tidy, lint, govulncheck, race tests, 85% coverage floor); run it before pushing.
- `make lint` uses whatever `golangci-lint` is on PATH; CI pins the version in `.github/workflows/ci.yml`, so match it locally.
- `.golangci.yml` is strict. Fix findings; a `//nolint` names one linter and carries a `// reason` (nolintlint enforces this).

## Layout

- `internal/output`: the renderer every command writes through (JSON, key-value, tables, `--quiet`, colour).
- `internal/client/generated.go`: API types maintained by hand from `api/openapi.json`. `make generate` only prints a note: oapi-codegen cannot read OpenAPI 3.1.
- `README.md` and `skills/peppol/` (`SKILL.md` + `references/*.md`): the user docs for every command and flag.

## Command pattern

`internal/cli/me.go` is the reference:

1. `newXxxCmd() *cobra.Command` builds the command and flags; register it in `root.go`.
2. `runXxx(cmd, args)`: `resolveKey()` → `client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())` → API call → render.
3. Render with `r := output.FromContext(cmd.Context())`: `if r.IsJSON() { return r.JSON(v) }`, otherwise `r.KeyValue(...)` / `r.Table(...)`.
4. Return `&ExitError{Err: ..., Code: N}`: 1 general, 2 auth (`client.ErrUnauthorized`, no key), 3 validation failed, 4 not found (`client.ErrNotFound`).

Client methods build requests with `http.NewRequestWithContext(c.ctx, ...)` so Ctrl-C cancels them; compare errors with `errors.Is`/`errors.As`.

## Tests

Tests stay off the real config dir, credentials and network.

- **CLI**: add tests to the `internal/cli/*_extra_test.go` file for the command group, using the harness in `harness_extra_test.go`: `newFakeAPI(t)` with `api.json("GET /api/...", status, body)`, `useEnvKey(t, goodKey)` or `isolateConfig(t)`, `runCLI(t, stdin, args...)`, then `wantOK` / `wantCode` / `wantContains` / `decodeJSON`. The harness swaps `http.DefaultTransport` and the global `flags`, so these tests run sequentially.
- **Client**: add the method to `newEndpointCases` in `internal/client/client_errors_extra_test.go` (covers status mapping, cancellation, transport errors), plus success-path tests in `client_test.go` with `httptest.NewServer` and `NewClient("key", WithBaseURL(srv.URL))`.

## Adding a command

1. Command in `internal/cli/<group>.go`, following the command pattern.
2. Client method in `internal/client/client.go`; new types in `generated.go`.
3. Tests as above.
4. Docs: `README.md` (commands table, examples) and `skills/peppol/` (`SKILL.md` command list + the matching `references/*.md`).

## Authentication

`resolveKey()` (in `me.go`) picks the workspace (`--workspace` flag, else the config's active one), then `PEPPOL_API_KEY` wins over the stored key. Keys are plain files in the config dir (`$XDG_CONFIG_HOME/peppol-cli`, default `~/.config/peppol-cli`): `credentials` or `credentials-<workspace>`, see `internal/config/keyring.go`.
