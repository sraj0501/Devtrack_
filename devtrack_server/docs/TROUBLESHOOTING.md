# DevTrack Troubleshooting Guide

Common issues and how to solve them.

---

## Configuration Issues

### "Configuration missing [VARIABLE_NAME]"

**Problem**: Required environment variable not set in `.env`.

**Why this happens**:
- `.env` file incomplete or uses old format
- Variable was accidentally deleted
- Upgrading from older version without updating .env

**Solution**:

1. Check which variable is missing:
```bash
# Error message tells you: Configuration missing IPC_CONNECT_TIMEOUT_SECS

# Check if variable exists in .env
grep "IPC_CONNECT_TIMEOUT_SECS" .env
# If no output, it's missing
```

2. Get correct value from `.env_sample`:
```bash
grep "IPC_CONNECT_TIMEOUT_SECS" .env_sample
# Output: IPC_CONNECT_TIMEOUT_SECS=5
```

3. Add to `.env`:
```bash
echo "IPC_CONNECT_TIMEOUT_SECS=5" >> .env
```

4. Restart daemon:
```bash
devtrack restart
```

**Common missing variables**:
- Timeouts: `IPC_CONNECT_TIMEOUT_SECS`, `HTTP_TIMEOUT_SHORT`, `HTTP_TIMEOUT`, `HTTP_TIMEOUT_LONG`
- Hosts: `OLLAMA_HOST`, `LMSTUDIO_HOST`
- Models: `GIT_SAGE_DEFAULT_MODEL`
- Prompts: `PROMPT_TIMEOUT_SIMPLE_SECS`, `PROMPT_TIMEOUT_WORK_SECS`, `PROMPT_TIMEOUT_TASK_SECS`
- LLM: `LLM_REQUEST_TIMEOUT_SECS`
- Sentiment: `SENTIMENT_ANALYSIS_WINDOW_MINUTES`

**Verification**:
```bash
# Check all 12 required variables are present
grep -E "IPC_CONNECT_TIMEOUT_SECS|HTTP_TIMEOUT_SHORT|HTTP_TIMEOUT|HTTP_TIMEOUT_LONG|IPC_RETRY_DELAY_MS|OLLAMA_HOST|LMSTUDIO_HOST|GIT_SAGE_DEFAULT_MODEL|PROMPT_TIMEOUT_SIMPLE_SECS|PROMPT_TIMEOUT_WORK_SECS|PROMPT_TIMEOUT_TASK_SECS|LLM_REQUEST_TIMEOUT_SECS" .env | wc -l
# Should output: 12 or higher
```

### "Configuration invalid: [VARIABLE] must be greater than 0"

**Problem**: Timeout or delay variable set to invalid value (0 or negative).

**Example**:
```
Configuration invalid: HTTP_TIMEOUT must be greater than 0
Set in .env: HTTP_TIMEOUT=30
```

**Solution**:

1. Edit `.env` and fix value:
```bash
nano .env
# Change: HTTP_TIMEOUT=0
# To:     HTTP_TIMEOUT=30
```

2. Verify value is valid:
```bash
# Timeouts should be positive integers (seconds)
HTTP_TIMEOUT=30        # Good ✓
HTTP_TIMEOUT=0         # Bad - will error
HTTP_TIMEOUT=-1        # Bad - will error

# Delays should be positive integers (milliseconds)
IPC_RETRY_DELAY_MS=2000    # Good ✓
IPC_RETRY_DELAY_MS=0       # Bad - will error
```

3. Restart:
```bash
devtrack restart
```

### "Configuration invalid: [HOST] must be a valid URL"

**Problem**: Host variable (OLLAMA_HOST, LMSTUDIO_HOST) not a valid URL.

**Examples**:
```
Configuration invalid: OLLAMA_HOST must be a valid URL
Set in .env: OLLAMA_HOST=http://localhost:11434

# Bad formats:
OLLAMA_HOST=localhost:11434          # Missing http://
OLLAMA_HOST=11434                    # Missing host
OLLAMA_HOST=192.168.1.100            # Missing http://
```

