# DevTrack Configuration Reference

Complete reference for all .env variables and configuration options.

---

## Overview

DevTrack is configured via a single `.env` file with **no hardcoded defaults or fallback paths**.

- Copy `.env_sample` to `.env`
- Edit with your paths and credentials
- All variables must be absolute paths, not relative
- Daemon reads `.env` at startup via `DEVTRACK_ENV_FILE`

---

## Environment Variable Groups

Configuration variables are organized by function:

1. **System Paths** - Where DevTrack stores data
2. **IPC Configuration** - Go ↔ Python communication
3. **LLM/AI Configuration** - Which AI provider to use
4. **Integration Credentials** - Azure DevOps, GitHub, Jira, Teams
5. **Feature Flags** - Enable/disable features
6. **Scheduling** - Timer and trigger intervals

---

## System Paths (Required)

These define where DevTrack stores everything.

### PROJECT_ROOT

**What**: Root directory of automation_tools repository
**Format**: Absolute path
**Example**: `/home/user/Documents/automation_tools`
**Required**: YES

```bash
PROJECT_ROOT=/home/user/Documents/automation_tools
```

### DEVTRACK_WORKSPACE

**What**: Git repository to monitor
**Format**: Absolute path
**Example**: `/home/user/Documents/automation_tools` or `/home/user/Projects/my-project`
**Required**: YES
**Note**: Can be same as PROJECT_ROOT or any Git repository

```bash
# Monitor the automation_tools repo itself
DEVTRACK_WORKSPACE=${PROJECT_ROOT}

# Or monitor a different project
DEVTRACK_WORKSPACE=/home/user/Projects/my-project
```

### DEVTRACK_HOME

**What**: Directory for DevTrack binary and config
**Format**: Absolute path
**Example**: `${PROJECT_ROOT}/devtrack-bin`
**Required**: Usually
**Default**: `${PROJECT_ROOT}/devtrack-bin`

```bash
DEVTRACK_HOME=${PROJECT_ROOT}/devtrack-bin
```

### DATA_DIR

**What**: Root data directory for databases, logs, reports, pids
**Format**: Absolute path
**Example**: `${PROJECT_ROOT}/Data`
**Required**: YES

```bash
DATA_DIR=${PROJECT_ROOT}/Data
```

### DATABASE_DIR

**What**: SQLite database directory
**Format**: Absolute path
**Example**: `${DATA_DIR}/db`
**Required**: Usually
**Default**: `${DATA_DIR}/db`

```bash
DATABASE_DIR=${DATA_DIR}/db
```

### LOG_DIR

**What**: Log files directory
**Format**: Absolute path
**Example**: `${DATA_DIR}/logs`
**Required**: Usually
**Default**: `${DATA_DIR}/logs`

```bash
LOG_DIR=${DATA_DIR}/logs
```

### PID_DIR

**What**: Process ID file directory
**Format**: Absolute path
**Example**: `${DATA_DIR}/pid`
**Required**: Usually
**Default**: `${DATA_DIR}/pid`

```bash
PID_DIR=${DATA_DIR}/pid
```

### REPORT_DIR

**What**: Generated reports directory
**Format**: Absolute path
**Example**: `${DATA_DIR}/reports`
**Required**: Usually
**Default**: `${DATA_DIR}/reports`

```bash
REPORT_DIR=${DATA_DIR}/reports
```

---

## IPC Configuration

How Go daemon and Python bridge communicate.

### IPC_HOST

**What**: Host where IPC server listens
**Format**: IP address or hostname
**Example**: `127.0.0.1` or `localhost`
**Default**: `127.0.0.1`
**Note**: Should be localhost (not exposed to network)

```bash
IPC_HOST=127.0.0.1
```

### IPC_PORT

**What**: Port for IPC communication
**Format**: Integer 1024-65535
**Example**: `35893`
**Default**: `35893`
**Note**: Change if port already in use

```bash
IPC_PORT=35893
```

### IPC_CONNECT_TIMEOUT_SECS

**What**: Timeout for IPC server connection attempts (seconds)
**Format**: Integer (seconds)
**Example**: `5`
**Required**: YES (no default)
**Note**: If Python bridge can't connect to Go daemon, fails with clear error

