---
name: Azure DevOps Connection Reference
description: What variables and steps are needed to connect to Azure DevOps in this project
type: reference
---

## Required `.env` Variables

| Variable | Description |
|---|---|
| `AZURE_DEVOPS_PAT` | Personal Access Token (primary auth) |
| `AZURE_ORGANIZATION` | Org name from URL: `https://dev.azure.com/{org}` |
| `AZURE_PROJECT` | Project name within the org |

## Optional Variables

| Variable | Description |
|---|---|
| `AZURE_API_KEY` | Fallback to `AZURE_DEVOPS_PAT` |
| `AZURE_API_VERSION` | API version (default: `7.1`) |
| `EMAIL` | Your Azure DevOps user email (for filtering assigned work items) |
| `AZURE_PARENT_WORK_ITEM_ID` | Parent work item ID for task creation |
| `AZURE_DEFAULT_ASSIGNEE` | Default assignee email for new work items |
| `AZURE_STARTING_WORK_ITEM_ID` | Starting ID offset |
| `AZURE_EXCEL_FILE` | Path to Excel file for task import |
| `AZURE_EXCEL_SHEET` | Sheet name in the Excel file |

## How to Generate a PAT

1. Go to `https://dev.azure.com/{your-org}`
2. Click profile icon → **Personal access tokens**
3. **New Token** → set scopes: `Work Items (Read & Write)`, `Code (Read)` at minimum
4. Copy the token immediately — it won't be shown again

## Where It's Documented

- Full docs: `docs/CONFIGURATION.md` → "Azure DevOps" section (line ~957)
- Sample values: `.env_sample` lines 167–176
- Code: `backend/azure/azure_work_items.py` (reads `ORGANIZATION` + `AZURE_DEVOPS_PAT`)
- Config accessors: `backend/config.py` → `azure_org()`, `azure_pat()`