**Solution**:

1. Check format in `.env`:
```bash
# Must start with http:// or https://
OLLAMA_HOST=http://localhost:11434          # Good ✓
OLLAMA_HOST=http://192.168.1.100:11434      # Good ✓
OLLAMA_HOST=https://ollama.example.com:443  # Good ✓

# Must end with port number
OLLAMA_HOST=http://localhost:11434          # Good ✓
OLLAMA_HOST=http://localhost                # Bad - missing port
```

2. Edit and fix:
```bash
nano .env
# Change: OLLAMA_HOST=localhost:11434
# To:     OLLAMA_HOST=http://localhost:11434
```

3. Restart:
```bash
devtrack restart
```

### "Daemon fails at startup with configuration error"

**Problem**: Multiple configuration issues preventing startup.

**Diagnosis**:

1. Check full error message:
```bash
devtrack start
# Look at error output - it tells you which variable is the problem
```

2. Check daemon log:
```bash
tail -50 Data/logs/daemon.log
# Will show configuration errors in detail
```

3. Validate all configuration:
```bash
# Copy .env_sample to reference
diff .env .env_sample
# Shows which variables are missing or different
```

**Solution**:

1. Restore from sample:
```bash
# Backup current
cp .env .env.broken

# Start fresh
cp .env_sample .env

# Edit with your values
nano .env
```

2. Update paths to match your system:
```bash
# Change these to YOUR paths
PROJECT_ROOT=/your/absolute/path/automation_tools
DEVTRACK_WORKSPACE=/your/git/repo
DATA_DIR=/your/absolute/path/Data
```

3. Try starting again:
```bash
devtrack start
devtrack status
```

---

## Installation Issues

### "devtrack: command not found"

**Problem**: Binary not in PATH or not installed properly.

**Solutions**:

1. Check if binary exists:
```bash
ls -la ~/.local/bin/devtrack
```

2. Add to PATH if missing:
```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

3. Or use full path:
```bash
~/.local/bin/devtrack status
```

4. If binary doesn't exist, rebuild:
```bash
cd devtrack-bin
go build -o devtrack .
cp devtrack ~/.local/bin/
chmod +x ~/.local/bin/devtrack
```

---

### "Python version mismatch" / "spaCy incompatible with Python 3.14"

**Problem**: Using Python 3.14+ which spaCy doesn't support yet.

**Solutions**:

1. Check current Python version:
```bash
python3 --version
```

2. If 3.14+, install Python 3.13:
```bash
# macOS
brew install python@3.13

# Ubuntu/Debian
sudo apt install python3.13

# Then use explicitly
uv --python python3.13 sync
```

3. Or let uv auto-downgrade:
```bash
uv sync  # Should auto-use 3.13 from pyproject.toml
```

---

### ".env not found" or "Missing required environment variables"

**Problem**: .env configuration not found or incomplete.

**Solutions**:

1. Create .env if missing:
```bash
cp .env_sample .env
```

2. Edit .env with your paths:
```bash
nano .env
```

3. Verify required variables:
```bash
grep "PROJECT_ROOT\|DEVTRACK_WORKSPACE" .env
```

4. Make sure paths are absolute (not relative):
```bash
# WRONG:
PROJECT_ROOT=./automation_tools

# RIGHT:
PROJECT_ROOT=/home/user/automation_tools
```

---

### "spaCy NLP model not found"

**Problem**: `ModuleNotFoundError: No module named 'en_core_web_sm'`

**Solutions**:

1. Download the model:
```bash
uv run python -m spacy download en_core_web_sm
```

2. Verify installation:
```bash
uv run python -c "import spacy; nlp = spacy.load('en_core_web_sm'); print('OK')"
```

3. Check where it's installed:
```bash
python -m spacy info
```

---

## Daemon Issues

### "DevTrack daemon is not running" or won't start

**Problem**: Daemon fails to start or crashes immediately.

**Solutions**:

1. Check for errors:
```bash
devtrack start
# Look for error message
```

2. View full logs:
```bash
tail -50 Data/logs/daemon.log
```

3. Check if port is in use:
```bash
lsof -i :35893  # Default IPC port
```

If something is using the port:
```bash
# Kill process using port
lsof -ti:35893 | xargs kill -9

