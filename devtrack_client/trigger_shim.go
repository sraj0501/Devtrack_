package main

// trigger_shim.go — forwarding aliases from package main to internal/trigger.
// Lets the CLI files and tests keep using bare names while the HTTP trigger
// client, TLS helpers, and trigger payload types live in internal/trigger.

import (
	trig "github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

// ── Type aliases — trigger client + payloads ─────────────────────────────────

type HTTPTriggerClient = trig.HTTPTriggerClient
type CommitTriggerData = trig.CommitTriggerData
type TimerTriggerData = trig.TimerTriggerData
type TaskUpdateData = trig.TaskUpdateData

// ── Type aliases — plan / boardroom DTOs ─────────────────────────────────────

type PlanPreviewRequest = trig.PlanPreviewRequest
type PlanPreviewResponse = trig.PlanPreviewResponse
type PlanCreatedItem = trig.PlanCreatedItem
type PlanFailedItem = trig.PlanFailedItem
type PlanCreateResponse = trig.PlanCreateResponse
type BoardroomRequest = trig.BoardroomRequest
type BoardroomResponse = trig.BoardroomResponse
type BoardroomHistoryEntry = trig.BoardroomHistoryEntry
type BoardroomChatRequest = trig.BoardroomChatRequest
type BoardroomPersonaResponse = trig.BoardroomPersonaResponse
type BoardroomChatResponse = trig.BoardroomChatResponse

// ── Function forwards ─────────────────────────────────────────────────────────

func NewHTTPTriggerClient() *HTTPTriggerClient                  { return trig.NewHTTPTriggerClient() }
func GenerateSelfSignedCert(certPath, keyPath string) error     { return trig.GenerateSelfSignedCert(certPath, keyPath) }
