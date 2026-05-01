# DevTrack Usage Guide - How to Track Your Code

Once DevTrack is installed and running, it automatically tracks your coding activity. Here's how it works and how to use it effectively.

---

## How DevTrack Tracks Your Code

### Automatic Tracking
DevTrack monitors your Git activity in real-time. Whenever you **commit code**, the daemon:

1. **Detects the commit** (via fsnotify - instant detection)
2. **Parses the commit message** with NLP (spaCy)
3. **Extracts task information**:
   - Ticket IDs (#123, PROJ-456, etc.)
   - Action verbs (fixed, implemented, started, etc.)
   - Time estimates/spent
   - Status (completed, in_progress, etc.)
4. **Logs to database** (SQLite at `~/.devtrack/devtrack.db`)
5. **Sends to APIs** (Azure DevOps, GitHub - if configured)

**You don't need to do anything special** - just make commits as normal!

---

## Writing Good Commit Messages for Tracking

The better your commit messages, the more DevTrack can extract. Here are patterns that work best:

### ✅ Best Practices

#### Format: `Action on #TICKET - Description (time)`

**Examples:**
```bash
git commit -m "Fixed #PROJ-456 - User authentication bug (2h)"
git commit -m "Implemented #123 - Payment gateway integration (4.5h)"
git commit -m "Started working on #TASK-789 - API documentation"
git commit -m "Refactored #456 - Database query optimization (1.5h)"
git commit -m "Completed #PROJ-123 - User dashboard feature (6h)"
```

### What DevTrack Extracts

| Element | Examples | Usage |
|---------|----------|-------|
| **Ticket ID** | `#123`, `PROJ-456`, `TASK-789` | Links to project management |
| **Action Verb** | `Fixed`, `Implemented`, `Started`, `Refactored` | Determines status |
| **Description** | `User authentication bug` | Task description |
| **Time** | `(2h)`, `(4.5h)`, `(30m)` | Time tracking |

### Status Detection

Action verbs are mapped to task statuses:

| Action Verbs | Status |
|--------------|--------|
| Fixed, Completed, Finished, Resolved, Closed | `completed` |
| Started, Beginning, Initiating | `in_progress` |
| Working, Implementing, Refactoring, Updating | `in_progress` |
| Testing, Debugging, Reviewing | `in_progress` |

---

## Daily Workflow

### 1. **Morning: Start the Daemon**
```bash
# Option A: Use the script (from project root)
cd ~/Documents/GitHub/automation_tools
./run_devtrack_local.sh

# Option B: Start manually from your project
cd ~/Documents/GitHub/your-project
devtrack start &
disown

# Verify it's running
devtrack status
```

### 2. **During Work: Code Normally**
```bash
# Work on your code as usual
vim src/api.py
npm test
# ... your normal workflow ...

# Make meaningful commits with task info
git add src/api.py
git commit -m "Fixed #API-123 - Rate limiting on /users endpoint (1.5h)"

# DevTrack automatically detects and processes this!
```

### 3. **Check What Was Tracked**
```bash
# View recent logs
tail -50 ~/.devtrack/daemon.log

# Look for:
# - 🎯 COMMIT TRIGGER
# - 📝 Parsing commit message with NLP...
# - Parsed result: ticket_id, action_verb, status, etc.
```

### 4. **End of Day: View Summary**
```bash
# Database statistics
devtrack db-stats

# Recent activity logs
devtrack logs

# Check specific trigger in logs
grep "COMMIT TRIGGER" ~/.devtrack/daemon.log
```

---

## Viewing Tracked Data

### Check Logs (Detailed)
```bash
# Watch logs in real-time
tail -f ~/.devtrack/daemon.log

# Filter for commit triggers only
grep -A 10 "COMMIT TRIGGER" ~/.devtrack/daemon.log

# Filter for NLP parsing results
grep -A 5 "Parsed result" ~/.devtrack/daemon.log

# Show last 10 commits tracked
grep "commit_hash" ~/.devtrack/daemon.log | tail -10
```

### Database Queries
```bash
# If you have sqlite3 installed:
sqlite3 ~/.devtrack/devtrack.db

# Useful queries:
SELECT * FROM triggers ORDER BY created_at DESC LIMIT 10;
SELECT commit_message, created_at FROM triggers WHERE trigger_type='commit';
SELECT COUNT(*) FROM triggers WHERE DATE(created_at) = DATE('now');
```

### CLI Commands
```bash
# Current daemon status
devtrack status

# Database statistics
devtrack db-stats

# Recent daemon logs (last 50 lines)
devtrack logs

# Help and all commands
devtrack help
```

---

## Real-World Examples

### Example 1: Feature Development
```bash
# Day 1: Start new feature
cd ~/Documents/GitHub/my-project
git checkout -b feature/user-profiles

# Make changes
vim src/profiles.py
git commit -m "Started #FEAT-456 - User profile page implementation"

# Continue working
vim src/profiles.py
git commit -m "Working on #FEAT-456 - Added profile data model (2h)"

# Check tracking
tail -30 ~/.devtrack/daemon.log | grep "FEAT-456"
```

**DevTrack extracts:**
- Ticket: `FEAT-456`
- Status: `in_progress` (from "Started" and "Working")
- Time: `2h` total tracked

### Example 2: Bug Fix
```bash
# Fix critical bug
vim src/auth.py
git commit -m "Fixed #BUG-789 - Login redirect vulnerability (3h)"

# Check parsing immediately
tail -40 ~/.devtrack/daemon.log
```

**DevTrack extracts:**
- Ticket: `BUG-789`
- Status: `completed` (from "Fixed")
- Time: `3h`
- Action: `fixed`

### Example 3: Multiple Tasks
```bash
# Morning work session
git commit -m "Implemented #TASK-111 - REST API endpoints (2.5h)"

# Afternoon work
git commit -m "Fixed #BUG-222 - Database migration script (1h)"

# End of day review
git commit -m "Completed #TASK-333 - API documentation (1.5h)"

# View all today's work
grep "COMMIT TRIGGER" ~/.devtrack/daemon.log | grep "$(date +%Y-%m-%d)"
```

---

## Advanced Usage

### Configure Work Hours (Auto-pause outside work hours)
```bash
devtrack config set work-hours "09:00-17:00"
```

### Adjust Trigger Interval (Timer-based prompts)
```bash
# Prompt every 2 hours instead of default 3
devtrack config set interval 120
```

### Pause/Resume Scheduler
```bash
# Pause scheduled prompts (keeps git monitoring)
devtrack pause

# Resume
devtrack resume
```

### Force Immediate Trigger
```bash
# Manually trigger without waiting for commit
devtrack force-trigger
```

### Skip Next Scheduled Trigger
```bash
# Skip the next timer trigger once
devtrack skip-next
```

---

## Monitoring and Debugging

### Is DevTrack Running?
```bash
# Quick check
devtrack status

# Process check
ps aux | grep devtrack
ps aux | grep python_bridge.py

# Port check
lsof -i :35893  # IPC port
```

### Check What Repository is Being Monitored
```bash
# Look in logs for "Started monitoring Git repository"
grep "Git repository" ~/.devtrack/daemon.log

# Should show the directory where you started the daemon
```

### Verify NLP Parser is Working
```bash
# Check logs for spaCy model loading
grep -i "spacy\|nlp" ~/.devtrack/daemon.log

# Should see:
# - Loaded spaCy model: en_core_web_sm
# - ✓ NLP parser initialized
```

### Test Commit Detection
```bash
# Make a test commit
echo "test" >> README.md
git add README.md
git commit -m "Testing #TEST-123 - DevTrack commit detection (0.5h)"

# Check logs immediately (within 2-3 seconds)
tail -50 ~/.devtrack/daemon.log

# Should see:
# - New commit detected (fsnotify)
# - 🎯 COMMIT TRIGGER
# - 📝 Parsing commit message with NLP...
# - Parsed result: {...}
```

---

## Common Issues

### ❌ Commits Not Being Detected

**Check if daemon started in correct repository:**
```bash
# Daemon must be started from the repository you want to monitor
cd ~/Documents/GitHub/your-project  # ← Important!
devtrack start &
```

**Check if daemon is running:**
```bash
devtrack status
# Should show: Status: ✅ RUNNING
```

**Check logs for errors:**
```bash
tail -100 ~/.devtrack/daemon.log | grep -i error
```

### ❌ NLP Not Extracting Ticket IDs

**Supported formats:**
- `#123` - Simple number
- `PROJ-456` - Project prefix with number
- `TASK-789` - Any word prefix with dash and number

**Not supported yet:**
- `ticket 123` (no # or prefix)
- `Story #123` (keyword before #)

**Check NLP parsing output:**
```bash
grep "Parsed result" ~/.devtrack/daemon.log | tail -5
```

### ❌ Time Not Being Extracted

**Supported formats:**
- `(2h)` or `(2.5h)` - Hours
- `(30m)` or `(45min)` - Minutes
- `2h 30m` - Combined

**Must be in parentheses at end of message:**
```bash
# ✅ Good
"Fixed bug (2h)"

# ❌ Won't parse
"Spent 2h fixing bug"
"Fixed bug - 2 hours"
```

---

## Best Practices

### 1. **Commit Often with Good Messages**
Small, frequent commits with descriptive messages work better than large commits:

```bash
# ✅ Good: Specific, trackable
git commit -m "Implemented #API-456 - User authentication endpoint (1.5h)"
git commit -m "Fixed #API-456 - Add JWT token validation (0.5h)"
git commit -m "Tested #API-456 - Authentication flow (1h)"

# ❌ Less useful: Vague, no task info
git commit -m "various fixes"
git commit -m "work in progress"
```

### 2. **Always Include Ticket IDs**
Even if working without a ticketing system, use consistent IDs:

```bash
git commit -m "Implemented #MYPROJ-001 - Initial setup"
git commit -m "Fixed #MYPROJ-002 - Build configuration"
```

### 3. **Track Time Accurately**
Include time spent to build accurate work logs:

```bash
git commit -m "Fixed #BUG-123 - Memory leak (3h)"  # ← Time helps reporting
```

### 4. **Use Consistent Action Verbs**
Helps DevTrack determine task status correctly:

```bash
# Starting work
git commit -m "Started #TASK-123 - Description"

# During work
git commit -m "Working on #TASK-123 - Description (2h)"

# Finishing
git commit -m "Completed #TASK-123 - Description (1h)"
```

### 5. **Review Logs Periodically**
Check that DevTrack is parsing correctly:

```bash
# Every few commits, verify parsing worked
tail -50 ~/.devtrack/daemon.log | grep -A 10 "COMMIT TRIGGER"
```

---

## Integration with Other Tools

### Azure DevOps (Coming Soon)
Once configured, DevTrack will automatically:
- Update work item status
- Add time tracking entries
- Link commits to work items

### GitHub Issues (Coming Soon)
- Link commits to issues
- Update issue comments
- Track time per issue

### Email Reports (Coming Soon)
```bash
# Generate daily summary
devtrack send-summary manager@company.com
```

---

## Tips for Maximum Value

1. **Start daemon in your main working repository**
2. **Keep it running during work hours**
3. **Write descriptive commit messages with ticket IDs**
4. **Include time estimates/spent**
5. **Review logs weekly to see your patterns**
6. **Use consistent ticket ID format across projects**

---

## Quick Reference Card

```bash
# SETUP
./run_devtrack_local.sh              # Start with validation
devtrack start &                 # Manual start

# DAILY USE
devtrack status                  # Check running state
tail -f ~/.devtrack/daemon.log       # Watch activity

# COMMIT FORMAT
git commit -m "Action #TICKET - Description (time)"

# EXAMPLES
git commit -m "Fixed #PROJ-456 - Bug description (2h)"
git commit -m "Implemented #123 - Feature name (3.5h)"
git commit -m "Started #TASK-789 - New work item"

# VIEW DATA
devtrack logs                    # Recent logs
devtrack db-stats                # Database stats
grep "COMMIT TRIGGER" ~/.devtrack/daemon.log

# CONTROL
devtrack pause                   # Pause scheduler
devtrack resume                  # Resume
devtrack restart                 # Restart daemon
devtrack stop                    # Stop completely
```

---

## Getting Help

- **Issue not listed here?** Check [LOCAL_SETUP.md](LOCAL_SETUP.md) troubleshooting section
- **Feature requests?** Open a GitHub issue
- **Questions?** Check the [Wiki](wiki/index.html)

---

**Happy tracking!** 🚀

DevTrack works best when you forget it's running - just commit your code naturally with good messages, and it handles the rest.
