# Local Kimi integration verification

Observed 2026-10-25 on Windows amd64 with Kimi 2.3.2, Entire 0.12.1, and Go 1.27.3. Owning fork: MooseGooseConsulting/external-agents. Runtime hardening branch: `codex/kimi-local`; integration target: `agent/kimi-code`.

## Executed proof

| Requirement | Evidence | Result |
|---|---|---|
| Fork/source ownership | Organization fork created; adapter source committed and pushed | Pass |
| Local discovery | Installed `entire-agent-kimi-code.exe` on absolute PATH; `entire agent list` in coldaine-configurations lists kimi-code | Pass |
| Real capture | Fresh prompt creates proof.txt with Write; commit checkpoint `01M5T6D1P3J1A6HGX8D7WM0140` contains the correct Kimi session, file, prompt, actual tool call and assistant response | Pass |
| Resume | Native resumed Edit creates checkpoint `01M5T6DMHBK9S4VKF27F9QRD1E`; `entire session resume kimi-smoke` prints the correct Kimi command. The resumed checkpoint has exactly 3 newly appended API calls, matching the native log delta | Pass |
| Raw storage | Smoke script reads checkpoint transcript and chunks/reassembles it byte-for-byte; recorded SHA-256 `e001462c70aa916ad5323d0c08bfca8bd36a6d7ec5eb4cddaa825fac4497b0dd` | Pass |
| Readable transcript | `checkpoint explain --full` displays the user prompt, Write tool and assistant response through compact-transcript | Pass |
| Token accounting | Checkpoint has real per-call usage; unit fixtures count usage.record exactly once and ignore duplicate step.end usage | Pass |
| Foreground subagent | Checkpoint `01M5T53VAYGD07YCPY0G7Z5HJ9` has child-created subagent-proof-v2.txt, correlated tool ID, agent-0 task, child tokens and `transcript_stored: true`; one parent TurnStart/TurnEnd | Pass |
| Failure | `scripts/smoke-failure.py` uses an isolated home and local HTTP 400 server; real StopFailure captured and Entire moves active → idle, without claiming a file checkpoint when no file changed | Pass |
| Interruption | Interactive Escape interrupts an active model turn; Entire receives TurnEnd and returns active → idle; native cancellation records remain available | Pass |
| Session end | Interactive `/exit` emits SessionEnd and moves idle → ended; headless prompt-mode process exit does not emit SessionEnd | Pass with documented mode difference |
| Compaction | Interactive `/compact` succeeds; wire.jsonl retains prior records and appends compaction records. PreCompact is a tested explicit no-op for Entire's offset reset | Pass with documented no-op |
| Fork | Native `kimi fork --yes` creates a child session with state.forkedFrom; resuming the fork records its own session and completes a real prompt | Pass |
| Actual repository | Fresh read-only Kimi session `session_9ab215e0-87d7-4628-bde0-9b238add16c7` appears in Entire status in coldaine-configurations with model, prompt and token usage | Pass |
| Pure checks | Adapter `go test -count=1 ./...`, `go vet ./...`, repository golangci-lint configuration (0 issues with Go 1.26), and pinned upstream compliance suite with semantic transcript fixture | Pass |
| Cross-platform checks | [CI run 40222597105](https://github.com/MooseGooseConsulting/external-agents/actions/runs/40222597105), code revision bb72e22: Windows, macOS and Ubuntu unit/vet/build and protocol tests | Pass |

## Corrections driven by failures

1. The shared compliance launcher selects any CWD containing go.mod. An initial invocation therefore ran the adapter module instead of compliance tests. `scripts/verify-protocol.py` resolves fixture paths and uses a neutral directory and fresh isolated Kimi home. Subsequent runs executed the actual mandatory/hooks/transcript/token/subagent packages, including the semantic fixture.
2. Kimi's TurnStarted event is asynchronous. A loopback HTTP 400 provider demonstrated TurnEnd overtaking TurnStart. Synchronous UserPromptSubmit now owns user turns; user-origin TurnStarted duplicates are ignored. This requires ten global rules rather than nine.
3. Foreground children emit TurnStarted/Stop with the parent session ID. The adapter ignores the child start and correlates native subagent lifecycle records so the child Stop does not prematurely close its parent.
4. Kimi compaction is append-only. Emitting Entire's Compaction event reset byte cursors and replayed historical usage in a live test. PreCompact now returns JSON null and documents why; byte cursors continue normally. A regression proves newly appended usage is counted without replaying prior records.
5. Unix returns ENOTDIR when a generic test-transcript.json file is treated as the parent of a child directory. Child scans now apply only to JSONL session snapshots. Windows, macOS and Ubuntu checks pass.
6. A stronger resumed-checkpoint audit found that Entire 0.12.1 stores line counts in checkpoint/token cursors while the external protocol documents bytes. The adapter now records an explicit repository-local compatibility mode during installation and translates live line positions to byte boundaries. Standalone protocol calls keep byte semantics. The smoke test asserts the second checkpoint contains exactly the newly appended native API calls, rather than accepting nonzero token counters as proof.

## Limits and preservation

- Live authenticated provider tests were performed on Windows. macOS/Linux CI tests are credential-free unit and protocol checks, not real Kimi provider sessions.
- The inherited tmux lifecycle harness was cross-compiled for Linux; it was not executed on this Windows host. Native smoke scripts supply the real local lifecycle evidence above.
- Foreground subagent behavior is live-tested. Arbitrary simultaneous/background subagent combinations are not exhaustively verified.
- Structured Write/Edit paths are extracted; arbitrary shell writes are not guessed.
- Restoring deleted native Kimi sessions from a checkpoint onto another machine is not supported. write-session restores Entire-owned data; resume uses the existing native Kimi session.
- Global config edits preserve unrelated bytes and hooks, back up sensitive configuration outside Git, and restrict Windows backups/temp files to the current user. Tests use sentinel values; real credentials are never fixtures. Live authenticated probes use existing authentication in place, without copying credentials.
- Work continues independently of the parallel Kimi swarm checkout. [PR #1](https://github.com/MooseGooseConsulting/external-agents/pull/1) targets the organization fork's agent/kimi-code branch; no upstream issue, PR, or acceptance is required.
