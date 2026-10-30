"""Regression checks for the isolated upstream protocol-suite launcher."""
import json
from pathlib import Path
import runpy
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).resolve().parent / "verify-protocol.py"
FIXTURES = SCRIPT.parent.parent / "testdata"


class VerifyProtocolTests(unittest.TestCase):
    def run_wrapper(self, suite_exit):
        with tempfile.TemporaryDirectory() as temp:
            binary = Path(temp) / "adapter"
            binary.touch()

            def verify(args, *, cwd, env):
                self.assertEqual(args[:3], ["external-agents-tests", "verify", str(binary)])
                self.assertEqual(args[3], "--fixtures")
                self.assertNotEqual(Path(cwd), SCRIPT.parent.parent)
                self.assertEqual(Path(args[4]).parent, Path(cwd))
                fixture = json.loads(Path(args[4]).read_text(encoding="utf-8"))
                analyzer = fixture["transcript_analyzer"]
                self.assertEqual(analyzer["session_ref"], str(FIXTURES / "transcripts" / "normal.jsonl"))
                self.assertTrue(Path(analyzer["session_ref"]).is_file())
                self.assertEqual(analyzer["modified_files"], ["kimi-integration-probe.txt"])
                self.assertTrue(analyzer["prompts"])
                self.assertEqual(analyzer["summary"], "PROBE_DONE")
                self.assertEqual(env["KIMI_CODE_HOME"], str(Path(cwd) / "kimi-home"))
                self.assertEqual(env["ENTIRE_KIMI_OFFSET_UNIT"], "bytes")
                return subprocess.CompletedProcess(args, suite_exit)

            with patch.object(sys, "argv", [str(SCRIPT), str(binary)]), patch("subprocess.run", side_effect=verify) as run:
                with self.assertRaises(SystemExit) as result:
                    runpy.run_path(str(SCRIPT), run_name="__main__")
                self.assertEqual(result.exception.code, suite_exit)
                run.assert_called_once()
                self.assertFalse(Path(run.call_args.kwargs["cwd"]).exists())

    def test_suite_success(self):
        self.run_wrapper(0)

    def test_suite_failure_is_not_hidden(self):
        self.run_wrapper(7)


if __name__ == "__main__":
    unittest.main()
