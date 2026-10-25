# Kimi Code — local Entire integration

Owner: MooseGooseConsulting/external-agents. Delivery targets local Windows; no upstream acceptance gate.

## Research

Verified 2026-10-25 with Kimi 2.3.2 and Entire 0.12.1. Protocol source: entireio/cli tag v0.12.1, docs/architecture/external-agent-protocol.md and cmd/entire/cli/agent/external/{types,external}.go.
Native hooks: global KIMI_CODE_HOME/config.toml [[hooks]], JSON stdin, session_id and cwd. Prompt mode: kimi -p PROMPT (cannot combine --auto). Resume: kimi --session ID.
Captured real SessionStart, UserPromptSubmit, TurnStarted and Stop in a disposable repository, using existing authentication without copying credentials. Original config restored byte-for-byte.
UserPromptSubmit.prompt is a content-parts array; TurnStarted.prompt is text. Use TurnStarted as the single turn boundary, including task/system turns; no duplicate UserPromptSubmit event.
Stop is a valid native hook event, distinct from StopFailure, confirmed by the official Kimi Code hook documentation; both mark the end of a turn (Entire type 3). StopFailure additionally carries `error_type`/`error_message` (real capture: `APITimeoutError`).
Verified 2026-10-25 with the ACP lifecycle capture script (`scripts/capture-kimi-lifecycle.py`) in a disposable repository: PreCompact, PostCompact, StopFailure and Interrupt captured real payloads, plus SessionEnd.
SessionEnd fires on a graceful session close (ACP `session/close` followed by closing the ACP stdio session; an interactive TUI close is the other graceful path but was not captured — no PTY on this Windows host). A print-mode `kimi -p` process exit does not emit SessionEnd; do not infer it from process exit.
StopFailure requires a `--model` alias that fails mid-run, supplied by the environment (`KIMI_LIFECYCLE_FAILURE_MODEL`); the capture script skips that scenario with a clear message when unset and never writes the model value to output.

## Native data

session_index.jsonl has sessionId/sessionDir/workDir. Main transcript: sessions/<workDirKey>/<sessionId>/agents/main/wire.jsonl; agent-* siblings contain subagents. Never modify native records.
Verified actual prompt, assistant output and Write tool execution in wire.jsonl. turn.prompt.input holds text parts. context.append_loop_event.event has tool.call (name,args,toolCallId), tool.result and content.part. usage.record has per-call usage inputOther/output/inputCacheRead/inputCacheCreation; ignore duplicated step.end usage.

## Mapping

SessionStart→1; TurnStarted→2; Stop/StopFailure/Interrupt→3; PreCompact→4; SessionEnd→5; SubagentStart→6; SubagentStop→7. UserPromptSubmit and PostCompact are diagnostic/no-op to avoid duplicates.
Session helpers resolve native paths from the index with repository/containment validation. Raw reads/chunks preserve bytes. write-session stores restoration data only in Entire-owned storage; it does not rewrite Kimi sessions. Compact-transcript converts wire events to Entire format for checkpoint readability. Transcript analysis extracts structured Write/Edit paths and user prompts; arbitrary shell writes are not inferred.

## Test prerequisites and limits

Real tests require kimi and entire on PATH, existing authenticated Kimi config and a disposable Git repository. Global hook installation affects all repositories, while Entire dispatch remains repository scoped. Pure protocol tests use isolated KIMI_CODE_HOME. `scripts/smoke-kimi.py` is the native Windows smoke script; `scripts/capture-kimi-lifecycle.py` is the native ACP lifecycle capture (temporary global hook append installed and restored through a guarded atomic `os.replace`; the `finally` restore applies byte-for-byte only when the config is unchanged since install, concurrent drift is reported and never clobbered; requires existing authentication and never prints configuration).
Captured scenarios on record under `testdata/`: compaction (`lifecycle-compaction`), failure (`lifecycle-failure`, StopFailure with `APITimeoutError`), interrupt (`lifecycle-interrupt`) and graceful close (`lifecycle-SessionEnd`). Focused `go test` coverage parses these payloads and maps all nine hook rules without credentials. Interactive TUI close and historical imports remain uncaptured/unimplemented; no AI summary provider, background daemon or historical import.
