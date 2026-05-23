"""
Azure DevOps connectivity check.

Loads .env directly, makes a plain HTTP request to verify credentials.
Run via:  devtrack azure-check
          uv run python backend/azure/check.py
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
    from requests.auth import HTTPBasicAuth

    from backend.config import azure_pat, get_azure_organization, get_azure_project_name, get_azure_api_version
    pat = azure_pat()
    org = get_azure_organization()
    project = get_azure_project_name()
    api_version = get_azure_api_version()

    print("Azure DevOps Connection Check")
    print("=" * 40)
    print()

    # --- Config summary ---
    print("Config")
    print("-" * 40)
    print(f"  Org:        {org or '(not set — AZURE_ORGANIZATION)'}")
    print(f"  Project:    {project or '(not set — AZURE_PROJECT)'}")
    if pat:
        masked = pat[:4] + "*" * max(0, len(pat) - 8) + pat[-4:] if len(pat) > 8 else "****"
        print(f"  PAT:        {masked}  ({len(pat)} chars)")
    else:
        print("  PAT:        (not set — AZURE_DEVOPS_PAT or AZURE_API_KEY)")
    print(f"  API ver:    {api_version}")
    print(f"  .env from:  {project_root}")
    print()

    if not org or not pat:
        missing = [v for v, k in [("AZURE_ORGANIZATION", org), ("AZURE_DEVOPS_PAT", pat)] if not k]
        print(f"ERROR: Missing required vars: {', '.join(missing)}")
        return 1

    auth = HTTPBasicAuth("", pat)
    headers = {"Accept": "application/json"}
    timeout = 15

    # --- Test 1: List projects (org-level auth check) ---
    print("Tests")
    print("-" * 40)
    url = f"https://dev.azure.com/{org}/_apis/projects?api-version={api_version}"
    print(f"  GET {url}")
    try:
        r = requests.get(url, auth=auth, headers=headers, timeout=timeout)
    except requests.exceptions.ConnectionError as e:
        print(f"  FAIL  Connection error: {e}")
        return 1
    except requests.exceptions.Timeout:
        print(f"  FAIL  Request timed out after {timeout}s")
        return 1

    if r.status_code == 401:
        print(f"  FAIL  401 Unauthorized — PAT is invalid or expired")
        print()
        print("  Fix: generate a new PAT at:")
        print(f"       https://{org}.visualstudio.com/_usersSettings/tokens")
        print("  Required scopes: Work Items (Read & Write), Project (Read)")
        return 1
    elif r.status_code == 403:
        print(f"  FAIL  403 Forbidden — PAT lacks required scopes")
        return 1
    elif r.status_code != 200:
        print(f"  FAIL  HTTP {r.status_code}: {r.text[:200]}")
        return 1

    data = r.json()
    projects = data.get("value", [])
    project_names = [p["name"] for p in projects]
    print(f"  OK    Auth succeeded — {len(projects)} project(s) visible")
    for name in project_names:
        marker = " <-- AZURE_PROJECT" if name == project else ""
        print(f"        • {name}{marker}")
    print()

    if project and project not in project_names:
        print(f"  WARN  AZURE_PROJECT='{project}' not found above.")
        print(f"        Available: {', '.join(project_names)}")
        print()

    # --- Test 2: Query work items assigned to me ---
    if project:
        wiql_url = f"https://dev.azure.com/{org}/{project}/_apis/wit/wiql?api-version={api_version}"
        wiql = {"query": "SELECT [System.Id] FROM WorkItems WHERE [System.AssignedTo] = @Me ORDER BY [System.ChangedDate] DESC"}
        print(f"  POST {wiql_url}")
        try:
            r2 = requests.post(wiql_url, json=wiql, auth=auth, headers=headers, timeout=timeout)
        except Exception as e:
            print(f"  FAIL  {e}")
            return 1

        if r2.status_code == 200:
            items = r2.json().get("workItems", [])
            print(f"  OK    WIQL query succeeded — {len(items)} work item(s) assigned to you")
        else:
            print(f"  FAIL  HTTP {r2.status_code}: {r2.text[:200]}")
            return 1
        print()

    print("Result: All checks passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
