"""Credential-free, cross-platform tests of the built Kimi adapter process."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
import unittest

MODULE = Path(__file__).resolve().parent.parent
FIXTURES = MODULE / "testdata"
SUFFIX = ".exe" if os.name == "nt" else ""


class CliTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.build_dir = tempfile.TemporaryDirectory(prefix="kimi-cli-build-")
        root = Path(cls.build_dir.name)
        cls.binary = root / ("entire-agent-kimi-code" + SUFFIX)
        cls.fake = root / ("entire" + SUFFIX)
        for output, source in ((cls.binary, "./cmd/entire-agent-kimi-code"),
                               (cls.fake, "./scripts/testdata/fake_entire.go")):
            subprocess.run(["go", "build", "-o", str(output), source], cwd=MODULE,
                           check=True, timeout=120)

    @classmethod
    def tearDownClass(cls):
        cls.build_dir.cleanup()

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="kimi-cli-test-")
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name)
        self.repo, self.home, self.fake_dir = (root / n for n in ("repo", "home", "fake"))
        for path in (self.repo, self.home, self.fake_dir):
            path.mkdir()
        self.env = dict(os.environ, ENTIRE_REPO_ROOT=str(self.repo),
                        KIMI_CODE_HOME=str(self.home), HOME=str(self.home),
                        USERPROFILE=str(self.home), ENTIRE_KIMI_OFFSET_UNIT="bytes",
                        GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull,
                        FAKE_ENTIRE_DIR=str(self.fake_dir),
                        PATH=self.build_dir.name + os.pathsep + os.environ.get("PATH", ""))
        self.env.pop("FAKE_ENTIRE_FAIL", None)
        self.env.pop("FAKE_ENTIRE_BLOCK", None)
        subprocess.run(["git", "init", "-q", str(self.repo)], env=self.env,
                       check=True, capture_output=True, timeout=30)

    def cli(self, *args, data=b"", success=True, env=None):
        result = subprocess.run([str(self.binary), *map(str, args)], input=data,
                                cwd=self.repo, env=env or self.env,
                                capture_output=True, timeout=30)
        if success:
            self.assertEqual(result.returncode, 0, result.stderr.decode(errors="replace"))
        else:
            self.assertNotEqual(result.returncode, 0, "command unexpectedly succeeded")
        return result

    def payload(self, fixture, **overrides):
        data = json.loads((FIXTURES / "hooks" / (fixture + ".json")).read_text(encoding="utf-8"))
        data.update(cwd=str(self.repo), **overrides)
        return json.dumps(data).encode()

    def setup_native(self, content):
        session = self.home / "sessions" / "wd_test" / "session_test"
        native = session / "agents" / "main" / "wire.jsonl"
        native.parent.mkdir(parents=True)
        native.write_bytes(content)
        index = {"sessionId": "session_test", "sessionDir": str(session), "workDir": str(self.repo)}
        (self.home / "session_index.jsonl").write_text(json.dumps(index) + "\n", encoding="utf-8")
        return native

    def stop_process(self, process):
        if process.poll() is None:
            (self.fake_dir / "release").touch()
            try:
                process.wait(timeout=3)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=10)
        for stream in (process.stdin, process.stdout, process.stderr):
            if stream is not None:
                stream.close()

    def wait_for(self, path, process=None):
        deadline = time.monotonic() + 10
        while not path.exists():
            if process is not None and process.poll() is not None:
                self.fail(f"process exited before creating {path}: {process.communicate()}")
            if time.monotonic() > deadline:
                self.fail(f"timed out waiting for {path}")
            time.sleep(.01)

    def assert_waiting_for_lock(self, process, entered):
        self.assertIsNone(process.poll(), "second hook exited before the first released the lock")
        self.assertFalse(entered.exists(), "second hook entered Entire before the first released the lock")

    def test_dispatch_serializes_processes_and_propagates_failure(self):
        (self.repo / ".entire").mkdir()
        env = dict(self.env, FAKE_ENTIRE_BLOCK="turn-started")
        first_payload = self.payload("TurnStarted")
        second_payload = self.payload("StopFailure", session_id="session_probe")
        first = subprocess.Popen([str(self.binary), "dispatch-hook", "turn-started"],
                                 cwd=self.repo, env=env, stdin=subprocess.PIPE,
                                 stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.addCleanup(self.stop_process, first)
        first.stdin.write(first_payload)
        first.stdin.close()
        first.stdin = None
        self.wait_for(self.fake_dir / "turn-started.json", first)
        second = subprocess.Popen([str(self.binary), "dispatch-hook", "stop-failure"],
                                  cwd=self.repo, env=env, stdin=subprocess.PIPE,
                                  stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.addCleanup(self.stop_process, second)
        second.stdin.write(second_payload)
        second.stdin.close()
        second.stdin = None
        # The first child is held in Entire. A second child must not enter it.
        time.sleep(.3)
        self.assert_waiting_for_lock(second, self.fake_dir / "stop-failure.json")
        other_payload = self.payload("SessionStart", session_id="session_other")
        self.cli("dispatch-hook", "session-end", data=other_payload)
        self.assertEqual((self.fake_dir / "session-end.json").read_bytes(), other_payload)
        self.assertFalse((self.fake_dir / "stop-failure.json").exists())
        (self.fake_dir / "release").touch()
        first_out, first_err = first.communicate(timeout=10)
        second_out, second_err = second.communicate(timeout=10)
        self.assertEqual(first.returncode, 0, first_err)
        self.assertEqual(second.returncode, 0, second_err)
        self.assertEqual(first_out, b"controlled Entire success")
        self.assertEqual(second_out, b"controlled Entire success")
        self.assertEqual((self.fake_dir / "turn-started.json").read_bytes(), first_payload)
        self.assertEqual((self.fake_dir / "stop-failure.json").read_bytes(), second_payload)
        lock = self.repo / ".entire" / "metadata" / "kimi-code" / "session_probe.hook-lock"
        self.assertFalse(lock.exists())
        failed = self.cli("dispatch-hook", "stop-failure", data=second_payload, success=False,
                          env=dict(self.env, FAKE_ENTIRE_FAIL="stop-failure"))
        self.assertIn(b"controlled Entire failure", failed.stderr)
        self.assertFalse(lock.exists(), "failed Entire process left lock behind")
        self.cli("dispatch-hook", "stop-failure", data=second_payload)

    def test_serialization_check_rejects_nonserializing_dispatch(self):
        env = dict(self.env, FAKE_ENTIRE_BLOCK="turn-started")
        first = subprocess.Popen([str(self.fake), "dispatch-hook", "turn-started"],
                                 cwd=self.repo, env=env, stdin=subprocess.PIPE,
                                 stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.addCleanup(self.stop_process, first)
        first.stdin.write(self.payload("TurnStarted"))
        first.stdin.close()
        first.stdin = None
        self.wait_for(self.fake_dir / "turn-started.json", first)
        second = subprocess.Popen([str(self.fake), "dispatch-hook", "stop-failure"],
                                  cwd=self.repo, env=env, stdin=subprocess.PIPE,
                                  stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.addCleanup(self.stop_process, second)
        second.stdin.write(self.payload("StopFailure", session_id="session_probe"))
        second.stdin.close()
        second.stdin = None
        entered = self.fake_dir / "stop-failure.json"
        self.wait_for(entered)
        with self.assertRaises(AssertionError):
            self.assert_waiting_for_lock(second, entered)

    def test_dispatch_rejects_invalid_scope_and_is_noop_when_disabled(self):
        payload = self.payload("TurnStarted")
        self.cli("dispatch-hook", "turn-started", data=payload)
        self.assertFalse((self.repo / ".entire").exists())
        self.cli("dispatch-hook", "turn-started", data=self.payload("TurnStarted", session_id="../escape"), success=False)
        foreign = json.loads(payload)
        foreign["cwd"] = str(self.home)
        self.cli("dispatch-hook", "turn-started", data=json.dumps(foreign).encode(), success=False)
        self.assertEqual(list(self.fake_dir.iterdir()), [])

    def test_session_writes_cannot_escape_the_repository(self):
        metadata = self.repo / ".entire" / "metadata" / "kimi-code"
        resolved = json.loads(self.cli("resolve-session-file", "--session-dir", metadata,
                                       "--session-id", "../escape").stdout)
        self.assertEqual(resolved["session_file"], "")
        outside = self.home / "escape.jsonl"
        session = {"session_id": "session_test", "session_ref": str(outside), "native_data": "e30="}
        result = self.cli("write-session", data=json.dumps(session).encode(), success=False)
        self.assertIn(b"outside Entire-owned store", result.stderr)
        self.assertFalse(outside.exists())

    def test_snapshot_partial_append_and_resume_cursor(self):
        first = b'{"type":"usage.record","usage":{"inputOther":10,"output":1}}\n'
        duplicate = b'{"type":"step.end","usage":{"inputOther":10,"output":1}}\n'
        last = b'{"type":"usage.record","usage":{"inputOther":20,"output":2}}\n'
        native = self.setup_native(first + duplicate + last[:-5])
        payload = self.payload("TurnStarted", session_id="session_test")
        event = json.loads(self.cli("parse-hook", "--hook", "turn-started", data=payload).stdout)
        snapshot = Path(event["session_ref"])
        self.assertEqual(snapshot.read_bytes(), first + duplicate)
        pos = json.loads(self.cli("get-transcript-position", "--path", snapshot).stdout)["position"]
        self.assertEqual(pos, len(first + duplicate))
        native.write_bytes(first + duplicate + last)
        self.cli("parse-hook", "--hook", "turn-started", data=payload)
        self.assertEqual(snapshot.read_bytes(), first + duplicate + last)
        total = json.loads(self.cli("calculate-tokens", data=snapshot.read_bytes()).stdout)
        resumed = json.loads(self.cli("calculate-tokens", "--offset", pos, data=snapshot.read_bytes()).stdout)
        self.assertEqual((total["input_tokens"], total["api_call_count"]), (30, 2))
        self.assertEqual((resumed["input_tokens"], resumed["output_tokens"], resumed["api_call_count"]), (20, 2, 1))
        runtime = self.repo / ".entire" / "metadata" / "kimi-code" / "runtime.json"
        runtime.write_text('{"entire_version":"Entire CLI 0.12.1","offset_unit":"lines"}', encoding="utf-8")
        line_pos = json.loads(self.cli("get-transcript-position", "--path", snapshot).stdout)["position"]
        self.assertEqual(line_pos, 3)
        resumed = json.loads(self.cli("calculate-tokens", "--offset", 2, data=snapshot.read_bytes()).stdout)
        self.assertEqual((resumed["input_tokens"], resumed["api_call_count"]), (20, 1))

    def test_correlates_concurrent_children_and_rejects_ambiguous_identity(self):
        native = self.setup_native((FIXTURES / "transcripts" / "background-main.jsonl").read_bytes())
        expectations = [("CHILD_1_DONE", "agent-1", "tool_HHe1V8mbJ16XWcDU6QYLbdWV"),
                        ("Slept for 3 seconds, created `child-0.txt` containing exactly `CHILD_0_OK`, and did not use git.\n\nCHILD_0_DONE", "agent-0", "tool_ELqIq6WE0320Chnr0WGRxeNw")]
        for response, child, tool in expectations:
            payload = self.payload("SubagentStop", response=response)
            event = json.loads(self.cli("parse-hook", "--hook", "subagent-stop", data=payload).stdout)
            self.assertEqual((event["subagent_id"], event["tool_use_id"]), (child, tool))
        rows = [json.loads(line) for line in native.read_text(encoding="utf-8").splitlines()]
        for row in rows:
            if row.get("type") == "subagent.completed":
                row["resultSummary"] = "Done"
        native.write_text("".join(json.dumps(row) + "\n" for row in rows), encoding="utf-8")
        result = self.cli("parse-hook", "--hook", "subagent-stop",
                          data=self.payload("SubagentStop", response="Done"), success=False)
        self.assertIn(b"uniquely correlate", result.stderr)

    def test_hook_installation_preserves_unrelated_configuration(self):
        config = self.home / "config.toml"
        original = b'default_model = "sentinel"\n\n[[hooks]]\nevent = "Foreign"\ncommand = "keep"\n'
        config.write_bytes(original)
        result = json.loads(self.cli("install-hooks").stdout)
        self.assertEqual(result["hooks_installed"], 9)
        self.assertTrue(config.read_bytes().startswith(original))
        self.assertTrue(json.loads(self.cli("are-hooks-installed").stdout)["installed"])
        self.cli("uninstall-hooks")
        self.assertEqual(config.read_bytes(), original)
        self.assertFalse(json.loads(self.cli("are-hooks-installed").stdout)["installed"])


if __name__ == "__main__":
    unittest.main()
