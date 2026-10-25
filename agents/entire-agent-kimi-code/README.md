# Kimi Code for Entire

Locally maintained in [MooseGooseConsulting/external-agents](https://github.com/MooseGooseConsulting/external-agents), with runtime hardening on `codex/kimi-local` for integration into `agent/kimi-code`. Tested with Kimi 2.3.2 and Entire 0.12.1 on Windows.

## Install

From this directory, `go build -o entire-agent-kimi-code.exe ./cmd/entire-agent-kimi-code` on Windows, or omit `.exe` on Linux/macOS. Copy the binary to an **absolute** directory already on PATH (for example `$HOME/.local/bin`). Keep source in this fork so the binary can be rebuilt.

In each repository, run `entire enable --agent kimi-code --local --skip-push-sessions` (or `entire agent add kimi-code` when Entire is already enabled). Verify the untracked `.entire/settings.local.json` enables `external_agents`, then run `entire agent list`. Hooks are global but dispatch only in Entire-enabled repositories. Start a fresh Kimi session.

Ten managed rules use SessionStart, UserPromptSubmit, TurnStarted, Stop, StopFailure, Interrupt, PreCompact, SessionEnd, SubagentStart and SubagentStop. UserPromptSubmit is synchronous and owns user turns; TurnStarted owns non-user turns. User TurnStarted duplicates are ignored, preventing fast-failure ordering races. Child turns and stops are correlated using the wire log, so a foreground child cannot prematurely close its parent. Kimi 2.3.2's subagent hook payloads omit IDs; the adapter resolves the native subagent ID and parent tool call from `subagent.spawned` records.

## Storage and capabilities

PreCompact is an explicit no-op with a diagnostic on stderr: Kimi appends compaction records while retaining the original wire log. Emitting Entire's Compaction event would reset byte cursors and double-count historical prompts/tokens. Normal capture continues from its current raw byte offset after compaction. SessionEnd is emitted by interactive `/exit`; prompt-mode process exit does not emit it in Kimi 2.3.2.

Native `$KIMI_CODE_HOME/sessions/.../wire.jsonl` and `state.json` are read-only. Byte-identical raw snapshots and restoration data live under ignored `.entire/metadata/kimi-code/`. Child snapshots use Entire's `<sessionId>/subagents/agent-<agentId>.jsonl` convention. Checkpoint compaction translates native prompts, assistant output, tool calls and results into Entire Transcript Format. Raw bytes remain available separately.

Entire 0.12.1's runtime condensation code supplies **line counts**, despite its external protocol describing byte offsets. Installation records this version's compatibility mode in `.entire/metadata/kimi-code/runtime.json`; the adapter translates those line positions to raw byte boundaries before parsing. Standalone protocol calls without that repository-local compatibility file retain byte-offset behavior. This prevents resumed checkpoints from counting earlier API calls again. Other Entire versions need their runtime cursor contract verified before claiming compatibility.

`ENTIRE_KIMI_OFFSET_UNIT=bytes` or `lines` explicitly selects the mode at installation. The protocol test wrapper selects bytes and avoids probing a version-manager shim under the suite's fake HOME. Normal local installation probes the actual Entire version.

Declared: hooks, transcript analysis, compact transcript, token calculation and subagent aggregation. Token accounting counts `usage.record` once per call and ignores duplicate `step.end` usage. No model-generated summary provider, daemon, ACP service or historical import.

Structured Write/Edit paths are attributed; arbitrary shell writes are not guessed. Git remains authoritative for committed files. `write-session` restores only Entire-owned data, never native Kimi runtime files. `kimi --session <id>` resumes the existing native session; restoring a deleted native Kimi session on another machine is not supported.

## Verify

`go test ./...` and `go vet ./...` cover captured payloads, raw bytes, Unicode/partial appends, token accounting, foreground child correlation, path confinement and config preservation. Pure tests set isolated homes. `.github/workflows/kimi-code.yml` runs these and pinned upstream protocol tests on Windows, macOS and Linux without Kimi credentials.

Run `python scripts/verify-protocol.py <binary>` after installing the pinned upstream `external-agents-tests` CLI. The wrapper resolves semantic fixture paths, creates an isolated Kimi home and runs from a neutral directory. The upstream launcher otherwise selects the current Go module and can produce a false green by running that module's tests instead of the compliance suite.

For a real authenticated local test: `py -3 scripts/smoke-kimi.py --output <evidence-directory>`. This uses existing authentication in place, creates a disposable repo, and proves Write → commit → checkpoint → raw and readable transcripts → token usage → real resumed Edit → second checkpoint. It never copies credentials. Agent/hooks must already be installed. The inherited Unix/tmux harness also registers `KIMI_E2E=1 E2E_AGENT=kimi-code`.

`py -3 scripts/smoke-failure.py` runs a real Kimi request against a loopback-only HTTP 400 provider in an isolated home, without authentication or production config changes. It asserts StopFailure closes the Entire turn. Interactive `/compact`, Escape, and `/exit` cover compaction, interruption and session end; `-p '/compact'` is ordinary model input and does not exercise compaction.

See `EVIDENCE.md` for executed results and limitations, and `AGENT.md` for native schema research.

## Config safety and removal

Installation appends a comment-delimited block, retaining unrelated config bytes, takes a timestamped backup beside config.toml, and writes atomically. Windows backups and temporary config files get an explicit current-user-only DACL before secret-bearing data is written. Installation uses a lock and rejects concurrent edits or damaged markers. `are-hooks-installed` checks every rule.

Run `entire-agent-kimi-code uninstall-hooks` to remove only the managed block. **This removes capture hooks globally**, affecting all repositories. Existing checkpoints remain. Remove the installed binary to stop discovery. Backups contain the original sensitive config; keep them private and outside Git.