# Or change port in .env
IPC_PORT=35894

# Restart
devtrack restart
```

4. Check .env is valid:
```bash
cat .env | grep "PROJECT_ROOT\|DEVTRACK_WORKSPACE"
# Should show absolute paths, not empty
```

5. Try manual start with debugging:
```bash
cd devtrack-bin
./devtrack start --verbose
```

---

### Daemon crashes immediately after startup

**Problem**: Daemon starts but dies after a few seconds.

**Solutions**:

1. Check logs for crash reason:
```bash
tail -100 Data/logs/daemon.log | grep -i "error\|panic"
```

2. Check database is not corrupted:
```bash
# Backup database
cp Data/db/devtrack.db Data/db/devtrack.db.bak

# Remove and let daemon recreate
rm Data/db/devtrack.db

# Start again
devtrack start
```

3. Check Go binary compatibility:
```bash
file devtrack-bin/devtrack  # Should show "Mach-O 64-bit" on macOS, "ELF 64-bit" on Linux

# Rebuild if wrong architecture
cd devtrack-bin
go build -o devtrack .
```

4. Check system resources:
```bash
# Free up disk space
df -h

# Check memory
free -h  # Linux
vm_stat  # macOS
```

---

### "IPC connection failed" or "Failed to connect to daemon"

**Problem**: Python bridge can't connect to Go daemon.

**Solutions**:

1. Verify daemon is running:
```bash
devtrack status
ps aux | grep devtrack
```

2. Check IPC configuration matches in .env:
```bash
grep "IPC_HOST\|IPC_PORT" .env
```

3. Verify port is listening:
```bash
netstat -tuln | grep 35893  # Linux
netstat -an | grep 35893     # macOS
```

4. Check firewall isn't blocking:
```bash
# macOS - check if in firewall exceptions
sudo launchctl list com.apple.security.firewall

# Linux - check UFW if installed
sudo ufw status
sudo ufw allow 35893

# Windows - PowerShell
netsh advfirewall firewall add rule name="DevTrack IPC" dir=in action=allow protocol=tcp localport=35893
```

5. Try different port:
```bash
# Edit .env
IPC_PORT=35894

# Restart
devtrack restart
```

---

## Git Monitoring Issues

### Git commits not detected

**Problem**: DevTrack doesn't react to new commits.

**Solutions**:

1. Check daemon is running:
```bash
devtrack status
```

2. Verify correct repository is being monitored:
```bash
grep "DEVTRACK_WORKSPACE" .env

# Should match where you're making commits
pwd
```

3. Check git monitor is active:
```bash
tail Data/logs/daemon.log | grep -i "monitor\|watching\|git"
```

4. Test with a new commit:
```bash
cd /path/to/monitored/repo
echo "test" > test.txt
git add test.txt
git commit -m "Test commit"

# Check logs immediately
tail Data/logs/daemon.log | tail -20
```

5. Check if .git directory exists:
```bash
ls -la .git/
```

6. Try forcing a trigger:
```bash
devtrack force-trigger
# This should prompt for work update even if commits not detected
```

---

### "Repository not found" or similar Git errors

**Problem**: Git monitor reports errors accessing repo.

**Solutions**:

1. Verify path is correct:
```bash
echo $DEVTRACK_WORKSPACE
cd $DEVTRACK_WORKSPACE
pwd
```

2. Check repo is valid:
```bash
git status
# Should show working tree, not errors
```

3. Check .git permissions:
```bash
ls -la .git
# Should be readable by your user
chmod -R u+rwx .git
```

4. Verify git is installed:
```bash
git --version
```

5. Try in different directory:
```bash
# Test monitoring a different repo
mkdir test-repo
cd test-repo
git init
git config user.email "test@example.com"
git config user.name "Test User"
echo "test" > test.txt
git add test.txt
git commit -m "Initial commit"

