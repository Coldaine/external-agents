// A deterministic executable used only by the Kimi CLI process tests.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Println("Entire CLI 0.12.1")
		return
	}
	var name string
	switch {
	case len(os.Args) == 4 && os.Args[1] == "hooks" && os.Args[2] == "kimi-code":
		name = os.Args[3]
	case len(os.Args) == 3 && os.Args[1] == "dispatch-hook":
		// Negative control: deliberately bypass the adapter's session lock.
		name = os.Args[2]
	default:
		fmt.Fprintln(os.Stderr, "unexpected arguments", os.Args[1:])
		os.Exit(2)
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		panic(err)
	}
	dir := os.Getenv("FAKE_ENTIRE_DIR")
	if err := os.WriteFile(filepath.Join(dir, name+".json"), data, 0600); err != nil {
		panic(err)
	}
	if os.Getenv("FAKE_ENTIRE_BLOCK") == name {
		deadline := time.Now().Add(15 * time.Second)
		for {
			if _, err := os.Stat(filepath.Join(dir, "release")); err == nil {
				break
			}
			if time.Now().After(deadline) {
				fmt.Fprintln(os.Stderr, "fake Entire timed out")
				os.Exit(2)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	if os.Getenv("FAKE_ENTIRE_FAIL") == name {
		fmt.Fprintln(os.Stderr, "controlled Entire failure")
		os.Exit(17)
	}
	fmt.Fprint(os.Stdout, "controlled Entire success")
}