```bash
IPC_CONNECT_TIMEOUT_SECS=5
```

### IPC_RETRY_DELAY_MS

**What**: Delay between IPC reconnection attempts (milliseconds)
**Format**: Integer (milliseconds)
**Example**: `2000`
**Required**: YES (no default)
**Note**: Controls how long daemon waits before retrying failed IPC connections

```bash
IPC_RETRY_DELAY_MS=2000
```

---

## Timeout Configuration

All request timeouts must be explicitly configured. Missing any causes daemon to fail at startup with clear error message.

### HTTP_TIMEOUT_SHORT

**What**: Timeout for fast HTTP operations (seconds)
**Format**: Integer (seconds)
**Example**: `10`
**Required**: YES (no default)
**Used by**: Quick API calls, health checks, brief operations

```bash
HTTP_TIMEOUT_SHORT=10
```

### HTTP_TIMEOUT

**What**: Timeout for standard HTTP operations (seconds)
**Format**: Integer (seconds)
**Example**: `30`
**Required**: YES (no default)
**Used by**: Most API calls, normal operations

```bash
HTTP_TIMEOUT=30
```

### HTTP_TIMEOUT_LONG

**What**: Timeout for long-running HTTP operations (seconds)
**Format**: Integer (seconds)
**Example**: `60`
**Required**: YES (no default)
**Used by**: Large file uploads, complex API operations, data collection

```bash
HTTP_TIMEOUT_LONG=60
```

### LLM_REQUEST_TIMEOUT_SECS

**What**: Timeout for LLM API requests (seconds)
**Format**: Integer (seconds)
**Example**: `120`
**Required**: YES (no default)
**Used by**: AI model inference, text generation, enhancement operations

```bash
LLM_REQUEST_TIMEOUT_SECS=120
```

### PROMPT_TIMEOUT_SIMPLE_SECS

**What**: Timeout for simple TUI prompts (seconds)
**Format**: Integer (seconds)
**Example**: `30`
**Required**: YES (no default)
**Used by**: Quick yes/no prompts, simple confirmations

```bash
PROMPT_TIMEOUT_SIMPLE_SECS=30
```

### PROMPT_TIMEOUT_WORK_SECS

**What**: Timeout for work update prompts (seconds)
**Format**: Integer (seconds)
**Example**: `60`
**Required**: YES (no default)
**Used by**: Work update text input, task description entry

```bash
PROMPT_TIMEOUT_WORK_SECS=60
```

### PROMPT_TIMEOUT_TASK_SECS

**What**: Timeout for detailed task prompts (seconds)
**Format**: Integer (seconds)
**Example**: `120`
**Required**: YES (no default)
**Used by**: Complex task descriptions, multi-line input

```bash
PROMPT_TIMEOUT_TASK_SECS=120
```

---

## Host & Endpoint Configuration

### OLLAMA_HOST

**What**: URL where Ollama server is running
**Format**: `http://host:port`
**Example**: `http://localhost:11434`
**Required**: YES if using Ollama (no default)
**Note**: Used for local AI inference

```bash
OLLAMA_HOST=http://localhost:11434

# If Ollama on different machine
OLLAMA_HOST=http://192.168.1.100:11434
```

### LMSTUDIO_HOST

**What**: URL where LM Studio server is running
**Format**: `http://host:port/v1`
**Example**: `http://localhost:1234/v1`
**Required**: YES if using LM Studio (no default)
**Note**: Used for local AI inference with LM Studio backend

```bash
LMSTUDIO_HOST=http://localhost:1234/v1
```

---

## Model & Analysis Configuration

### GIT_SAGE_DEFAULT_MODEL

**What**: Default LLM model for git-sage tool
**Format**: String (model name)
**Example**: `llama3`
**Required**: YES (no default)
**Note**: Must be available in selected LLM provider

```bash
GIT_SAGE_DEFAULT_MODEL=llama3
```

### SENTIMENT_ANALYSIS_WINDOW_MINUTES

