#!/usr/bin/env python3

import argparse
import re
import sys
from pathlib import Path


def parse_required_env_vars(config_env_path: Path) -> list[str]:
    content = config_env_path.read_text(encoding="utf-8")
    match = re.search(r"var requiredEnvVars = \[]string\{(.*?)\n\}", content, re.S)
    if not match:
        raise ValueError("Could not find requiredEnvVars in config_env.go")
    block = match.group(1)
    return re.findall(r'"([A-Z0-9_]+)"', block)


def parse_env_keys(env_sample_path: Path) -> set[str]:
    keys: set[str] = set()
    for line in env_sample_path.read_text(encoding="utf-8").splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        match = re.match(r"^([A-Z][A-Z0-9_]*)\s*=", stripped)
        if match:
            keys.add(match.group(1))
    return keys


def main() -> int:
    parser = argparse.ArgumentParser(description="Validate .env_sample against requiredEnvVars in devtrack-bin/config_env.go")
    parser.add_argument(
        "--config-env",
        default="devtrack-bin/config_env.go",
        help="Path to config_env.go",
    )
    parser.add_argument(
        "--env-sample",
        default=".env_sample",
        help="Path to .env_sample",
    )
    args = parser.parse_args()

    config_env_path = Path(args.config_env)
    env_sample_path = Path(args.env_sample)

    if not config_env_path.exists():
        print(f"ERROR: file not found: {config_env_path}")
        return 2
    if not env_sample_path.exists():
        print(f"ERROR: file not found: {env_sample_path}")
        return 2

    required = set(parse_required_env_vars(config_env_path))
    present = parse_env_keys(env_sample_path)

    missing = sorted(required - present)
    extra = sorted(present - required)

    allowed_extra = {"AZURE_DEVOPS_PAT", "GITHUB_TOKEN", "JIRA_API_TOKEN", "DATA_DIR"}
    unexpected_extra = [key for key in extra if key not in allowed_extra]

    print(f"Required keys: {len(required)}")
    print(f"Keys in sample: {len(present)}")

    if missing:
        print("\nMissing keys in .env_sample:")
        for key in missing:
            print(f"  - {key}")

    if unexpected_extra:
        print("\nUnexpected extra keys in .env_sample:")
        for key in unexpected_extra:
            print(f"  - {key}")

    if missing:
        print("\nResult: FAIL")
        return 1

    if unexpected_extra:
        print("\nResult: PASS (with warnings)")
        return 0

    print("\nResult: PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
