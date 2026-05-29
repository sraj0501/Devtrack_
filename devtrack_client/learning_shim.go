package main

// learning_shim.go — forwarding aliases from package main to internal/learning.

import (
	ilearn "github.com/sraj0501/Devtrack_/devtrack_client/internal/learning"
)

// ── Type aliases ─────────────────────────────────────────────────────────────

type LearningCommands = ilearn.LearningCommands
type LearningStatus = ilearn.LearningStatus

// ── Function forwards ─────────────────────────────────────────────────────────

func NewLearningCommands() *LearningCommands { return ilearn.NewLearningCommands() }
