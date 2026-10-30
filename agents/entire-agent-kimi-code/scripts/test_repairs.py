"""Regression tests for release archive and lifecycle fixture tools."""
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest
import zipfile

SCRIPTS = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("verify_release", SCRIPTS / "verify-release.py")
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ScriptRegressionTests(unittest.TestCase):
    def test_archive_binary_is_extracted_to_fixed_destination(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            for kind, member, system in (("tar", "../escape/entire-agent-kimi-code", "linux"),
                                         ("tar", "nested/entire-agent-kimi-code", "linux"),
                                         ("zip", "../escape/entire-agent-kimi-code.exe", "windows"),
                                         ("zip", "nested/entire-agent-kimi-code.exe", "windows")):
                with self.subTest(kind=kind, member=member):
                    dest = root / (kind + str(len(list(root.iterdir()))))
                    dest.mkdir()
                    archive = dest / ("package.zip" if kind == "zip" else "package.tar.gz")
                    if kind == "tar":
                        with tarfile.open(archive, "w:gz") as tar:
                            info = tarfile.TarInfo(member)
                            info.size = len(b"binary")
                            tar.addfile(info, io.BytesIO(b"binary"))
                    else:
                        with zipfile.ZipFile(archive, "w") as zf:
                            zf.writestr(member, b"binary")
                    binary = release.extract_host_binary(archive, system, dest)
                    self.assertEqual(binary, dest / release.binary_name(system))
                    self.assertEqual(binary.read_bytes(), b"binary")
                    self.assertFalse((root / "escape").exists())
                    self.assertFalse((dest / "nested").exists())
                    if system == "linux" and os.name != "nt":
                        self.assertTrue(binary.stat().st_mode & 0o111)

    def test_sanitize_transcript_alias_and_existing_flag(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source = root / "input.jsonl"
            source.write_text(json.dumps({"type": "llm.request", "secret": "drop"}) + "\n" +
                              json.dumps({"type": "turn.prompt", "path": str(root / "file")}) + "\n",
                              encoding="utf-8")
            outputs = []
            for command in (("sanitize-transcript",), ("sanitize", "--transcript")):
                output = root / ("out" + str(len(outputs)) + ".jsonl")
                subprocess.run([sys.executable, str(SCRIPTS / "capture-kimi-lifecycle.py"),
                                command[0], str(source), str(output), "--repo", str(root), *command[1:]],
                               check=True, capture_output=True, timeout=30)
                outputs.append(output.read_text(encoding="utf-8"))
            self.assertEqual(outputs[0], outputs[1])
            rows = [json.loads(line) for line in outputs[0].splitlines()]
            self.assertEqual(rows, [{"type": "turn.prompt", "path": "/test/repo/file"}])


    def test_sanitize_hook_keeps_existing_behavior(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source = root / "hook.json"
            output = root / "sanitized.json"
            source.write_text(json.dumps({"session_id": "session_private", "cwd": str(root)}), encoding="utf-8")
            subprocess.run([sys.executable, str(SCRIPTS / "capture-kimi-lifecycle.py"), "sanitize",
                            str(source), str(output), "--repo", str(root)],
                           check=True, capture_output=True, timeout=30)
            self.assertEqual(json.loads(output.read_text(encoding="utf-8")),
                             {"session_id": "session_lifecycle", "cwd": "/test/repo"})


if __name__ == "__main__":
    unittest.main()
