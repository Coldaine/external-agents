# Kimi Code for Entire

Locally maintained in [MooseGooseConsulting/external-agents](https://github.com/MooseGooseConsulting/external-agents), branch `agent/kimi-code`. Tested with Kimi 2.3.2 and Entire 0.12.1 on Windows.

## Install

From this directory, `go build -o entire-agent-kimi-code.exe ./cmd/entire-agent-kimi-code` on Windows, or omit `.exe` on Linux/macOS. Copy the binary to an **absolute** directory already on PATH (for example `$HOME/.local/bin`). Keep source in this fork so the binary can be rebuilt.

In each repository, run `entire enable --agent kimi-code --local --skip-push-sessions` (or `entire agent add kimi-code` when Entire is already enabled). Verify the untracked `.entire/settings.local.json` enables `external_agents`, then run `entire agent list`. Hooks are global but dispatch only in Entire-enabled repositories. Start a fresh Kimi session.

Nine managed rules use SessionStart, TurnStarted, Stop, StopFailure, Interrupt, PreCompact, SessionEnd, SubagentStart and SubagentStop. TurnStarted handles user and non-user turns. Child turns and stops are correlated using the wire log, so a foreground child cannot prematurely close its parent. Kimi 2.3.2's subagent hook payloads omit IDs; the adapter resolves the native subagent ID and parent tool call from `subagent.spawned` records.

## Storage and capabilities

Native `$KIMI_CODE_HOME/sessions/.../wire.jsonl` and `state.json` are read-only. Byte-identical raw snapshots and restoration data live under ignored `.entire/metadata/kimi-code/`. Child snapshots use Entire's `<sessionId>/subagents/agent-<agentId>.jsonl` convention. Checkpoint compaction translates native prompts, assistant output, tool calls and results into Entire Transcript Format. Raw bytes remain available separately.

Declared: hooks, transcript analysis, compact transcript, token calculation and subagent aggregation. Token accounting counts `usage.record` once per call and ignores duplicate `step.end` usage. No model-generated summary provider, daemon, ACP service or historical import.

Structured Write/Edit paths are attributed; arbitrary shell writes are not guessed. Git remains authoritative for committed files. `write-session` restores only Entire-owned data, never native Kimi runtime files. `kimi --session <id>` resumes the existing native session; restoring a deleted native Kimi session on another machine is not supported.

## Verify

`go test ./...` and `go vet ./...` cover captured payloads, raw bytes, Unicode/partial appends, token accounting, foreground child correlation, path confinement and config preservation. Pure tests set isolated homes. `.github/workflows/kimi-code.yml` runs these and pinned upstream protocol tests on Windows, macOS and Linux without Kimi credentials.

Run the shared compliance CLI **from a directory without go.mod**, with a fresh isolated `KIMI_CODE_HOME` and `--fixtures <absolute-path>/testdata/compliance.json`. Its launcher otherwise selects the current Go module and can produce a false green by running that module's tests instead of the compliance suite.

For a real authenticated local test: `py -3 scripts/smoke-kimi.py --output <evidence-directory>`. This uses existing authentication in place, creates a disposable repo, and proves Write → commit → checkpoint → raw and readable transcripts → token usage → real resumed Edit → second checkpoint. It never copies credentials. Agent/hooks must already be installed. The inherited Unix/tmux harness also registers `KIMI_E2E=1 E2E_AGENT=kimi-code`.

See `EVIDENCE.md` for executed results and limitations, and `AGENT.md` for native schema research.

## Config safety and removal

Installation appends a comment-delimited block, retaining unrelated config bytes, takes a timestamped backup beside config.toml, and writes atomically. Windows backups and temporary config files get an explicit current-user-only DACL before secret-bearing data is written. Installation uses a lock and rejects concurrent edits or damaged markers. `are-hooks-installed` checks every rule.

Run `entire-agent-kimi-code uninstall-hooks` to remove only the managed block. **This removes capture hooks globally**, affecting all repositories. Existing checkpoints remain. Remove the installed binary to stop discovery. Backups contain the original sensitive config; keep them private and outside Git.