**What**: Time window for sentiment analysis (minutes)
**Format**: Integer (minutes)
**Example**: `120`
**Required**: YES (no default)
**Note**: Controls how far back to analyze for sentiment trends

```bash
SENTIMENT_ANALYSIS_WINDOW_MINUTES=120
```

---

## LLM/AI Configuration

Which AI provider to use for enhancement and reporting.

### LLM_PROVIDER

**What**: Primary LLM provider
**Options**: `ollama`, `openai`, `anthropic`, `none`
**Default**: `ollama`
**Example**: `openai`
**Note**: If primary unavailable, tries fallbacks (OpenAI → Anthropic → Ollama)

```bash
LLM_PROVIDER=ollama      # Local AI (default, free)
# OR
LLM_PROVIDER=openai      # Cloud AI (paid, better quality)
# OR
LLM_PROVIDER=anthropic   # Cloud AI (paid, reasoning focused)
# OR
LLM_PROVIDER=none        # Disable AI (features degrade)
```

### Ollama Configuration

#### OLLAMA_URL

**What**: URL where Ollama server is running
**Format**: `http://host:port`
**Example**: `http://localhost:11434`
**Default**: `http://localhost:11434`

```bash
OLLAMA_URL=http://localhost:11434

# If Ollama on different machine
OLLAMA_URL=http://192.168.1.100:11434
```

#### OLLAMA_MODEL

**What**: Which Ollama model to use
**Options**: `mistral`, `llama2`, `neural-chat`, `orca-mini`, `codellama`, etc.
**Default**: `mistral`
**Note**: Download with `ollama pull {model}`

```bash
OLLAMA_MODEL=mistral           # Good balance (default)
OLLAMA_MODEL=orca-mini         # Fast (small hardware)
OLLAMA_MODEL=codellama         # Good for code (slow)
OLLAMA_MODEL=llama2            # Reliable (medium speed)
```

#### OLLAMA_TEMPERATURE

**What**: Creativity level (0=deterministic, 1=creative)
**Format**: Float 0.0-1.0
**Default**: `0.7`
**Example**: `0.3` for factual, `0.9` for creative

```bash
OLLAMA_TEMPERATURE=0.3      # For factual tasks
OLLAMA_TEMPERATURE=0.7      # Balanced
OLLAMA_TEMPERATURE=0.9      # For creative tasks
```

### OpenAI Configuration

#### OPENAI_API_KEY

**What**: OpenAI API key for authentication
**Format**: String starting with `sk-`
**Required if**: `LLM_PROVIDER=openai`
**Secret**: YES - never commit this
**Get**: https://platform.openai.com/api-keys

```bash
OPENAI_API_KEY=sk-proj-1234567890abcdefghijklmnopqrstuvwxyz
```

#### OPENAI_MODEL

**What**: Which OpenAI model to use
**Options**: `gpt-4`, `gpt-4-turbo-preview`, `gpt-3.5-turbo`
**Default**: `gpt-4`
**Cost**: gpt-4 most expensive, gpt-3.5-turbo cheapest
**Quality**: gpt-4 best, gpt-3.5-turbo good

```bash
OPENAI_MODEL=gpt-4                  # Best quality, slower
OPENAI_MODEL=gpt-4-turbo-preview    # Fast + good quality
OPENAI_MODEL=gpt-3.5-turbo          # Fast + cheap
```

#### OPENAI_TEMPERATURE

**What**: Creativity level for OpenAI
**Format**: Float 0.0-1.0
**Default**: `0.7`

```bash
OPENAI_TEMPERATURE=0.7
```

#### OPENAI_MAX_TOKENS

**What**: Maximum response length
**Format**: Integer 256-4096
**Default**: `1024`
**Higher**: Longer responses, more tokens used (more cost)

```bash
OPENAI_MAX_TOKENS=1024
```

### Anthropic Configuration

#### ANTHROPIC_API_KEY

**What**: Anthropic API key for Claude
**Format**: String starting with `sk-ant-`
**Required if**: `LLM_PROVIDER=anthropic`
**Secret**: YES
**Get**: https://console.anthropic.com/account/keys

```bash
ANTHROPIC_API_KEY=sk-ant-1234567890abcdefghijklmnopqrstuvwxyz
```