# Change DEVTRACK_WORKSPACE in .env to test-repo
# Restart daemon
# Try again
```

---

## AI & LLM Issues

### "Ollama not responding" or AI enhancement disabled

**Problem**: AI features aren't working, Ollama connection failed.

**Solutions**:

1. Check if Ollama is running:
```bash
curl http://localhost:11434/api/tags
```

If curl fails, Ollama isn't running:
```bash
# Start Ollama
ollama serve

# Or use as service
brew services start ollama    # macOS
sudo systemctl start ollama   # Linux
```

2. Verify Ollama port and URL:
```bash
# Edit .env
OLLAMA_URL=http://localhost:11434
OLLAMA_MODEL=mistral

# Or if Ollama on different machine
OLLAMA_URL=http://192.168.1.100:11434
```

3. Check if model is downloaded:
```bash
curl http://localhost:11434/api/tags | grep mistral
```

If not downloaded:
```bash
ollama pull mistral
ollama pull llama2  # or another model
```

4. Test Ollama directly:
```bash
ollama run mistral "test prompt"
```

5. Check Ollama logs:
```bash
# macOS
tail -f /var/log/ollama.log

# Linux
journalctl -u ollama -f

# Windows
# Check Ollama app logs
```

6. Try different model:
```bash
# Edit .env
OLLAMA_MODEL=llama2

# Restart
devtrack restart
```

---

### "OpenAI API key invalid" or "Authentication failed"

**Problem**: OpenAI integration not working.

**Solutions**:

1. Verify API key format:
```bash
echo $OPENAI_API_KEY
# Should start with "sk-"
```

2. Check API key is valid:
```bash
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"
# Should return list of models, not error
```

3. Check API key has correct permissions:
- Go to https://platform.openai.com/api-keys
- Delete the key and create a new one
- Ensure it has 'read' and 'write' permissions

4. Check account has credits:
```bash
# Go to: https://platform.openai.com/account/billing/overview
# Should show available balance
```

5. Check rate limits not exceeded:
```bash
# Go to: https://platform.openai.com/account/rate-limits
# May be temporarily rate-limited
```

6. Add to .env correctly:
```bash
OPENAI_API_KEY=sk-...your-full-key...
OPENAI_MODEL=gpt-4

# Restart
devtrack restart
```

---

### AI enhancement "takes forever" or hangs

**Problem**: Commit enhancement or report generation stalls.

**Solutions**:

1. Check Ollama isn't overloaded:
```bash
# Ollama on CPU will be slow
# Check CPU usage
top  # or Activity Monitor on macOS

# If at 100%, Ollama is busy
# Wait or restart Ollama
```

2. Check network if using cloud API:
```bash
# Test internet
ping google.com
curl -s https://api.openai.com/v1/models -H "Authorization: Bearer $OPENAI_API_KEY" | head
```

3. Check response timeout in code:
```bash
# Edit .env if available
LLM_TIMEOUT=30  # seconds

# Or in backend/llm/provider.py
# May need to increase timeout value
```

4. Try with shorter prompt:
```bash
# Smaller commits = faster enhancement
# Very large diffs take longer

# Check what's being sent:
tail Data/logs/python_bridge.log | grep "enhancement\|prompt"
```

5. Try local Ollama with smaller model:
```bash
ollama pull orca-mini
OLLAMA_MODEL=orca-mini
devtrack restart
```

---

## Work Update & Parsing Issues

### Work updates not being parsed correctly

**Problem**: Natural language parsing doesn't extract correct task/time.

**Solutions**:

1. Check NLP model is loaded:
```bash
uv run python -c "import spacy; nlp = spacy.load('en_core_web_sm'); print(nlp('test'))"
```

2. Test parsing directly:
```bash
uv run python << 'EOF'
from backend.nlp_parser import parse_update

