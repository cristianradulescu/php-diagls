# Performance review

Review of the php-diagls implementation for performance issues, ranked by impact.
Line references are against the tree at commit `8eeb981`.

**Status: all findings implemented.** See the `Status` line under each section for the
commit that addressed it.

## Summary

| # | Finding | Location | Impact | Status |
|---|---------|----------|--------|--------|
| 1 | `didChange` re-analyzes the unchanged on-disk file | `internal/server/server.go:201` | High | ✅ Done (`aacda60`) |
| 2 | php-cs-fixer per-rule passes are fully serial | `internal/diagnostics/phpcsfixer.go:81` | High | ✅ Done (`1472ef6`, `ffcf942`) |
| 3 | No cancellation/timeout reaches the analysis | `internal/server/server.go:342`, `:373` | High | ✅ Done (`69cf3f4`) |
| 4 | `didClose` schedules a full analysis of a closed file | `internal/server/server.go:254` | Medium | ✅ Done (`dd93232`) |
| 5 | Regexes recompiled per call | several | Low | ✅ Done (`53e6824`) |
| 6 | Unbounded map growth, redundant work | several | Low | ✅ Done (`e9dad1f`) |

---

## 1. Diagnostics on `didChange` re-analyze the unchanged on-disk file

**`internal/server/server.go:201`**

The document cache (`s.documents`) is only ever *read* in `scheduleFormatting`. Every
provider's `Analyze(filePath)` shells out to a tool that reads the file **from disk inside
the container**. So each debounced keystroke burst fires a complete docker analysis
pipeline whose input is byte-identical to the previous run's — the results cannot differ
until a save happens.

For a file with ~8 php-cs-fixer violations that is roughly 17 `docker exec` invocations
(see finding 2) per 300 ms typing pause, all producing diagnostics that were already
published.

**Options**

- Drop `scheduleDiagnostics` from `handleDidChange` and rely on `handleDidSave`'s priority
  path.
- Or keep it, but skip when the buffer hash is unchanged since the last analyzed *disk*
  state.

Given that the tools read from disk, on-save-only is the honest behavior.

**Status: done (`aacda60`).** `handleDidChange` no longer calls `scheduleDiagnostics`;
diagnostics now only re-run on `didOpen`/`didSave`/`didChangeWatchedFiles`.

## 2. php-cs-fixer's per-rule passes are fully serial

**`internal/diagnostics/phpcsfixer.go:81-92`, `:107`**

`CLAUDE.md` documents the N+1 as intentional, and removing it is not proposed here — but
it is serialized, and `explainRule` (`:185`) adds another blocking `docker exec` per
uncached rule *inside* the same loop. Worst case for N rules is `1 + N + N` sequential
container round-trips, each paying PHP bootstrap (~0.3–1 s). N=8 gives ~15 s of wall time
for a single file.

**Fixes, cheapest first**

1. **Skip the extra pass when `len(file.Rules) == 1`.** The initial pass already returns
   `file.Diff`, and with a single rule that diff *is* the isolated diff. Single-violation
   files are the common case, and this makes them free.
2. **Run the per-rule passes through a bounded worker pool** (`errgroup` with
   `SetLimit(4)` or similar). They are independent; the only shared state is the
   diagnostics slice.
3. **Hoist `explainRule` out of the hot path.** Its `sync.Map` cache is good, but the
   misses are serial docker calls. Collect the distinct rules first, warm descriptions
   concurrently, then build diagnostics.

**Status: done (`1472ef6`, `ffcf942`).** Fix 1 (single-rule shortcut) and fix 2
(bounded worker pool, `maxConcurrentRuleAnalyses = 4`, extracted into `analyzeRule`) were
implemented. Fix 3 was superseded: with the per-rule passes now parallelized, `explainRule`
misses happen concurrently across rules anyway, so a separate warm-up pass wasn't needed.

## 3. No cancellation reaches the analysis — superseded work runs to completion

**`internal/server/server.go:342`, `:373`**

`DiagnosticsProvider.Analyze(filePath string)` takes no `context.Context`, and every
provider passes `context.Background()` to the container
(`phpcsfixer.go:64,83,191`, `phpstan.go:60`, `phplint.go:46`). The generation counter is
only checked *after* `collectDiagnostics` returns, so a stale run still burns its full
docker cost before being discarded.

`timer.Stop()` returning false (callback already fired) makes this routine, not rare:
rapid edits plus a save can leave several full pipelines in flight against the same file
concurrently, with no cap on how many.

