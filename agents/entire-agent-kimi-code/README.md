# Kimi Code for Entire

Locally maintained in [MooseGooseConsulting/external-agents](https://github.com/MooseGooseConsulting/external-agents), branch `agent/kimi-code`. Tested with Kimi 2.3.2 and Entire 0.12.1 on Windows.

## Install

From this directory, `go build -o entire-agent-kimi-code.exe ./cmd/entire-agent-kimi-code` on Windows, or omit `.exe` on Linux/macOS. Copy the binary to an **absolute** directory already on PATH (for example `$HOME/.local/bin`). Keep source in this fork so the binary can be rebuilt.

In each repository, run `entire enable --agent kimi-code --local --skip-push-sessions` (or `entire agent add kimi-code` when Entire is already enabled). Verify the untracked `.entire/settings.local.json` enables `external_agents`, then run `entire agent list`. Hooks are global but dispatch only in Entire-enabled repositories. Start a fresh Kimi session.

Nine managed rules use SessionStart, TurnStarted, Stop, StopFailure, Interrupt, PreCompact, SessionEnd, SubagentStart and SubagentStop. Stop is a valid native Kimi hook event, distinct from StopFailure and confirmed by the official Kimi Code hook documentation. TurnStarted is the only turn boundary and handles user and non-user turns. Child turns and stops are correlated using the wire log, so a foreground child cannot prematurely close its parent. Kimi 2.3.2's subagent hook payloads omit IDs; the adapter resolves the native subagent ID and parent tool call from `subagent.spawned` records.

## Lifecycle events

SessionEnd fires only on a graceful close — ACP `session/close` (plus closing the ACP stdio session) or an interactive TUI close. A print-mode `kimi -p` process exit does not emit SessionEnd, so it is never inferred from process exit. StopFailure is captured from a turn whose model fails mid-run; `scripts/capture-kimi-lifecycle.py` takes that model from `KIMI_LIFECYCLE_FAILURE_MODEL` and skips the scenario with a clear message when unset, so no personal model alias is baked in; the failure run's stderr is redacted of the model value before reporting.

## Lifecycle capture (existing-auth probe)

`py -3 scripts/capture-kimi-lifecycle.py --output <raw-capture-dir>` drives a real ACP session against the installed Kimi CLI using existing authentication (credentials are never copied or printed). It temporarily appends a comment-delimited hook block to the global `config.toml` through a guarded sibling-temp atomic `os.replace`, with a fresh comparison immediately before each swap. `finally` restores the original bytes only when the file still matches the capture-time content; detected concurrent drift keeps the temporary hooks in place and fails loudly for manual reconciliation — it is never silently clobbered. A hard kill outside `finally` can leave the temporary hook block in place, to be removed manually. Because the hooks are global, capture temporarily affects every Kimi repository on the machine; Entire dispatch itself stays repository scoped. Raw captures land in a disposable directory; the `sanitize` and `sanitize-transcript` subcommands turn a raw capture into the checked-in fixtures under `testdata/` (dropping payload-heavy and environment-specific records such as `llm.*` requests, `profile.bind` and `changeNotifier.snapshot`).

## Storage and capabilities

Native `$KIMI_CODE_HOME/sessions/.../wire.jsonl` and `state.json` are read-only. Byte-identical raw snapshots and restoration data live under ignored `.entire/metadata/kimi-code/`. Child snapshots use Entire's `<sessionId>/subagents/agent-<agentId>.jsonl` convention. Checkpoint compaction translates native prompts, assistant output, tool calls and results into Entire Transcript Format. Raw bytes remain available separately.

Declared: hooks, transcript analysis, compact transcript, token calculation and subagent aggregation. Token accounting counts `usage.record` once per call and ignores duplicate `step.end` usage. No model-generated summary provider, daemon, ACP service or historical import.

Structured Write/Edit paths are attributed; arbitrary shell writes are not guessed. Git remains authoritative for committed files. `write-session` restores only Entire-owned data, never native Kimi runtime files. `kimi --session <id>` resumes the existing native session; restoring a deleted native Kimi session on another machine is not supported.

## Verify

`go test ./...` and `go vet ./...` cover captured payloads, raw bytes, Unicode/partial appends, token accounting, foreground child correlation, lifecycle edge mapping (Stop, StopFailure, Interrupt, PreCompact, SessionEnd), path confinement and config preservation. Pure tests set isolated homes. `.github/workflows/kimi-code.yml` runs the module suite and pinned upstream protocol tests on Windows, macOS and Linux without Kimi credentials, and CI's `test-windows-paths (kimi-code)` job runs the path-security and portability subset (`TestSessionIDsAndResumeCommand`, `TestGetSessionDirAndResolveAgreement`, `TestSessionIndexMalformedAndMultipleRecords`, `TestSessionIndexRejectsEscapingSessionDir`, `TestNonCanonicalRepoPathResolves`, `TestManagedPathConfinement`, `TestReadSessionRestorationAndFallback`, `TestRejectForeignSessionAndPathEscapes`, `TestChildTranscriptsConfinedAndSkipsNonJSONL`, `TestRecordsParseCRLFAndSkipMalformedLines`) on Windows.

Run the shared compliance CLI **from a directory without go.mod**, with a fresh isolated `KIMI_CODE_HOME` and `--fixtures <absolute-path>/testdata/compliance.json`. Its launcher otherwise selects the current Go module and can produce a false green by running that module's tests instead of the compliance suite.

For a real authenticated local test: `py -3 scripts/smoke-kimi.py --output <evidence-directory>`. This uses existing authentication in place, creates a disposable repo, and proves Write → commit → checkpoint → raw and readable transcripts → token usage → real resumed Edit → second checkpoint. It never copies credentials. Agent/hooks must be installed before running. `py -3 scripts/capture-kimi-lifecycle.py --output <raw-capture-dir>` additionally captures real compaction, interrupt, failure and graceful-close hook payloads (see "Lifecycle capture" above); export `KIMI_LIFECYCLE_FAILURE_MODEL` to include the StopFailure scenario.

See `AGENT.md` for native schema research and captured lifecycle evidence.

## Config safety and removal

Installation appends a comment-delimited block, retaining unrelated config bytes, takes a timestamped backup beside config.toml, and writes atomically. Windows backups and temporary config files get an explicit current-user-only DACL before secret-bearing data is written. Installation uses a lock and rejects concurrent edits or damaged markers. `are-hooks-installed` checks every rule.

Run `entire-agent-kimi-code uninstall-hooks` to remove only the managed block. **This removes capture hooks globally**, affecting all repositories. Existing checkpoints remain. Remove the installed binary to stop discovery. Backups contain the original sensitive config; keep them private and outside Git.