result = parse_update("Working on PR #42 fixing auth bug (2h)")
print(result)
EOF
```

3. Try simpler format:
```bash
# Instead of: "Working on PR #42 fixing auth bug (2h)"
# Try: "PR #42 - 2h"

# Check logs to see what was extracted
tail Data/logs/python_bridge.log | grep -i "parse\|npl"
```

4. Check git context is helping:
```bash
# Work updates can also extract from branch name
git branch  # Should show feature branch

# If branch is named like "feature/pr-42-auth"
# Parser will auto-detect PR #42
```

---

### Timer trigger not firing / "Prompt never shows"

**Problem**: Scheduled work update prompts don't appear.

**Solutions**:

1. Check scheduler is running:
```bash
tail Data/logs/daemon.log | grep -i "scheduler\|cron"
```

2. Check scheduler interval:
```bash
grep -i "timer\|schedule" .env
# Look for WORK_UPDATE_TIMER_INTERVAL=120
```

3. Force trigger to test:
```bash
devtrack force-trigger
# Should show prompt immediately
```

4. If force-trigger works but scheduled doesn't:
```bash
# Check daemon log for scheduler errors
tail -100 Data/logs/daemon.log | grep -i "error\|trigger\|scheduler"

# Check cron format if custom
# Should be: "*/120 * * * *" for every 120 minutes
```

5. Check TUI dependencies:
```bash
# Try showing prompt manually
uv run python << 'EOF'
from backend.user_prompt import show_work_update_prompt
show_work_update_prompt()
EOF
```

---

## Integration Issues

### "Failed to update Azure DevOps" or similar

**Problem**: Project management system updates not working.

**Solutions**:

1. Verify credentials:
```bash
grep "AZURE_DEVOPS\|GITHUB\|JIRA" .env
# Should have tokens/passwords set
```

2. Test credential validity:
```bash
# For Azure DevOps
curl -u :{PAT_TOKEN} https://dev.azure.com/{ORG}/_apis/projects

# For GitHub
curl -H "Authorization: token {TOKEN}" https://api.github.com/user
```

3. Check permissions:
- Azure DevOps: Token needs 'Work Items Read & Write'
- GitHub: Token needs 'repo', 'read:org' scopes
- Jira: User must have 'Edit' permission on project

4. Verify organization/project names:
```bash
grep "ORG\|PROJECT\|REPO" .env
# Should match your actual org/project names
```

5. Check network connectivity:
```bash
curl https://dev.azure.com
curl https://api.github.com
curl https://jira.example.com
```

6. Review logs for specific errors:
```bash
tail Data/logs/python_bridge.log | grep -i "azure\|github\|jira\|api\|error"
```

---

## Performance Issues

### "DevTrack uses too much CPU/Memory"

**Problem**: Daemon or Python bridge hogging resources.

**Solutions**:

1. Check what's using resources:
```bash
top        # Linux/macOS
taskmgr    # Windows

ps aux | grep devtrack
ps aux | grep python_bridge
```

2. If Go daemon using CPU:
```bash
# Check for git monitor issues
tail Data/logs/daemon.log | grep -i "monitor"

# Try different polling interval
# Edit code: git_monitor.go
# Increase debounce time
```

3. If Python using memory:
```bash
# Check what's loaded
tail Data/logs/python_bridge.log | head -20

# If NLP model is issue, unload when not needed
# Or use smaller model: orca-mini instead of full model
```

4. Check database size:
```bash
ls -lh Data/db/devtrack.db

# If very large, archive old records
# Or delete: rm Data/db/devtrack.db (daemon will recreate)
```

5. Reduce logging:
```bash
# Edit .env
LOG_LEVEL=info  # Instead of debug

# Restart
devtrack restart
```

---

## Data & Database Issues

### "Database locked" or "Cannot write to database"

**Problem**: SQLite database is locked or corrupted.

**Solutions**:

1. Stop daemon:
```bash
devtrack stop
```

2. Check database:
```bash
ls -la Data/db/devtrack.db
# Should have read/write permissions
```

3. Fix permissions:
```bash
chmod 644 Data/db/devtrack.db
chmod 755 Data/db/
```

4. Verify database integrity:
```bash
sqlite3 Data/db/devtrack.db "PRAGMA integrity_check;"
```

5. If corrupted, restore from backup or recreate:
```bash
# Backup corrupt database
mv Data/db/devtrack.db Data/db/devtrack.db.corrupt