#### ANTHROPIC_MODEL

**What**: Which Claude model to use
**Options**: `claude-3-opus`, `claude-3-sonnet`, `claude-3-haiku`
**Default**: `claude-3-opus`
**Quality**: Opus > Sonnet > Haiku
**Cost**: Opus most expensive, Haiku cheapest

```bash
ANTHROPIC_MODEL=claude-3-opus        # Best quality
ANTHROPIC_MODEL=claude-3-sonnet      # Balanced
ANTHROPIC_MODEL=claude-3-haiku       # Fast + cheap
```

#### ANTHROPIC_MAX_TOKENS

**What**: Maximum response length
**Format**: Integer 256-200000
**Default**: `1024`

```bash
ANTHROPIC_MAX_TOKENS=1024
```

---

## Integration Credentials (Optional)

Add credentials to enable automatic task updates.

### Azure DevOps

#### AZURE_DEVOPS_ORG

**What**: Azure DevOps organization name
**Format**: String (from URL: dev.azure.com/{ORG})
**Example**: `my-organization`
**Get**: https://dev.azure.com

```bash
AZURE_DEVOPS_ORG=my-organization
```

#### AZURE_DEVOPS_PROJECT

**What**: Azure DevOps project name
**Format**: String
**Example**: `MyProject`
**Get**: From project URL or settings

```bash
AZURE_DEVOPS_PROJECT=MyProject
```

#### AZURE_DEVOPS_TOKEN

**What**: Personal Access Token for authentication
**Format**: String
**Secret**: YES
**Required**: Yes (if using Azure)
**Permissions**: Work Items (Read & Write)
**Get**: https://dev.azure.com

```bash
AZURE_DEVOPS_TOKEN=abcdefghijklmnopqrstuvwxyz123456
```

### GitHub

#### GITHUB_TOKEN

**What**: GitHub Personal Access Token
**Format**: String starting with `ghp_` or `github_pat_`
**Secret**: YES
**Required**: Yes (if using GitHub)
**Scopes**: `repo`, `read:org`
**Get**: https://github.com/settings/tokens

```bash
GITHUB_TOKEN=ghp_1234567890abcdefghijklmnopqrstuvwxyz
```

#### GITHUB_REPO

**What**: GitHub repository (owner/name)
**Format**: `owner/repo`
**Example**: `john-doe/my-project`

```bash
GITHUB_REPO=john-doe/my-project
```

### Jira

#### JIRA_HOST

**What**: Jira instance URL
**Format**: `https://your-domain.atlassian.net`
**Example**: `https://company.atlassian.net`

```bash
JIRA_HOST=https://company.atlassian.net
```

#### JIRA_USERNAME

**What**: Jira username or email
**Format**: String (usually email)
**Example**: `user@company.com`

```bash
JIRA_USERNAME=user@company.com
```

#### JIRA_API_TOKEN

**What**: Jira API token
**Format**: String
**Secret**: YES
**Get**: https://id.atlassian.com/manage-profile/security/api-tokens

```bash
JIRA_API_TOKEN=abcdefghijklmnopqrstuvwxyz123456
```

### Microsoft Teams

#### TEAMS_BOT_ID

**What**: Teams bot application ID
**Format**: UUID
**Required**: Yes (if using Teams)
**Get**: Azure Portal → App registrations

```bash
TEAMS_BOT_ID=123e4567-e89b-12d3-a456-426614174000
```

#### TEAMS_BOT_PASSWORD

**What**: Teams bot password/secret
**Format**: String
**Secret**: YES
**Get**: Azure Portal → App registrations → Certificates & secrets

```bash
TEAMS_BOT_PASSWORD=abcdefghijklmnopqrstuvwxyz123456
```

#### TEAMS_CHANNEL_ID

**What**: Teams channel to post to
**Format**: String (hex ID or email)
**Example**: `19:1234567890abcdef@thread.tacv2`

```bash
TEAMS_CHANNEL_ID=19:1234567890abcdef@thread.tacv2
```

### Microsoft Graph (Outlook)

#### MSGRAPH_CLIENT_ID

