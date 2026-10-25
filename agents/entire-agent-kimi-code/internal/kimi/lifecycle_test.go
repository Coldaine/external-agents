package kimi

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestCapturedLifecycleEdgesPreserveNativeLog(t *testing.T) {
	for _, tc := range []struct {
		event, hook, transcript, metadata, value string
		kind                                     int
	}{
		{"Interrupt", "interrupt", "interrupt", "reason", "cancelled", 3},
		{"StopFailure", "stop-failure", "failure", "error_type", "APITimeoutError", 3},
		{"SessionEnd", "session-end", "interrupt", "reason", "exit", 5},
		{"PreCompact", "pre-compact", "compaction", "", "", 0},
		{"PostCompact", "post-compact", "compaction", "", "", 0},
	} {
		t.Run(tc.event, func(t *testing.T) {
			repo, home := setup(t)
			path := nativeFixture(t, repo, home)
			wire, err := os.ReadFile("../../testdata/transcripts/lifecycle-" + tc.transcript + ".jsonl")
			if err != nil {
				t.Fatal(err)
			}
			put(t, path, wire)
			b, err := os.ReadFile("../../testdata/hooks/lifecycle-" + tc.event + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]any
			if err = json.Unmarshal(b, &payload); err != nil {
				t.Fatal(err)
			}
			payload["session_id"], payload["cwd"] = "session_test", repo
			b, err = json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			ev, err := New().ParseHook(tc.hook, b)
			if err != nil {
				t.Fatal(err)
			}
			if tc.kind == 0 {
				if ev != nil {
					t.Fatalf("append-only compaction reset offsets: %+v", ev)
				}
			} else {
				if ev == nil || ev.Type != tc.kind || ev.Metadata[tc.metadata] != tc.value {
					t.Fatalf("lost captured lifecycle outcome: %+v", ev)
				}
				stored, err := os.ReadFile(ev.SessionRef)
				if err != nil || !bytes.Equal(stored, wire) {
					t.Fatalf("snapshot changed native bytes: %v", err)
				}
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(after, wire) {
				t.Fatalf("hook modified Kimi's log: %v", err)
			}
		})
	}
}
