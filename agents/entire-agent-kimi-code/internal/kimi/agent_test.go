package kimi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MooseGooseConsulting/external-agents/agents/entire-agent-kimi-code/internal/protocol"
)

func setup(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	home := t.TempDir()
	t.Setenv("ENTIRE_REPO_ROOT", repo)
	t.Setenv("KIMI_CODE_HOME", home)
	return repo, home
}
func put(t *testing.T, path string, b []byte) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func nativeFixture(t *testing.T, repo, home string) string {
	t.Helper()
	dir := filepath.Join(home, "sessions", "wd_test", "session_test")
	path := filepath.Join(dir, "agents", "main", "wire.jsonl")
	b, e := os.ReadFile("../../testdata/transcripts/normal.jsonl")
	if e != nil {
		t.Fatal(e)
	}
	put(t, path, b)
	idx, _ := json.Marshal(map[string]string{"sessionId": "session_test", "sessionDir": dir, "workDir": repo})
	put(t, filepath.Join(home, "session_index.jsonl"), append(idx, '\n'))
	return path
}

func TestNativeCapturePreservesBytesAndCheckpointContent(t *testing.T) {
	repo, home := setup(t)
	native := nativeFixture(t, repo, home)
	before, _ := os.ReadFile(native)
	payload, _ := json.Marshal(map[string]any{"session_id": "session_test", "cwd": repo, "prompt": "test prompt"})
	ev, e := New().ParseHook("stop", payload)
	if e != nil {
		t.Fatal(e)
	}
	if ev.Type != 3 {
		t.Fatal(ev)
	}
	snapshot, _ := os.ReadFile(ev.SessionRef)
	if !bytes.Equal(before, snapshot) {
		t.Fatal("native snapshot changed bytes")
	}
	s, e := New().ReadSession(&protocol.HookInputJSON{SessionID: "session_test", SessionRef: ev.SessionRef})
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(before, s.NativeData) || len(s.ModifiedFiles) != 1 || s.ModifiedFiles[0] != "kimi-integration-probe.txt" {
		t.Fatalf("bad session files: %v", s.ModifiedFiles)
	}
	compact, e := New().CompactTranscript(ev.SessionRef)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := base64.StdEncoding.DecodeString(compact.Transcript)
	for _, want := range []string{"ENTIRE_KIMI_PROBE_20261025", "tool_use", "PROBE_DONE", "Wrote 26 bytes"} {
		if !bytes.Contains(b, []byte(want)) {
			t.Errorf("compact transcript missing %s", want)
		}
	}
	if e = New().WriteSession(s); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(native)
	if !bytes.Equal(before, after) {
		t.Fatal("write-session modified native Kimi log")
	}
}
func TestInstallerPreservesSecretAndForeignHooks(t *testing.T) {
	_, home := setup(t)
	path := filepath.Join(home, "config.toml")
	original := []byte("# preserve CRLF\r\napi_key = 'SENTINEL_PRIVATE_NOT_A_REAL_SECRET'\r\n[[hooks]]\r\nevent = 'Stop'\r\ncommand = 'echo unrelated'\r\n")
	put(t, path, original)
	a := New()
	n, e := a.InstallHooks(false, false)
	if e != nil || n != 10 {
		t.Fatalf("install: %d %v", n, e)
	}
	first, _ := os.ReadFile(path)
	if !bytes.HasPrefix(first, original) || !a.AreHooksInstalled() {
		t.Fatal("lost original config or rules")
	}
	n, e = a.InstallHooks(true, false)
	if e != nil || n != 0 {
		t.Fatalf("idempotence: %d %v", n, e)
	}
	second, _ := os.ReadFile(path)
	if !bytes.Equal(first, second) {
		t.Fatal("reinstall changed config")
	}
	damaged := bytes.Replace(first, []byte("turn-started"), []byte("wrong-command"), 1)
	put(t, path, damaged)
	if a.AreHooksInstalled() {
		t.Fatal("partial installation counted as complete")
	}
	if _, e = a.InstallHooks(false, false); e != nil {
		t.Fatal(e)
	}
	if e = a.UninstallHooks(); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, original) {
		t.Fatal("uninstall did not restore original bytes")
	}
	backups, _ := filepath.Glob(path + ".entire-agent-kimi-code.*.bak")
	if len(backups) == 0 {
		t.Fatal("missing recovery backup")
	}
}
func TestRejectForeignSessionAndPathEscapes(t *testing.T) {
	repo, home := setup(t)
	nativeFixture(t, repo, home)
	if _, e := nativeSession("session_test", t.TempDir()); e == nil {
		t.Fatal("accepted foreign repository")
	}
	for _, id := range []string{"../x", "..", "a/b", "a\\b", "a;echo hacked", "a\n"} {
		if validID(id) || New().ResolveSessionFile(repo, id) != "" {
			t.Errorf("unsafe id accepted %q", id)
		}
	}
	s := protocol.AgentSessionJSON{SessionID: "session_test", SessionRef: filepath.Join(t.TempDir(), "escape.jsonl")}
	if New().WriteSession(s) == nil {
		t.Fatal("write escaped Entire store")
	}
	payload, _ := json.Marshal(map[string]string{"session_id": "session_test", "cwd": t.TempDir()})
	if _, e := New().ParseHook("stop", payload); e == nil {
		t.Fatal("accepted mismatched hook cwd")
	}
	target := t.TempDir()
	dir, _ := New().GetSessionDir(repo)
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	link := filepath.Join(dir, "escape")
	if e := os.Symlink(target, link); e == nil {
		if managedPath(filepath.Join(link, "new.jsonl")) == nil {
			t.Fatal("write followed escaping symlink")
		}
	} else {
		t.Log("symlink creation unavailable:", e)
	}
}
func TestOffsetsPartialAppendsUnicodeAndUnknownRecords(t *testing.T) {
	first := []byte("{\"type\":\"turn.prompt\",\"input\":[{\"type\":\"text\",\"text\":\"世界 🙂\"}]}\n")
	tail := []byte("{\"type\":\"future-record\"")
	b := append(append([]byte{}, first...), tail...)
	rs, pos, e := records(b, 0)
	if e != nil || pos != len(first) || len(rs) != 1 {
		t.Fatalf("partial: %d %v %d", pos, e, len(rs))
	}
	b = append(b, []byte("}\nmalformed\n")...)
	rs, end, e := records(b, pos)
	if e != nil || len(rs) != 1 || end != len(b) {
		t.Fatalf("append: %d %v", end, e)
	}
	chunks, e := New().ChunkTranscript(b, 7)
	if e != nil {
		t.Fatal(e)
	}
	joined, _ := New().ReassembleTranscript(chunks)
	if !bytes.Equal(b, joined) {
		t.Fatal("chunk roundtrip corrupted UTF-8 bytes")
	}
	if _, _, e = records(b, -1); e == nil {
		t.Fatal("accepted negative offset")
	}
	if _, e = New().ChunkTranscript(b, 0); e == nil {
		t.Fatal("accepted zero chunk size")
	}
}
func TestTokensCountUsageRecordsOnce(t *testing.T) {
	b, e := os.ReadFile("../../testdata/transcripts/normal.jsonl")
	if e != nil {
		t.Fatal(e)
	}
	u, e := tokenUsage(b, 0)
	if e != nil {
		t.Fatal(e)
	}
	if u.Input != 3733 || u.Output != 170 || u.Read != 61952 || u.Calls != 2 {
		t.Fatalf("double-counted or omitted usage: %+v", u)
	}
	_, repo := setup(t)
	_ = repo
	var out bytes.Buffer
	if e = New().HandleAggregate("calculate-total-tokens", nil, bytes.NewReader(b), &out); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(out.String(), "subagent_tokens") {
		t.Fatal("invented subagent usage without subagents")
	}
}
func TestLifecycleEdgesMapAndDuplicateEventsNoOp(t *testing.T) {
	repo, home := setup(t)
	nativeFixture(t, repo, home)
	p, _ := json.Marshal(map[string]any{"session_id": "session_test", "cwd": repo, "prompt": []any{map[string]string{"type": "text", "text": "hello"}}, "reason": "test", "subagent_id": "0"})
	for _, h := range hookRules {
		ev, e := New().ParseHook(h.Name, p)
		if h.Type == 6 || h.Type == 7 {
			if ev != nil || e != nil {
				t.Fatal("uncorrelated subagent must be no-op", ev, e)
			}
			continue
		}
		if e != nil || ev == nil || ev.Type != h.Type {
			t.Fatalf("%s: %v %v", h.Name, ev, e)
		}
	}
	for _, name := range []string{"post-compact", "unknown"} {
		ev, e := New().ParseHook(name, p)
		if ev != nil || e != nil {
			t.Fatalf("duplicate %s wasn't no-op", name)
		}
	}
	userTurn, _ := json.Marshal(map[string]any{"session_id": "session_test", "cwd": repo, "origin_kind": "user", "prompt": "duplicate"})
	if ev, e := New().ParseHook("turn-started", userTurn); ev != nil || e != nil {
		t.Fatal("duplicate user turn-started", ev, e)
	}
}

