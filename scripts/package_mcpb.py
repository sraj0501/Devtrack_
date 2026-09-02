#!/usr/bin/env python3
"""Build a platform-specific DevTrack MCPB archive using only the standard library."""

from __future__ import annotations

import argparse
import json
import re
import stat
import zipfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
TEMPLATE = ROOT / "mcpb" / "manifest.template.json"
README = ROOT / "mcpb" / "README.md"
LICENSE = ROOT / "LICENSE"
SEMVER = re.compile(r"^(?:v)?(\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?)$")
ARCHIVE_TIMESTAMP = (1980, 1, 1, 0, 0, 0)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True, type=Path)
    parser.add_argument("--platform", required=True, choices=("darwin", "linux", "win32"))
    parser.add_argument("--version", required=True)
    parser.add_argument("--output", required=True, type=Path)
    return parser.parse_args()


def write_member(
    bundle: zipfile.ZipFile, name: str, content: bytes, mode: int = 0o644
) -> None:
    info = zipfile.ZipInfo(name, ARCHIVE_TIMESTAMP)
    info.create_system = 3
    info.compress_type = zipfile.ZIP_DEFLATED
    info.external_attr = (stat.S_IFREG | mode) << 16
    bundle.writestr(info, content)


def main() -> None:
    args = parse_args()
    match = SEMVER.fullmatch(args.version.strip())
    if not match:
        raise SystemExit(f"invalid semantic version: {args.version!r}")
    if not args.binary.is_file():
        raise SystemExit(f"binary not found: {args.binary}")

    version = match.group(1)
    binary_name = "devtrack.exe" if args.platform == "win32" else "devtrack"
    raw = TEMPLATE.read_text(encoding="utf-8")
    raw = raw.replace("__VERSION__", version)
    raw = raw.replace("__PLATFORM__", args.platform)
    raw = raw.replace("__BINARY__", binary_name)
    manifest = json.loads(raw)

    args.output.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(args.output, "w") as bundle:
        write_member(
            bundle,
            "manifest.json",
            (json.dumps(manifest, indent=2) + "\n").encode(),
        )
        write_member(bundle, "README.md", README.read_bytes())
        write_member(bundle, "LICENSE", LICENSE.read_bytes())
        write_member(bundle, f"server/{binary_name}", args.binary.read_bytes(), 0o755)

    with zipfile.ZipFile(args.output) as bundle:
        expected = {"manifest.json", "README.md", "LICENSE", f"server/{binary_name}"}
        if set(bundle.namelist()) != expected:
            raise SystemExit(f"unexpected MCPB contents: {bundle.namelist()}")
        packed = json.loads(bundle.read("manifest.json"))
        if packed["version"] != version or packed["compatibility"]["platforms"] != [args.platform]:
            raise SystemExit("packed manifest does not match requested version/platform")

    print(args.output)


if __name__ == "__main__":
    main()
