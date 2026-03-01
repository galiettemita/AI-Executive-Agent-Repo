package workflows

import "strings"

type DriftEvaluationInput struct {
	ConsecutiveHealthCheckFailures int
	SchemaHashMismatch             bool
	DeepHealthCheckFailure         bool
	CurrentSeverity                string
	NextHealthCheckPassed          bool
}

type DriftEvaluationOutput struct {
	Quarantine     bool
	DriftSeverity  string
	AutoRestored   bool
	RequiresReview bool
}

func EvaluateDriftWatchdog(input DriftEvaluationInput) DriftEvaluationOutput {
	if input.DeepHealthCheckFailure {
		return DriftEvaluationOutput{
			Quarantine:     true,
			DriftSeverity:  "critical",
			AutoRestored:   false,
			RequiresReview: true,
		}
	}
	if input.SchemaHashMismatch {
		return DriftEvaluationOutput{
			Quarantine:     true,
			DriftSeverity:  "elevated",
			AutoRestored:   false,
			RequiresReview: true,
		}
	}
	if input.ConsecutiveHealthCheckFailures >= 3 {
		return DriftEvaluationOutput{
			Quarantine:     true,
			DriftSeverity:  "low",
			AutoRestored:   false,
			RequiresReview: false,
		}
	}

	current := strings.ToLower(strings.TrimSpace(input.CurrentSeverity))
	if current == "low" && input.NextHealthCheckPassed {
		return DriftEvaluationOutput{
			Quarantine:     false,
			DriftSeverity:  "none",
			AutoRestored:   true,
			RequiresReview: false,
		}
	}
	if current == "elevated" || current == "critical" {
		return DriftEvaluationOutput{
			Quarantine:     true,
			DriftSeverity:  current,
			AutoRestored:   false,
			RequiresReview: true,
		}
	}

	return DriftEvaluationOutput{
		Quarantine:     false,
		DriftSeverity:  "none",
		AutoRestored:   false,
		RequiresReview: false,
	}
}