func TestSubagentCorrelationAndChildStops(t *testing.T) {
	repo, home := setup(t)
	path := nativeFixture(t, repo, home)
	main, e := os.ReadFile("../../testdata/transcripts/subagent-main.jsonl")
	if e != nil {
		t.Fatal(e)
	}
	// First reproduce the hook while a foreground child is running.
	lines := bytes.SplitAfter(main, []byte{'\n'})
	var active bytes.Buffer
	for _, line := range lines {
		if bytes.Contains(line, []byte(`"type":"subagent.completed"`)) || bytes.Contains(line, []byte(`"type": "subagent.completed"`)) {
			break
		}
		active.Write(line)
	}
	put(t, path, active.Bytes())
	p, _ := json.Marshal(map[string]any{"session_id": "session_test", "cwd": repo, "agent_name": "coder"})
	ev, e := New().ParseHook("subagent-start", p)
	if e != nil || ev == nil || ev.SubagentID != "agent-0" || ev.ToolUseID == "" {
		t.Fatalf("start correlation: %v %v", ev, e)
	}
	ev, e = New().ParseHook("stop", p)
	if e != nil || ev != nil {
		t.Fatal("child Stop ended parent turn", ev, e)
	}
	p2, _ := json.Marshal(map[string]any{"session_id": "session_test", "cwd": repo, "origin_name": "subagent"})
	ev, e = New().ParseHook("turn-started", p2)
	if e != nil || ev != nil {
		t.Fatal("child TurnStarted reopened parent", ev, e)
	}
	put(t, path, main)
	child, e := os.ReadFile("../../testdata/transcripts/subagent-child.jsonl")
	if e != nil {
		t.Fatal(e)
	}
	put(t, filepath.Join(filepath.Dir(filepath.Dir(path)), "agent-0", "wire.jsonl"), child)
	ev, e = New().ParseHook("subagent-stop", p)
	if e != nil || ev == nil || ev.SubagentID != "agent-0" {
		t.Fatalf("stop correlation: %v %v", ev, e)
	}
	files, _, e := New().ExtractModifiedFiles(ev.SessionRef, 0)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, f := range files {
		if strings.HasSuffix(f, "subagent-proof.txt") {
			found = true
		}
	}
	if !found {
		t.Fatal("child Write missing from aggregate", files)
	}
	dir := filepath.Join(filepath.Dir(ev.SessionRef), "session_test", "subagents")
	var out bytes.Buffer
	if e = New().HandleAggregate("calculate-total-tokens", []string{"--subagents-dir", dir}, bytes.NewReader(main), &out); e != nil {
		t.Fatal(e)
	}
	var usage tokens
	if e = json.Unmarshal(out.Bytes(), &usage); e != nil {
		t.Fatal(e)
	}
	if usage.Sub == nil || usage.Sub.Calls == 0 {
		t.Fatal("child usage missing", out.String())
	}
	ev, e = New().ParseHook("stop", p)
	if e != nil || ev == nil || ev.Type != 3 {
		t.Fatal("main Stop missing after child completion", ev, e)
	}
}