**What**: Azure AD app registration client ID
**Format**: UUID
**Get**: Azure Portal → App registrations → Overview

```bash
MSGRAPH_CLIENT_ID=123e4567-e89b-12d3-a456-426614174000
```

#### MSGRAPH_CLIENT_SECRET

**What**: Azure AD app registration secret
**Format**: String
**Secret**: YES
**Get**: Azure Portal → App registrations → Certificates & secrets

```bash
MSGRAPH_CLIENT_SECRET=abcdefghijklmnopqrstuvwxyz123456
```

#### MSGRAPH_TENANT_ID

**What**: Azure AD tenant ID
**Format**: UUID
**Get**: Azure Portal → Azure AD → Overview

```bash
MSGRAPH_TENANT_ID=123e4567-e89b-12d3-a456-426614174000
```

---

## Feature Flags

Enable/disable specific features.

### Commit Enhancement

#### COMMIT_ENHANCEMENT_ENABLED

**What**: Enable AI-powered commit message enhancement
**Format**: `true` or `false`
**Default**: `true`
**Requires**: LLM provider configured

```bash
COMMIT_ENHANCEMENT_ENABLED=true
```

#### COMMIT_MAX_ATTEMPTS

**What**: Max refinement attempts in interactive workflow
**Format**: Integer 1-10
**Default**: `5`

```bash
COMMIT_MAX_ATTEMPTS=5
```

#### COMMIT_CONTEXT_ENABLED

**What**: Include git context in enhancement
**Format**: `true` or `false`
**Default**: `true`

```bash
COMMIT_CONTEXT_ENABLED=true
```

### Work Updates

#### WORK_UPDATE_ENABLED

**What**: Enable work update prompting
**Format**: `true` or `false`
**Default**: `true`

```bash
WORK_UPDATE_ENABLED=true
```

#### WORK_UPDATE_NLP_ENABLED

**What**: Use NLP parsing for work updates
**Format**: `true` or `false`
**Default**: `true`
**Requires**: spaCy installed

```bash
WORK_UPDATE_NLP_ENABLED=true
```

#### WORK_UPDATE_CONTEXT_ENABLED

**What**: Add git context to work updates
**Format**: `true` or `false`
**Default**: `true`

```bash
WORK_UPDATE_CONTEXT_ENABLED=true
```

### Conflict Resolution

#### CONFLICT_RESOLUTION_ENABLED

**What**: Enable auto conflict detection/resolution
**Format**: `true` or `false`
**Default**: `true`

```bash
CONFLICT_RESOLUTION_ENABLED=true
```

#### CONFLICT_RESOLUTION_AUTO

**What**: Auto-resolve without asking
**Format**: `true` or `false`
**Default**: `false` (ask user)

```bash
CONFLICT_RESOLUTION_AUTO=false
```

#### CONFLICT_RESOLUTION_STRATEGY

**What**: Default resolution strategy
**Options**: `smart`, `both`, `ours`, `theirs`
**Default**: `smart`
**Note**: `smart` requires AI enabled

```bash
CONFLICT_RESOLUTION_STRATEGY=smart
```

### Report Generation

#### REPORT_GENERATION_ENABLED

**What**: Enable daily/weekly report generation
**Format**: `true` or `false`
**Default**: `true`

```bash
REPORT_GENERATION_ENABLED=true
```

#### REPORT_AI_ENHANCEMENT

**What**: Use AI for report summaries
**Format**: `true` or `false`
**Default**: `true`
**Requires**: LLM provider

```bash
REPORT_AI_ENHANCEMENT=true
```

### Learning

#### LEARNING_ENABLED

**What**: Enable AI learning from your communications
**Format**: `true` or `false`
**Default**: `false` (opt-in)

```bash
LEARNING_ENABLED=false
```

#### LEARNING_TEAMS_ENABLED

**What**: Learn from Teams chat
**Format**: `true` or `false`
**Default**: `false` (requires LEARNING_ENABLED=true)

```bash
LEARNING_TEAMS_ENABLED=false
```

#### LEARNING_OUTLOOK_ENABLED

