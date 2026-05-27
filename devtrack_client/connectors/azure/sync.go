package azure

import (
	"database/sql"
	"fmt"
	"time"
)

const createAzureTable = `
CREATE TABLE IF NOT EXISTS azure_workitems (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	work_item_id   INTEGER NOT NULL UNIQUE,
	title          TEXT NOT NULL,
	state          TEXT,
	assigned_to    TEXT,
	work_item_type TEXT,
	url            TEXT,
	updated_at     TEXT,
	synced_at      DATETIME
)`

// Sync fetches all assigned open work items and upserts them into the azure_workitems SQLite table.
func Sync(db *sql.DB) error {
	if _, err := db.Exec(createAzureTable); err != nil {
		return fmt.Errorf("azure sync: create table: %w", err)
	}

	items, err := ListWorkItems()
	if err != nil {
		return fmt.Errorf("azure sync: list work items: %w", err)
	}

	now := time.Now().UTC()
	upserted := 0
	for _, item := range items {
		_, err := db.Exec(`
			INSERT INTO azure_workitems (work_item_id, title, state, assigned_to, work_item_type, url, updated_at, synced_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(work_item_id) DO UPDATE SET
				title          = excluded.title,
				state          = excluded.state,
				assigned_to    = excluded.assigned_to,
				work_item_type = excluded.work_item_type,
				url            = excluded.url,
				updated_at     = excluded.updated_at,
				synced_at      = excluded.synced_at`,
			item.ID, item.Title(), item.State(), item.AssignedTo(),
			item.WorkItemType(), item.WebURL, item.UpdatedAt(), now)
		if err != nil {
			return fmt.Errorf("azure sync: upsert #%d: %w", item.ID, err)
		}
		upserted++
	}

	fmt.Printf("azure sync: %d work items synced to SQLite\n", upserted)
	return nil
}

// ListCached returns work items stored in SQLite without hitting the API.
func ListCached(db *sql.DB) ([]WorkItem, error) {
	if _, err := db.Exec(createAzureTable); err != nil {
		return nil, fmt.Errorf("azure cached: create table: %w", err)
	}

	rows, err := db.Query(`
		SELECT work_item_id, title, state, assigned_to, work_item_type, url, updated_at
		FROM azure_workitems ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []WorkItem
	for rows.Next() {
		var item WorkItem
		item.Fields = make(map[string]any)
		var id int
		var title, state, assignedTo, wiType, url, updatedAt string
		if err := rows.Scan(&id, &title, &state, &assignedTo, &wiType, &url, &updatedAt); err != nil {
			return nil, err
		}
		item.ID = id
		item.Fields["System.Title"] = title
		item.Fields["System.State"] = state
		item.Fields["System.AssignedTo"] = assignedTo
		item.Fields["System.WorkItemType"] = wiType
		item.Fields["System.ChangedDate"] = updatedAt
		item.WebURL = url
		items = append(items, item)
	}
	return items, rows.Err()
}
