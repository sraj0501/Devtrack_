import requests
from requests.auth import HTTPBasicAuth
import os
import sys
import pandas as pd
from datetime import datetime

# Add project root for config
_script_dir = os.path.dirname(os.path.abspath(__file__))
_project_root = os.path.dirname(os.path.dirname(_script_dir))
if _project_root not in sys.path:
    sys.path.insert(0, _project_root)

try:
    from backend.config import _load_env, azure_org, azure_project, azure_pat
    _load_env()
    org = azure_org()
    project = azure_project()
    pat = azure_pat()
except ImportError:
    from dotenv import load_dotenv
    load_dotenv(os.path.join(_project_root, ".env"))
    org = os.getenv("ORGANIZATION")
    project = os.getenv("PROJECT")
    pat = os.getenv("AZURE_DEVOPS_PAT") or os.getenv("AZURE_API_KEY")

if not org or not project or not pat:
    print("❌ Missing ORGANIZATION, PROJECT, or AZURE_DEVOPS_PAT/AZURE_API_KEY in .env")
    sys.exit(1)
api_version = os.getenv("API_VERSION", "7.1")
assigned_to = os.getenv("EMAIL")

query_url = f"https://dev.azure.com/{org}/{project}/_apis/wit/wiql?api-version={api_version}"
work_item_url = f"https://dev.azure.com/{org}/_apis/wit/workitems"
headers = {"Content-Type": "application/json"}
auth = HTTPBasicAuth('', pat)

# === WIQL QUERY TO FETCH USER STORIES ===
wiql_query = {
    "query": f"""
    SELECT [System.Id],[System.WorkItemType], [System.Title], [System.State], [System.AssignedTo]
    FROM WorkItems
    WHERE [System.TeamProject] = '{project}'
    AND [System.AssignedTo] = '{assigned_to}'
    AND [System.WorkItemType] = 'Product Backlog Item'
    ORDER BY [System.ChangedDate] DESC"""
}

# === EXECUTE QUERY ===
response = requests.post(query_url, json=wiql_query, headers=headers, auth=auth)
if response.status_code != 200:
    print(f"❌ Failed to fetch work items: {response.status_code} - {response.text}")
    sys.exit()

work_item_ids = [item["id"] for item in response.json()["workItems"]]
print(f"🔍 Found {len(work_item_ids)} user stories.")


# === FETCH DETAILS FOR EACH WORK ITEM ===
if work_item_ids:
    ids_str = ",".join(map(str, work_item_ids[:200]))
    details_url = f"{work_item_url}?ids={ids_str}&api-version={api_version}"
    details_response = requests.get(details_url, headers=headers, auth=auth)

    if details_response.status_code == 200:
        items = details_response.json()["value"]
        # print(items)
        data = [{
            "ID": item["id"],
            "WorkItem Type": item["fields"].get("System.WorkItemType",""),
            "Title": item["fields"].get("System.Title", ""),
            "State": item["fields"].get("System.State", ""),
            "Assigned To": item["fields"].get("System.AssignedTo", {}).get("displayName", "Unassigned")
        } for item in items]

        df = pd.DataFrame(data)
        print(df)
    else:
        print(f"❌ Failed to fetch item details: {details_response.status_code} - {details_response.text}")
else:
    print("⚠️ No user stories found.")