**What**: Learn from Outlook emails
**Format**: `true` or `false`
**Default**: `false` (requires LEARNING_ENABLED=true)

```bash
LEARNING_OUTLOOK_ENABLED=false
```

---

## Scheduling

### Timer Triggers

#### WORK_UPDATE_TIMER_INTERVAL

**What**: Minutes between work update prompts
**Format**: Integer (minutes)
**Default**: `120` (2 hours)
**Example**: `60` for every hour, `240` for every 4 hours

```bash
WORK_UPDATE_TIMER_INTERVAL=120
```

#### REPORT_GENERATION_INTERVAL

**What**: Minutes between automatic reports
**Format**: Integer (minutes)
**Default**: `1440` (daily)
**Note**: Leave empty to disable automatic reports

```bash
REPORT_GENERATION_INTERVAL=1440     # Daily
# REPORT_GENERATION_INTERVAL=10080   # Weekly (7 days)
```

### Cron Expressions

For advanced scheduling:

#### WORK_UPDATE_SCHEDULE

**What**: Cron expression for work update prompts
**Format**: Cron format (5 fields)
**Example**: `0 9 * * 1-5` (9 AM weekdays)
**Note**: Overrides WORK_UPDATE_TIMER_INTERVAL if set

```bash
# Every 2 hours (120 minutes)
WORK_UPDATE_SCHEDULE=0 */2 * * *

# 9 AM, 1 PM, 5 PM on weekdays
WORK_UPDATE_SCHEDULE=0 9,13,17 * * 1-5
```

---

## Logging

### LOG_LEVEL

**What**: Verbosity of logging
**Options**: `debug`, `info`, `warn`, `error`
**Default**: `info`
**Note**: `debug` creates much larger log files

```bash
LOG_LEVEL=info              # Default
LOG_LEVEL=debug             # Verbose (for troubleshooting)
LOG_LEVEL=warn              # Less verbose
LOG_LEVEL=error             # Only errors
```

### LOG_FORMAT

**What**: Log message format
**Options**: `json`, `text`, `pretty`
**Default**: `text`

```bash
LOG_FORMAT=text             # Human readable
LOG_FORMAT=json             # Structured for parsing
```

---

## Docker Configuration

### DEVTRACK_DOCKER

**What**: Enable Docker mode
**Format**: `true` or `false`
**Default**: `false`
**Note**: Set automatically if running in container

```bash
DEVTRACK_DOCKER=false      # Local installation
DEVTRACK_DOCKER=true       # Docker installation
```

---

## Template Configuration

Here's a complete template with all variables:

