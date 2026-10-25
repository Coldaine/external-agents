package kimi

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

type subagent struct {
	ID          string `json:"subagentId"`
	Name        string `json:"subagentName"`
	ToolID      string `json:"parentToolCallId"`
	Parent      string `json:"parentAgentId"`
	Description string `json:"description"`
	Background  bool   `json:"runInBackground"`
	Done        bool
	Type        string `json:"type"`
}

func subagentRecords(ref string) []subagent {
	b, _ := os.ReadFile(ref)
	out := []subagent{}
	indices := map[string]int{}
	for _, line := range bytes.Split(b, []byte{'\n'}) {
		var s subagent
		if json.Unmarshal(line, &s) != nil {
			continue
		}
		switch s.Type {
		case "subagent.spawned":
			indices[s.ID] = len(out)
			out = append(out, s)
		case "subagent.completed", "subagent.cancelled", "subagent.failed":
			if i, ok := indices[s.ID]; ok {
				out[i].Done = true
			}
		}
	}
	return out
}
func hasForegroundSubagent(subs []subagent) bool {
	for _, s := range subs {
		if !s.Done && !s.Background && s.Parent == "main" {
			return true
		}
	}
	return false
}
func findSubagent(subs []subagent, name string, done bool) (subagent, bool) {
	for i := len(subs) - 1; i >= 0; i-- {
		s := subs[i]
		if s.Name == name && s.Done == done && s.Parent == "main" {
			return s, true
		}
	}
	return subagent{}, false
}
func copySubagents(native, ref, id string) error {
	sourceDir := filepath.Dir(filepath.Dir(native))
	root, _ := canonical(sourceDir)
	entries, e := os.ReadDir(sourceDir)
	if e != nil {
		return e
	}
	for _, ent := range entries {
		if !ent.IsDir() || len(ent.Name()) < 6 || ent.Name()[:6] != "agent-" {
			continue
		}
		path := filepath.Join(sourceDir, ent.Name(), "wire.jsonl")
		actual, e := canonical(path)
		if e != nil || !within(root, actual) {
			continue
		}
		b, e := os.ReadFile(path)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		dest := filepath.Join(filepath.Dir(ref), id, "subagents", "agent-"+ent.Name()+".jsonl")
		if e = managedPath(dest); e != nil {
			return e
		}
		if e = atomicWrite(dest, completePrefix(b)); e != nil {
			return e
		}
	}
	return nil
}
func childTranscripts(dir string) ([][]byte, error) {
	entries, e := os.ReadDir(dir)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	root, _ := canonical(dir)
	out := [][]byte{}
	for _, ent := range entries {
		if ent.IsDir() || filepath.Ext(ent.Name()) != ".jsonl" {
			continue
		}
		p := filepath.Join(dir, ent.Name())
		actual, e := canonical(p)
		if e != nil || !within(root, actual) {
			continue
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return nil, e
		}
		out = append(out, b)
	}
	return out, nil
}
