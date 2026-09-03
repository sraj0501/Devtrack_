#!/usr/bin/env python3
"""Generate official MCP Registry metadata from versioned MCPB artifacts."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
from pathlib import Path


SEMVER = re.compile(r"^(?:v)?(\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?)$")
ARTIFACTS = (
    "devtrack_mcpb_windows_amd64.mcpb",
    "devtrack_mcpb_darwin_amd64.mcpb",
    "devtrack_mcpb_darwin_arm64.mcpb",
    "devtrack_mcpb_linux_amd64.mcpb",
    "devtrack_mcpb_linux_arm64.mcpb",
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True)
    parser.add_argument("--dist", required=True, type=Path)
    parser.add_argument("--output", type=Path)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    match = SEMVER.fullmatch(args.version.strip())
    if not match:
        raise SystemExit(f"invalid semantic version: {args.version!r}")
    version = match.group(1)
    tag = f"v{version}"

    packages = []
    for name in ARTIFACTS:
        artifact = args.dist / name
        if not artifact.is_file():
            raise SystemExit(f"missing MCPB artifact: {artifact}")
        packages.append(
            {
                "registryType": "mcpb",
                "identifier": (
                    "https://github.com/sraj0501/Devtrack_/releases/download/"
                    f"{tag}/{name}"
                ),
                "fileSha256": hashlib.sha256(artifact.read_bytes()).hexdigest(),
                "transport": {"type": "stdio"},
            }
        )

    document = {
        "$schema": "https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json",
        "name": "io.github.sraj0501/devtrack",
        "title": "DevTrack",
        "description": (
            "Read local tickets, commits, pending actions, voice profile, and EOD context over MCP."
        ),
        "repository": {
            "url": "https://github.com/sraj0501/Devtrack_.git",
            "source": "github",
        },
        "version": version,
        "packages": packages,
    }
    output = args.output or args.dist / "server.json"
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(document, indent=2) + "\n", encoding="utf-8")
    print(output)


if __name__ == "__main__":
    main()