```bash
# ============================================
# SYSTEM PATHS (REQUIRED - use absolute paths)
# ============================================
PROJECT_ROOT=/home/user/Documents/automation_tools
DEVTRACK_WORKSPACE=${PROJECT_ROOT}
DEVTRACK_HOME=${PROJECT_ROOT}/devtrack-bin
DATA_DIR=${PROJECT_ROOT}/Data
DATABASE_DIR=${DATA_DIR}/db
LOG_DIR=${DATA_DIR}/logs
PID_DIR=${DATA_DIR}/pid
REPORT_DIR=${DATA_DIR}/reports

# ============================================
# IPC CONFIGURATION
# ============================================
IPC_HOST=127.0.0.1
IPC_PORT=35893

# ============================================
# LLM/AI CONFIGURATION
# ============================================
LLM_PROVIDER=ollama
OLLAMA_URL=http://localhost:11434
OLLAMA_MODEL=mistral
OLLAMA_TEMPERATURE=0.7

# Optional: Fallback providers
# OPENAI_API_KEY=sk-...
# OPENAI_MODEL=gpt-4
# OPENAI_TEMPERATURE=0.7
# OPENAI_MAX_TOKENS=1024

# ANTHROPIC_API_KEY=sk-ant-...
# ANTHROPIC_MODEL=claude-3-opus
# ANTHROPIC_MAX_TOKENS=1024

# ============================================
# INTEGRATION CREDENTIALS (OPTIONAL)
# ============================================
# Azure DevOps
# AZURE_DEVOPS_ORG=my-org
# AZURE_DEVOPS_PROJECT=MyProject
# AZURE_DEVOPS_TOKEN=...

# GitHub
# GITHUB_TOKEN=ghp_...
# GITHUB_REPO=user/repo

# Jira
# JIRA_HOST=https://company.atlassian.net
# JIRA_USERNAME=user@company.com
# JIRA_API_TOKEN=...

# Teams
# TEAMS_BOT_ID=...
# TEAMS_BOT_PASSWORD=...
# TEAMS_CHANNEL_ID=...

# Microsoft Graph
# MSGRAPH_CLIENT_ID=...
# MSGRAPH_CLIENT_SECRET=...
# MSGRAPH_TENANT_ID=...

# ============================================
# FEATURE FLAGS
# ============================================
COMMIT_ENHANCEMENT_ENABLED=true
COMMIT_MAX_ATTEMPTS=5
COMMIT_CONTEXT_ENABLED=true

WORK_UPDATE_ENABLED=true
WORK_UPDATE_NLP_ENABLED=true
WORK_UPDATE_CONTEXT_ENABLED=true

CONFLICT_RESOLUTION_ENABLED=true
CONFLICT_RESOLUTION_AUTO=false
CONFLICT_RESOLUTION_STRATEGY=smart

REPORT_GENERATION_ENABLED=true
REPORT_AI_ENHANCEMENT=true

LEARNING_ENABLED=false
LEARNING_TEAMS_ENABLED=false
LEARNING_OUTLOOK_ENABLED=false

# ============================================
# SCHEDULING
# ============================================
WORK_UPDATE_TIMER_INTERVAL=120
REPORT_GENERATION_INTERVAL=1440

# ============================================
# LOGGING
# ============================================
LOG_LEVEL=info
LOG_FORMAT=text

# ============================================
# DOCKER
# ============================================
DEVTRACK_DOCKER=false
```

---

## Validation

Validate your .env file:

```bash
# Python validation script
uv run python validate_env_sample.py

# Manual validation
# Check paths exist
ls -d $PROJECT_ROOT
ls -d $DEVTRACK_WORKSPACE

# Check env is readable
cat .env | head

# Test critical variables
devtrack status
```

---

## Common Configuration Scenarios

### Minimal Setup (Local, Free)

```bash
PROJECT_ROOT=/home/user/automation_tools
DEVTRACK_WORKSPACE=${PROJECT_ROOT}
DATA_DIR=${PROJECT_ROOT}/Data
LLM_PROVIDER=ollama
OLLAMA_URL=http://localhost:11434
OLLAMA_MODEL=mistral
```

### Production Setup (Cloud AI, Full Integration)

```bash
PROJECT_ROOT=/opt/automation_tools
DEVTRACK_WORKSPACE=/var/git/my-project
DATA_DIR=/var/devtrack/data
DATABASE_DIR=/var/devtrack/db
LOG_DIR=/var/devtrack/logs

LLM_PROVIDER=openai
OPENAI_API_KEY=sk-...
OPENAI_MODEL=gpt-4

AZURE_DEVOPS_ORG=mycompany
AZURE_DEVOPS_PROJECT=CoreServices
AZURE_DEVOPS_TOKEN=...

GITHUB_TOKEN=ghp_...
GITHUB_REPO=mycompany/core-services

WORK_UPDATE_TIMER_INTERVAL=60
LEARNING_ENABLED=true
```

### Docker Setup (Cross-Platform)

```bash
PROJECT_ROOT=.
DEVTRACK_WORKSPACE=${PROJECT_ROOT}
DATA_DIR=${PROJECT_ROOT}/Data
DEVTRACK_DOCKER=true

LLM_PROVIDER=ollama
OLLAMA_URL=http://host.docker.internal:11434
OLLAMA_MODEL=mistral
```

---

## Changing Configuration

To change configuration after setup:

1. Stop daemon: `devtrack stop`
2. Edit .env: `nano .env`
3. Restart daemon: `devtrack start`

Some changes (like credentials) take effect immediately. Others (like log level) require restart.

---

**Need help?** See [Troubleshooting Guide](TROUBLESHOOTING.md) or [Installation Guide](INSTALLATION.md).