**Two consequences worth separating**

- *Wasted CPU / docker load* — fixable by adding `ctx` to the `Analyze` signature and
  threading it through, then cancelling the previous generation's context when a new one
  is scheduled.
- *Unbounded goroutine and process lifetime* — there is no timeout on the diagnostics path
  at all (only `Format` has one, `phpcsfixer.go:224`). A wedged `docker exec` leaks a
  goroutine and a process for the life of the session. A default timeout on the
  diagnostics context, mirroring `Format.TimeoutSeconds`, closes that.

**Status: done (`69cf3f4`).** `DiagnosticsProvider.Analyze` now takes a `context.Context`,
threaded through all three providers into `RunCommandInContainer`. A new
`beginDiagnosticsRun` helper cancels any in-flight run for a URI before starting a new one
and bounds each run with a 30s `diagnosticsTimeout` (mirroring `Format`'s default).

## 4. `handleDidClose` schedules a full analysis of a closed file

**`internal/server/server.go:254`**

The intent is presumably to clear diagnostics, but this runs the whole provider pipeline
300 ms after close and publishes whatever it finds. Publishing
`[]protocol.Diagnostic{}` directly — as `handleDidChangeWatchedFiles` already does for
deletes (`server.go:237`) — gets the same result with zero docker calls.

**Status: done (`dd93232`).** `handleDidClose` now publishes empty diagnostics directly.
Also added `cancelScheduledDiagnostics` to stop/cancel any pending or in-flight analysis
for that URI and bump its generation, so a debounced run already in progress at close time
can't republish stale diagnostics afterward.

## 5. Regexes recompiled per call

- `parseDiffForDiagnostics` (`phpcsfixer.go:134`) compiles per invocation, and it is called
  once per rule per file.
- `explainRule` compiles three (`phpcsfixer.go:198-202`).
- `phplint.go:56` and `utils.go:130` compile per call.

All are constant patterns; hoisting them to package-level
`var … = regexp.MustCompile(…)` is mechanical. Small next to findings 1–3, but free.

**Unrelated correctness note spotted here:** `phplint.go:56`'s pattern
`[Fatal|Parse] error:` is a character class, not an alternation — it matches any single one
of `F a t l | P r s e`. It happens to work because the real output starts with `P` or `F`,
but it does not do what it reads as.

**Status: done (`53e6824`).** All listed regexes hoisted to package-level vars. The
`phplint.go` pattern was also corrected to `(?:Fatal|Parse) error:...`; verified manually
that real php-lint output still matches identically and that the old bug's false-positive
surface no longer matches.

## 6. Minor: unbounded map growth and redundant work

- `diagGen` / `fmtGen` / `fmtTimers` (`server.go:41-47`) are never pruned; entries persist
  per-URI for the session. Bounded by files touched, so small, but `handleDidClose` is the
  natural cleanup point.
- `FindProjectRoot` (`utils.go:20`) walks the tree with `os.Stat` per provider per
  analysis. The root is fixed at `initialize` and could be resolved once and stored.
- `collectDiagnostics` (`server.go:497`) rechecks a hardcoded `/vendor/`, `/var/cache/`
  list that overlaps with the per-provider `excludePaths` mechanism; the exclusion is then
  re-evaluated inside each provider's `Analyze`. Harmless, just duplicated.

**Status: done (`e9dad1f`).** `cancelScheduledDiagnostics` now deletes (rather than just
bumps) `diagGen`/`diagCancel` entries for a closed URI, and a new
`clearFormattingGeneration` does the same for `fmtGen`. `fmtTimers` was deliberately left
alone: its pending callback owns an LSP `reply` that must fire exactly once, so stopping
that timer early would hang the client's formatting request; deleting the generation entry
lets the callback still fire and take its existing "superseded" branch instead.
`FindProjectRoot` now memoizes by the file's starting directory, since the project root
can't change mid-session. The `collectDiagnostics` hardcoded ignore-list duplication was
left as-is — genuinely harmless and out of scope for this pass.

---

## Suggested order of work

Highest return for least churn: **findings 1 and 4** are small, localized edits that remove
most of the redundant docker traffic, and **finding 2's single-rule shortcut** is a few
lines. Finding 3 is the right fix architecturally but touches the `DiagnosticsProvider`
interface and all three providers.

All six findings were implemented in this order (1, 4, 2, 3, 5, 6), each as its own commit
with `go build ./... && go vet ./... && go test ./...` (plus `-race` for the concurrency
changes) passing before moving to the next.
