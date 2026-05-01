---
name: alert_state_sqlite
description: SQLite fallback for alert poller delta tracking (last_checked per source) when MongoDB is unavailable; also fixed notification body dual-write
type: project
---

# SQLite Alert State Fallback

**Why:** Without MongoDB, alert pollers had no memory of `last_checked` timestamps. Every poll cycle would re-fetch from scratch, generating duplicate comment notifications on all 50 assigned issues.

**How to apply:** When working on the alert poller or adding new alert sources, use `_sqlite_load_last_checked` / `_sqlite_save_last_checked` as the fallback path when `store.is_available()` is False. The SQLite `alert_state` table is created automatically by the Go daemon on startup.

## Implementation

### Python (`backend/alert_poller.py`)

```python
def _sqlite_load_last_checked(user_id, source) -> Optional[datetime]
def _sqlite_save_last_checked(user_id, source, ts) -> None
```

Each poll function (`_poll_github`, `_poll_azure`, `_poll_jira`) now follows:
```python
if store.is_available():
    last_checked = await store.load_last_checked(user_id, source)
else:
    last_checked = _sqlite_load_last_checked(user_id, source)
# ... poll ...
if store.is_available():
    await store.save_last_checked(user_id, source, now)
else:
    _sqlite_save_last_checked(user_id, source, now)
```

### Go (`devtrack-bin/database.go`)

New table in schema:
```sql
CREATE TABLE IF NOT EXISTS alert_state (
    id           TEXT PRIMARY KEY,  -- "<user_id>:<source>"
    user_id      TEXT NOT NULL,
    source       TEXT NOT NULL,
    last_checked DATETIME NOT NULL,
    updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

New methods:
- `GetAlertLastChecked(userID, source string) (time.Time, bool, error)`
- `SetAlertLastChecked(userID, source string, ts time.Time) error` — upserts via `ON CONFLICT DO UPDATE`

### Also Fixed

`_write_notification_to_sqlite` now includes the `body` column (mapped from notification `summary` field). Previously the column was silently omitted, leaving `body=''` for all SQLite-written notifications.
