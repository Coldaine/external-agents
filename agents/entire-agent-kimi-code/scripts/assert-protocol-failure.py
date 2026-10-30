"""Assert the real protocol suite rejects an adapter that fails its info command."""
import subprocess
import sys
from pathlib import Path

wrapper = Path(__file__).with_name("verify-protocol.py")
result = subprocess.run([sys.executable, str(wrapper), sys.argv[1]], capture_output=True, text=True)
output = result.stdout + result.stderr
print(output, end="")
if result.returncode == 0 or "info subcommand failed" not in output:
    raise SystemExit("expected the protocol suite to reject the adapter's info command")