# Start daemon - it will recreate database
devtrack start
```

---

### "Lost work / history not saved"

**Problem**: Work updates or commit history disappeared.

**Solutions**:

1. Check if data exists:
```bash
# Lists trigger history
sqlite3 Data/db/devtrack.db "SELECT count(*) FROM triggers;"
```

2. View recent entries:
```bash
sqlite3 Data/db/devtrack.db << 'EOF'
.mode column
SELECT * FROM triggers ORDER BY trigger_time DESC LIMIT 10;
EOF
```

3. Backup current database:
```bash
cp Data/db/devtrack.db Data/db/devtrack.db.backup
```

4. Check logs for errors:
```bash
tail Data/logs/daemon.log | grep -i "error\|database"
tail Data/logs/python_bridge.log | grep -i "error\|database"
```

---

## Logging & Debugging

### "Can't find useful error messages in logs"

**Problem**: Logs don't have enough detail.

**Solutions**:

1. Enable debug logging:
```bash
# Edit .env
LOG_LEVEL=debug

# Restart
devtrack restart

# Now logs will be more verbose
```

2. Follow logs in real-time:
```bash
devtrack logs -f

# Or manually
tail -f Data/logs/daemon.log
tail -f Data/logs/python_bridge.log
```

3. Grep for specific issues:
```bash
# Find errors
tail Data/logs/*.log | grep -i "error\|fatal\|panic"

# Find NLP issues
tail Data/logs/*.log | grep -i "npl\|parse"

# Find API issues
tail Data/logs/*.log | grep -i "api\|http\|request"

# Find AI issues
tail Data/logs/*.log | grep -i "llm\|ollama\|openai"
```

4. Create minimal reproducible case:
```bash
# Simple test to isolate issue
uv run python << 'EOF'
import logging
logging.basicConfig(level=logging.DEBUG)

# Your test code here
from backend.nlp_parser import parse_update
result = parse_update("PR #42 (1h)")
print(result)
EOF
```

---

## Getting More Help

If you can't solve it:

1. **Check existing issues**: https://github.com/sraj0501/automation_tools/issues
2. **See Known Issues**: [KNOWN_ISSUES.md](../KNOWN_ISSUES.md)
3. **Review Architecture**: [CLAUDE.md](../CLAUDE.md) - Debugging patterns section
4. **Check Phase 3 Verification**: [PHASE_3_VERIFICATION.md](../PHASE_3_VERIFICATION.md)
5. **Create a new issue** with:
   - Error message
   - Steps to reproduce
   - Output of `devtrack status`
   - Last 50 lines of logs
   - Your OS and Python version

---

## Quick Diagnosis

Run this to diagnose most issues:

```bash
#!/bin/bash
echo "=== DevTrack Diagnosis ==="
echo ""
echo "Go daemon:"
devtrack version
devtrack status
echo ""

echo "Python:"
python3 --version
echo ""

echo "NLP model:"
uv run python -c "import spacy; nlp = spacy.load('en_core_web_sm'); print('OK')" || echo "MISSING"
echo ""

echo "Ollama:"
curl -s http://localhost:11434/api/tags | jq .models[0] || echo "NOT RUNNING"
echo ""

echo "Git:"
cd $DEVTRACK_WORKSPACE && git status | head -5
echo ""

echo "Database:"
ls -lh Data/db/devtrack.db
sqlite3 Data/db/devtrack.db "PRAGMA integrity_check;" | head
echo ""

echo ".env:"
cat .env | head -10
```

Save as `diagnose.sh` and run it:
```bash
chmod +x diagnose.sh
./diagnose.sh
```

---

**Still stuck?** Check the [Documentation Index](INDEX.md) for more guides or reach out on GitHub.
