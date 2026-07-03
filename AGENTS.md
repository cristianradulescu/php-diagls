# AGENTS.md

## What this is
Go LSP server for PHP. It doesn't analyze PHP itself — it shells out to
`docker exec <container> sh -c "<cmd>"` to run tools (php-cs-fixer, phpstan,
phplint) that are installed inside a user-specified Docker container. See
`README.md` for the user-facing config format (`.php-diagls.json`).

## Commands
- Build: `make build` (or `go build -o php-diagls ./main.go`)
- Test everything: `make test` (== `go test -v ./...`)
- Test one package/case: `go test ./internal/diagnostics/... -run TestPhpCsFixer_Analyze -v`
- No linter or CI config exists in this repo. Before committing, run
  `go build ./... && go vet ./... && go test ./...` — that's the full bar.
- The `php-diagls` binary in the repo root is a gitignored build artifact; don't commit it.

## Architecture (flow)
`main.go` → `internal/server` (LSP JSON-RPC handlers) → `internal/diagnostics`
(one file per provider: `phpcsfixer.go`, `phpstan.go`, `phplint.go`, all
implementing the `DiagnosticsProvider` interface in `main.go`) →
`internal/container` (the only place that actually calls `docker exec`).
`internal/config` loads `.php-diagls.json`. `internal/formatting` wraps
whichever diagnostics provider has `Format.Enabled` (currently only
phpcsfixer) behind a separate `FormattingProvider` interface.

Adding a new diagnostics provider: implement `Id/Name/Analyze`, register it
in the switch in `internal/diagnostics/main.go` (`NewDiagnosticsProvider`),
and if it needs formatting support, also implement `CanFormat`/`Format` and
add a case in `internal/formatting/factory.go`. Update
`schema/php-diagls.schema.json` and `.php-diagls.example.json` to match any
new config fields (schema uses `additionalProperties: false`, so new fields
must be added explicitly or config validation breaks for editors that use
the schema).

## Non-obvious behavior / gotchas
- **Config is loaded once, at LSP `initialize`**, from the workspace root
  (or `RootURI`/cwd fallback). If `.php-diagls.json` isn't found, the server
  calls `os.Exit(0)` silently (no client-visible error). Diagnostics/format
  providers are cached after first load — editing `.php-diagls.json` has no
  effect until the LSP session is restarted.
- **Explicit file paths bypass php-cs-fixer's own `Finder`/`exclude()`
  config.** Because php-diagls always invokes `php-cs-fixer fix <path> ...`
  with an explicit file argument (never bare `fix` for discovery), the
  tool's own Finder-based excludes in `.php-cs-fixer.dist.php` are never
  consulted. Use each provider's `excludePaths` field in `.php-diagls.json`
  (checked via `utils.IsPathExcluded`, wired into every provider's
  `Analyze`) to replicate exclusions instead.
- **php-cs-fixer diagnostics run 2 docker execs per detected rule**: one
  `fix --dry-run` pass to find which rules are violated, then one more
  `fix --dry-run --rules <rule>` per violated rule to isolate its diff. This
  is intentionally how per-rule diagnostic ranges/messages are produced —
  it's not accidental N+1, but be aware it's not cheap.
- **Formatting never writes files.** `Format()` pipes content via stdin
  (`fix - --diff`), gets a unified diff back, and applies it locally with
  `utils.ApplyUnifiedDiff` (in `internal/utils`). Exit code `8` from
  php-cs-fixer means "changes found", not a failure — only other non-zero
  codes are treated as errors.
- Provider tests (`internal/diagnostics/*_test.go`, `internal/container/*_test.go`)
  deliberately use nonexistent container names/binaries so they pass without
  a real Docker daemon or container — they assert the graceful-failure path
  (empty diagnostics, no error), not real tool output. Don't assume Docker
  access is available or needed for `go test ./...`; the `docker` CLI binary
  just needs to exist on PATH (it does in this environment) for `exec.Command`
  to run and fail cleanly.
- The `-stdin` CLI flag only changes where logs go (stderr); actual LSP
  communication is always over stdin/stdout regardless of the flag.

## Style notes
- Commit messages are short, imperative, no prefix (e.g. "Fix formatting
  race condition, improve performance", "Add phplint").
- Tests are table-driven Go tests using `t.TempDir()`/`t.Context()`; mirror
  existing patterns in `internal/*/*_test.go` rather than introducing new
  mocking frameworks.
