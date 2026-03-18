package soc2

import "time"

// ControlEvidence represents a single compliance evidence record.
type ControlEvidence struct {
	ControlID    string                 `json:"control_id"`
	Framework    string                 `json:"framework"` // "soc2" or "iso27001"
	EvidenceType string                 `json:"evidence_type"`
	CollectedAt  time.Time              `json:"collected_at"`
	Pass         bool                   `json:"pass"`
	Details      map[string]interface{} `json:"details"`
}

// Frameworks.
const (
	FrameworkSOC2     = "soc2"
	FrameworkISO27001 = "iso27001"
)

// SOC2 Trust Services Criteria control IDs.
const (
	ControlCC61 = "CC6.1" // Logical Access
	ControlCC66 = "CC6.6" // Boundary Protection
	ControlCC72 = "CC7.2" // Anomaly Detection
	ControlCC92 = "CC9.2" // Vendor Risk
	ControlPI14 = "PI1.4" // Processing Integrity
)

// DailyControls are collected every day.
var DailyControls = []string{ControlCC61, ControlCC72, ControlPI14}

// WeeklyControls are collected only on Mondays.
var WeeklyControls = []string{ControlCC66, ControlCC92}
