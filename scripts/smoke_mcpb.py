#!/usr/bin/env python3
"""Extract and execute a DevTrack MCPB on its claimed native platform."""

from __future__ import annotations

import argparse
import json
import os
import platform
import stat
import subprocess
import tempfile
import zipfile
from pathlib import Path


EXPECTED_TOOLS = {
    "get_active_context",
    "get_today_commits",
    "get_pending_actions",
    "get_voice_profile",
    "get_ticket_context",
    "get_eod_summary",
}
SYSTEMS = {"darwin": "Darwin", "linux": "Linux", "win32": "Windows"}


def normalize_arch(value: str) -> str:
    value = value.lower()
    if value in {"amd64", "x86_64"}:
        return "amd64"
    if value in {"arm64", "aarch64"}:
        return "arm64"
    return value


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("bundle", type=Path)
    parser.add_argument("--platform", required=True, choices=tuple(SYSTEMS))
    parser.add_argument("--arch", required=True, choices=("amd64", "arm64"))
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    actual_system = platform.system()
    actual_arch = normalize_arch(platform.machine())
    if actual_system != SYSTEMS[args.platform] or actual_arch != args.arch:
        raise SystemExit(
            f"runner mismatch: got {actual_system}/{actual_arch}, "
            f"expected {SYSTEMS[args.platform]}/{args.arch}"
        )

    with tempfile.TemporaryDirectory(prefix="devtrack-mcpb-") as temp:
        root = Path(temp)
        with zipfile.ZipFile(args.bundle) as archive:
            archive.extractall(root)

        manifest = json.loads((root / "manifest.json").read_text(encoding="utf-8"))
        if manifest["compatibility"]["platforms"] != [args.platform]:
            raise SystemExit("manifest platform does not match the native runner")
        if manifest.get("privacy_policies") != ["https://devtrack.cloud/privacy.html"]:
            raise SystemExit("manifest does not declare the DevTrack privacy policy")
        if "## Privacy Policy" not in (root / "README.md").read_text(encoding="utf-8"):
            raise SystemExit("bundle README does not contain a Privacy Policy section")

        executable = root / manifest["server"]["entry_point"]
        executable.chmod(executable.stat().st_mode | stat.S_IXUSR)

        version = subprocess.run(
            [str(executable), "version"], check=True, capture_output=True, text=True
        ).stdout
        if "Version:    dev" in version:
            raise SystemExit("bundle contains an unversioned development binary")

        project = root / "clean-project"
        project.mkdir()
        setup = subprocess.run(
            [str(executable), "mcp", "setup", "--dir", str(project)],
            check=True,
            capture_output=True,
            text=True,
            env={**os.environ, "DEVTRACK_ENV_FILE": str(root / "devtrack.env")},
        )
        mcp_config = json.loads((project / ".mcp.json").read_text(encoding="utf-8"))
        configured = mcp_config["mcpServers"]["devtrack"]
        if Path(configured["command"]).resolve() != executable.resolve():
            raise SystemExit("mcp setup did not register the extracted bundle executable")
        if configured["args"] != ["mcp"] or "Written:" not in setup.stdout:
            raise SystemExit("mcp setup produced an unexpected project configuration")

        database = root / "smoke" / "devtrack.db"
        database.parent.mkdir()
        requests = [
            {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}},
            {"jsonrpc": "2.0", "method": "notifications/initialized"},
            {"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}},
            {
                "jsonrpc": "2.0",
                "id": 3,
                "method": "tools/call",
                "params": {"name": "get_active_context", "arguments": {}},
            },
            {"jsonrpc": "2.0", "id": 99, "method": "shutdown"},
        ]
        payload = "\n".join(json.dumps(item) for item in requests) + "\n"
        proc = subprocess.run(
            [str(executable), "mcp", "serve", "--database", str(database)],
            input=payload,
            capture_output=True,
            text=True,
            timeout=30,
            env=os.environ.copy(),
        )
        if proc.returncode != 0:
            raise SystemExit(f"MCP server failed ({proc.returncode}): {proc.stderr}")

        responses = [json.loads(line) for line in proc.stdout.splitlines() if line.strip()]
        by_id = {response.get("id"): response for response in responses}
        if set(by_id) != {1, 2, 3, 99}:
            raise SystemExit(f"unexpected MCP response IDs: {sorted(by_id)}")
        if any("error" in response for response in responses):
            raise SystemExit(f"MCP smoke test returned an error: {responses}")

        tools = by_id[2]["result"]["tools"]
        if {tool["name"] for tool in tools} != EXPECTED_TOOLS:
            raise SystemExit("MCP bundle did not expose the expected six tools")
        for tool in tools:
            annotations = tool.get("annotations", {})
            if not tool.get("title") or annotations.get("readOnlyHint") is not True:
                raise SystemExit(f"missing read-only metadata for {tool.get('name')}")
            if annotations.get("destructiveHint") is not False:
                raise SystemExit(f"incorrect destructive metadata for {tool.get('name')}")

        print(
            f"PASS {args.platform}/{args.arch}: extracted bundle, versioned binary, clean-project "
            "setup, initialize, tools/list, tool call, and shutdown"
        )


if __name__ == "__main__":
    main()
