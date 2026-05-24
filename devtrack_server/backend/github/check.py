"""
GitHub connectivity check.

Loads .env directly, makes a plain HTTP request to verify credentials.
Run via:  devtrack github-check
          uv run python backend/github/check.py
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

    from backend.config import get_github_token, get_github_owner, get_github_repo, get_github_api_url, get_github_api_version
    raw_url = get_github_api_url().rstrip("/")
    api_base = raw_url if raw_url else "https://api.github.com"
    token = get_github_token()
    owner = get_github_owner()
    repo = get_github_repo()

    print("GitHub Connection Check")
    print("=" * 40)
    print()

    # --- Config summary ---
    print("Config")
    print("-" * 40)
    print(f"  API base:   {api_base}")
    print(f"  Owner:      {owner or '(not set — GITHUB_OWNER)'}")
    print(f"  Repo:       {repo or '(not set — GITHUB_REPO)'}")
    if token:
        masked = token[:4] + "*" * max(0, len(token) - 8) + token[-4:] if len(token) > 8 else "****"
        print(f"  Token:      {masked}  ({len(token)} chars)")
    else:
        print("  Token:      (not set — GITHUB_TOKEN)")
    print(f"  .env from:  {project_root}")
    print()

    if not token:
        print("ERROR: Missing required var: GITHUB_TOKEN")
        return 1

    headers = {
        "Authorization": f"Bearer {token}",
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": get_github_api_version(),
    }
    timeout = 15

    # --- Test 1: GET /user ---
    print("Tests")
    print("-" * 40)
    url = f"{api_base}/user"
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
        print("  FAIL  401 Unauthorized — token is invalid or expired")
        return 1
    elif r.status_code == 403:
        print("  FAIL  403 Forbidden — token lacks required scopes")
        return 1
    elif r.status_code != 200:
        print(f"  FAIL  HTTP {r.status_code}: {r.text[:200]}")
        return 1

    user = r.json()
    login = user.get("login", "?")
    user_id = user.get("id", "?")
    print(f"  OK    Auth succeeded — login={login} (id={user_id})")
    print()

    # --- Test 2: Issues assigned to me ---
    if owner and repo:
        url2 = f"{api_base}/repos/{owner}/{repo}/issues?assignee={login}&state=open&per_page=5"
    else:
        url2 = f"{api_base}/issues?assignee={login}&state=open&per_page=5"
    print(f"  GET {url2}")
    try:
        r2 = requests.get(url2, headers=headers, timeout=timeout)
    except Exception as e:
        print(f"  FAIL  {e}")
        return 1

    if r2.status_code == 200:
        items = r2.json()
        print(f"  OK    {len(items)} issue(s) returned (assigned to {login})")
    else:
        print(f"  FAIL  HTTP {r2.status_code}: {r2.text[:200]}")
        return 1
    print()

    # --- Test 3 (if GITHUB_REPO set): verify repo access ---
    if repo and owner:
        url3 = f"{api_base}/repos/{owner}/{repo}"
        print(f"  GET {url3}")
        try:
            r3 = requests.get(url3, headers=headers, timeout=timeout)
        except Exception as e:
            print(f"  FAIL  {e}")
            return 1

        if r3.status_code == 200:
            proj = r3.json()
            print(f"  OK    Repo accessible: {proj.get('full_name', f'{owner}/{repo}')}")
        elif r3.status_code == 404:
            print(f"  FAIL  Repo {owner}/{repo} not found — check GITHUB_OWNER / GITHUB_REPO")
            return 1
        else:
            print(f"  FAIL  HTTP {r3.status_code}: {r3.text[:200]}")
            return 1
        print()

    print("Result: All checks passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
