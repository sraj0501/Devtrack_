package main

// db_shim.go — forwarding aliases from package main to internal/db.

import (
	idb "github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)

// ── Type aliases ─────────────────────────────────────────────────────────────

type Database = idb.Database
type TriggerRecord = idb.TriggerRecord
type ResponseRecord = idb.ResponseRecord
type TaskUpdateRecord = idb.TaskUpdateRecord
type LogRecord = idb.LogRecord
type QueuedMessage = idb.QueuedMessage
type DeferredCommitRecord = idb.DeferredCommitRecord
type HealthSnapshot = idb.HealthSnapshot
type ReportRecord = idb.ReportRecord
type WorkSessionRecord = idb.WorkSessionRecord
type NotificationRecord = idb.NotificationRecord
type VacationState = idb.VacationState
type TicketCacheRecord = idb.TicketCacheRecord
type PMUpdateQueueRecord = idb.PMUpdateQueueRecord
type TicketSourceSummary = idb.TicketSourceSummary
type Skill = idb.Skill
type Inference = idb.Inference
type Correction = idb.Correction
type ConfidenceThreshold = idb.ConfidenceThreshold

// ── Function forwards ─────────────────────────────────────────────────────────

func NewDatabase() (*Database, error)                        { return idb.NewDatabase() }
func RunPendingMigrations()                                  { idb.RunPendingMigrations() }
func MarkAllMigrationsApplied()                              { idb.MarkAllMigrationsApplied() }
