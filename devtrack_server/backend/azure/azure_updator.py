import requests
from requests.auth import HTTPBasicAuth
import os
import sys
import pandas as pd
from datetime import datetime, timedelta

# Add project root for config
_script_dir = os.path.dirname(os.path.abspath(__file__))
_project_root = os.path.dirname(os.path.dirname(_script_dir))
if _project_root not in sys.path:
    sys.path.insert(0, _project_root)

# Load env from project root .env
try:
    from backend.config import _load_env, azure_org, azure_project, azure_pat
    _load_env()
    org = azure_org()
    project = azure_project()
    pat = azure_pat()
    from backend.config import azure_excel_file, azure_excel_sheet, azure_parent_work_item_id
    excel_file = str(azure_excel_file())
    sheet_name = azure_excel_sheet()
    selected_parent_id = azure_parent_work_item_id() or ""
except ImportError:
    from dotenv import load_dotenv
    for env_path in [os.path.join(_project_root, ".env"), ".env"]:
        if os.path.exists(env_path):
            load_dotenv(env_path)
            break
    org = os.getenv("ORGANIZATION")
    project = os.getenv("PROJECT")
    pat = os.getenv("AZURE_DEVOPS_PAT") or os.getenv("AZURE_API_KEY")
    excel_file = os.getenv("AZURE_EXCEL_FILE") or os.path.join(_project_root, "backend", "data", "tasks.xlsx")
    sheet_name = os.getenv("AZURE_EXCEL_SHEET", "my_tasks")
    selected_parent_id = os.getenv("AZURE_PARENT_WORK_ITEM_ID", "")

if not org or not project or not pat:
    print("❌ Missing ORGANIZATION, PROJECT, or AZURE_DEVOPS_PAT/AZURE_API_KEY in .env")
    sys.exit(1)

assigned_to = os.getenv("EMAIL")
api_version = os.getenv("API_VERSION", "7.1")

api_url = f"https://dev.azure.com/{org}/{project}/_apis/wit/workitems/$Task?api-version={api_version}"
query_url = f"https://dev.azure.com/{org}/{project}/_apis/wit/wiql?api-version={api_version}"
headers = {"Content-Type": "application/json-patch+json"}
auth = HTTPBasicAuth('', pat)

# === READ EXCEL ===
df = pd.read_excel(excel_file, sheet_name=sheet_name)

# Ensure correct data types and add columns if not present
for col in ["Work Item ID", "DevOps URL", "Iteration Path", "Parent Work Item ID"]:
    if col not in df.columns:
        df[col] = ""

# Explicitly set column data types to avoid pandas warnings
df["DevOps URL"] = df["DevOps URL"].astype(str)
df["Work Item ID"] = df["Work Item ID"].astype(str)
df["Iteration Path"] = df["Iteration Path"].astype(str)
df["Parent Work Item ID"] = df["Parent Work Item ID"].astype(str)

# === FETCH EXISTING TASK TITLES ===
def fetch_existing_tasks():
    """Fetch existing tasks using individual work item checks"""
    existing_titles = set()
    
    print("🔍 Checking for existing tasks to prevent duplicates...")
    
    try:
        # Check a range of recent work item IDs (skip if no parent ID configured)
        if not selected_parent_id or not str(selected_parent_id).strip():
            print("⚠️ AZURE_PARENT_WORK_ITEM_ID not set - duplicate check limited")
            return existing_titles
        parent_id = int(selected_parent_id)
        recent_ids = range(max(1, parent_id - 100), parent_id + 100)
        
        for wid in list(recent_ids)[:50]:  # Check last 50 work items
            detail_url = f"https://dev.azure.com/{org}/_apis/wit/workitems/{wid}?fields=System.Title,System.WorkItemType&api-version={api_version}"
            detail_resp = requests.get(detail_url, headers={"Content-Type": "application/json"}, auth=auth)
            
            if detail_resp.status_code == 200:
                fields = detail_resp.json().get("fields", {})
                if fields.get("System.WorkItemType") == "Task":
                    title = fields.get("System.Title", "")
                    if title:
                        existing_titles.add(title)
        
        print(f"✅ Found {len(existing_titles)} existing task titles")
        return existing_titles
            
    except Exception as e:
        print(f"❌ Failed to fetch existing tasks: {str(e)}")
        print("⚠️ Duplicate prevention relies on Excel file only")
        return existing_titles

existing_titles = fetch_existing_tasks()

# === FETCH ITERATIONS ===
iteration_url = f"https://dev.azure.com/{org}/{project}/_apis/work/teamsettings/iterations?api-version={api_version}"
iteration_resp = requests.get(iteration_url, headers=headers, auth=auth)
iterations = iteration_resp.json().get("value", []) if iteration_resp.status_code == 200 else []

