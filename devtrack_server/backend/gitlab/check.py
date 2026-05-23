"""
GitLab connectivity check.

Loads .env directly, makes a plain HTTP request to verify credentials.
Run via:  devtrack gitlab-check
          uv run python backend/gitlab/check.py
"""

import os
import sys
from pathlib import Path


def _load_env() -> Path:
    """Load .env by walking up from this file. Returns project root."""
    here = Path(__file__).resolve()
    for parent in [here, *here.parents]:
        env_file = parent / ".env"
        if env_file.exists():
            try:
                from dotenv import load_dotenv
                load_dotenv(env_file, override=True)
            except ImportError:
                pass  # dotenv not installed — rely on env already being set
            return parent
    return here.parent.parent.parent  # fallback: automation_tools/


def main() -> int:
    project_root = _load_env()

    import requests

    from backend.config import get_gitlab_url, get_gitlab_pat, get_gitlab_project_id_str
    gitlab_url = get_gitlab_url().rstrip("/")
    pat = get_gitlab_pat()
    project_id = get_gitlab_project_id_str()

    print("GitLab Connection Check")
    print("=" * 40)
    print()

    # --- Config summary ---
    print("Config")
    print("-" * 40)
    print(f"  URL:        {gitlab_url}")
    if pat:
        masked = pat[:4] + "*" * max(0, len(pat) - 8) + pat[-4:] if len(pat) > 8 else "****"
        print(f"  PAT:        {masked}  ({len(pat)} chars)")
    else:
        print("  PAT:        (not set — GITLAB_PAT)")
    print(f"  Project ID: {project_id or '(not set — GITLAB_PROJECT_ID)'}")
    print(f"  .env from:  {project_root}")
    print()

    if not pat:
        print("ERROR: Missing required var: GITLAB_PAT")
        return 1

    headers = {"Private-Token": pat, "Accept": "application/json"}
    timeout = 15

    # --- Test 1: GET /api/v4/user ---
    print("Tests")
    print("-" * 40)
    url = f"{gitlab_url}/api/v4/user"
    print(f"  GET {url}")
    try:
        r = requests.get(url, headers=headers, timeout=timeout)
    except requests.exceptions.ConnectionError as e:
        print(f"  FAIL  Connection error: {e}")
        return 1
    except requests.exceptions.Timeout:
        print(f"  FAIL  Request timed out after {timeout}s")
        return 1

    if r.status_code == 401:
        print("  FAIL  401 Unauthorized — PAT is invalid or expired")
        return 1
    elif r.status_code == 403:
        print("  FAIL  403 Forbidden — PAT lacks required scopes")
        return 1
    elif r.status_code != 200:
        print(f"  FAIL  HTTP {r.status_code}: {r.text[:200]}")
        return 1

    user = r.json()
    username = user.get("username", "?")
    user_id = user.get("id", "?")
    print(f"  OK    Auth succeeded — username={username} (id={user_id})")
    print()

    # --- Test 2: Issues assigned to me ---
    url2 = f"{gitlab_url}/api/v4/issues?assignee_id={user_id}&state=opened&per_page=5"
    print(f"  GET {url2}")
    try:
        r2 = requests.get(url2, headers=headers, timeout=timeout)
    except Exception as e:
        print(f"  FAIL  {e}")
        return 1

    if r2.status_code == 200:
        items = r2.json()
        total_header = r2.headers.get("X-Total", "?")
        print(f"  OK    {len(items)} issue(s) returned (total assigned: {total_header})")
    else:
        print(f"  FAIL  HTTP {r2.status_code}: {r2.text[:200]}")
        return 1
    print()

    # --- Test 3 (if GITLAB_PROJECT_ID set): verify project access ---
    if project_id:
        url3 = f"{gitlab_url}/api/v4/projects/{project_id}"
        print(f"  GET {url3}")
        try:
            r3 = requests.get(url3, headers=headers, timeout=timeout)
        except Exception as e:
            print(f"  FAIL  {e}")
            return 1

        if r3.status_code == 200:
            proj = r3.json()
            print(f"  OK    Project accessible: {proj.get('path_with_namespace', project_id)}")
        elif r3.status_code == 404:
            print(f"  FAIL  Project {project_id} not found — check GITLAB_PROJECT_ID")
            return 1
        else:
            print(f"  FAIL  HTTP {r3.status_code}: {r3.text[:200]}")
            return 1
        print()

    print("Result: All checks passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