sorted_iterations = sorted(
    [it for it in iterations if "attributes" in it and "startDate" in it["attributes"]],
    key=lambda x: x["attributes"]["startDate"]
)
bottom_5_iterations = sorted_iterations[-5:] if len(sorted_iterations) >= 5 else sorted_iterations

print("\n📆 Available Iterations (Last 5):")
for i, it in enumerate(bottom_5_iterations):
    print(f"{i + 1}. {it['path']}")

iteration_choice = int(input("Select the iteration number to assign to all tasks: ")) - 1
selected_iteration = bottom_5_iterations[iteration_choice]["path"]
print(f"📌 Selected Iteration Path: {selected_iteration}")

# === CREATE MISSING TASKS ===
due_date = (datetime.now() + timedelta(days=1)).strftime("%Y-%m-%dT18:00:00Z")

print(f"\n🚀 Processing {len(df)} tasks from Excel...")
print(f"📋 Found {len(existing_titles)} existing tasks in Azure DevOps")

# Debug: Show first few existing titles
if existing_titles:
    sample_titles = list(existing_titles)[:5]
    print(f"🔍 Sample existing titles: {sample_titles}")

created_count = 0
skipped_count = 0

for idx, row in df.iterrows():
    title = str(row["Title"]).strip()  # Ensure string and remove whitespace
    tags = str(row["Tags"]).strip() if not pd.isna(row["Tags"]) else ""

    print(f"\n📝 Processing: '{title}'")
    
    # Check if already has Work Item ID in Excel
    work_item_id = str(df.at[idx, "Work Item ID"]).strip()
    if work_item_id and work_item_id != "" and work_item_id.lower() != "nan":
        print(f"🔁 Skipped (already has Work Item ID '{work_item_id}'): {title}")
        skipped_count += 1
        continue
    
    # Check for duplicates in Azure DevOps (case-insensitive comparison)
    title_lower = title.lower()
    existing_titles_lower = {t.lower() for t in existing_titles}
    
    if title_lower in existing_titles_lower:
        # Find the exact match to show
        exact_match = next(t for t in existing_titles if t.lower() == title_lower)
        print(f"🔁 Skipped (already exists in Azure DevOps as '{exact_match}'): {title}")
        skipped_count += 1
        continue

    print(f"🔨 Creating: {title}")

    # Build the payload with field operations
    payload = [
        {"op": "add", "path": "/fields/System.Title", "value": title},
        {"op": "add", "path": "/fields/System.AssignedTo", "value": assigned_to},
        {"op": "add", "path": "/fields/System.Tags", "value": tags},
        {"op": "add", "path": "/fields/Microsoft.VSTS.Scheduling.DueDate", "value": due_date},
        {"op": "add", "path": "/fields/System.Description", "value": f"Auto-created task for: {title}"},
        {"op": "add", "path": "/fields/System.IterationPath", "value": selected_iteration}
    ]

    # Add parent relationship if specified
    if selected_parent_id and str(selected_parent_id).strip():
        payload.append({
            "op": "add",
            "path": "/relations/-",
            "value": {
                "rel": "System.LinkTypes.Hierarchy-Reverse",
                "url": f"https://dev.azure.com/{org}/_apis/wit/workItems/{selected_parent_id}",
                "attributes": {"comment": "Linked to parent story"}
            }
        })

    response = requests.post(api_url, json=payload, headers=headers, auth=auth)

    if response.status_code in [200, 201]:
        item = response.json()
        wid = str(item["id"])
        url = item["_links"]["html"]["href"]
        print(f"✅ Created: {title} | ID: {wid}")
        
        # Update the DataFrame
        df.at[idx, "Work Item ID"] = wid
        df.at[idx, "DevOps URL"] = url
        df.at[idx, "Iteration Path"] = selected_iteration
        if selected_parent_id and str(selected_parent_id).strip():
            df.at[idx, "Parent Work Item ID"] = str(selected_parent_id)
        
        # Add to existing titles to prevent duplicates in current run
        existing_titles.add(title)
        created_count += 1
        
    else:
        print(f"❌ Failed to create: {title} | Error: {response.text}")

print(f"\n📊 Summary:")
print(f"   ✅ Created: {created_count} tasks")
print(f"   🔁 Skipped: {skipped_count} tasks")
print(f"   📋 Total processed: {len(df)} tasks")

# === SAVE UPDATED EXCEL ===
df.to_excel(excel_file, sheet_name=sheet_name, index=False)
print(f"\n📁 Excel file updated with Work Item IDs and URLs → {excel_file}")