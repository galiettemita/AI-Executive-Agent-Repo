package pii

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
)

// PHI-specific detection patterns (HIPAA Protected Health Information).
var (
	phiICD10Re      = regexp.MustCompile(`\b[A-Z][0-9]{2}\.?[0-9A-Z]{0,4}\b`)
	phiBPRe         = regexp.MustCompile(`\b\d{2,3}/\d{2,3}\s*(?:mmHg)?\b`)
	phiHeartRateRe  = regexp.MustCompile(`(?i)\b\d{2,3}\s*(?:bpm|beats per minute)\b`)
	phiGlucoseRe    = regexp.MustCompile(`(?i)\b\d{2,3}\s*(?:mg/dL|mmol/L)\b`)
	phiDiagnosisRe  = regexp.MustCompile(`(?i)\b(?:diagnosed with|diagnosis of|patient has|suffering from|treated for)\s+[A-Za-z\s]{3,50}\b`)
	phiMRNRe        = regexp.MustCompile(`(?i)\b(?:MRN|Medical Record Number|Patient ID)[:\s#]*[0-9A-Z]{4,12}\b`)
	phiInsuranceRe  = regexp.MustCompile(`(?i)\b(?:insurance|policy|member)\s*(?:id|no|number|#)[:\s]*[A-Z0-9]{6,15}\b`)
	phiDOBRe        = regexp.MustCompile(`(?i)\b(?:date of birth|DOB|born on)[:\s]*\d{1,2}[/\-]\d{1,2}[/\-]\d{2,4}\b`)
	phiMedicationRe *regexp.Regexp // built at init from medication list
)

// phiMedicationNames is loaded from data file or uses default list.
var phiMedicationNames []string

func init() {
	// Try to load medication names from data file.
	if data, err := os.ReadFile("internal/security/pii/data/phi_medications.json"); err == nil {
		_ = json.Unmarshal(data, &phiMedicationNames)
	}

	// Fallback default list if file not available.
	if len(phiMedicationNames) == 0 {
		phiMedicationNames = defaultMedications
	}

	// Build regex alternation from medication list.
	escaped := make([]string, len(phiMedicationNames))
	for i, name := range phiMedicationNames {
		escaped[i] = regexp.QuoteMeta(strings.ToLower(name))
	}
	phiMedicationRe = regexp.MustCompile(`(?i)\b(?:` + strings.Join(escaped, "|") + `)\b`)
}

var defaultMedications = []string{
	"metformin", "lisinopril", "atorvastatin", "omeprazole", "amlodipine",
	"metoprolol", "albuterol", "gabapentin", "hydrochlorothiazide", "losartan",
	"levothyroxine", "acetaminophen", "ibuprofen", "amoxicillin", "azithromycin",
	"prednisone", "fluticasone", "montelukast", "escitalopram", "sertraline",
	"duloxetine", "bupropion", "trazodone", "alprazolam", "lorazepam",
	"clonazepam", "zolpidem", "tramadol", "oxycodone", "hydrocodone",
	"morphine", "fentanyl", "warfarin", "apixaban", "rivaroxaban",
	"clopidogrel", "aspirin", "insulin", "glipizide", "pioglitazone",
}

// ScanForPHI detects Protected Health Information patterns in text.
func ScanForPHI(text string) []DetectedPII {
	var results []DetectedPII

	type phiPattern struct {
		re         *regexp.Regexp
		typeName   string
		confidence float64
	}

	patterns := []phiPattern{
		{phiBPRe, "blood_pressure", 0.85},
		{phiHeartRateRe, "heart_rate", 0.85},
		{phiGlucoseRe, "blood_glucose", 0.85},
		{phiDiagnosisRe, "diagnosis", 0.90},
		{phiMRNRe, "medical_record_number", 0.95},
		{phiInsuranceRe, "insurance_id", 0.90},
		{phiDOBRe, "date_of_birth_health", 0.90},
	}

	if phiMedicationRe != nil {
		patterns = append(patterns, phiPattern{phiMedicationRe, "medication", 0.85})
	}

	// ICD-10 codes — only match if they look like real codes (not single letters).
	for _, loc := range phiICD10Re.FindAllStringIndex(text, -1) {
		match := text[loc[0]:loc[1]]
		// Filter out short matches that are likely not ICD-10 (e.g., "A12").
		if len(match) >= 4 && strings.Contains(match, ".") {
			results = append(results, DetectedPII{
				Type:       "icd10_code",
				Value:      match,
				StartIndex: loc[0],
				EndIndex:   loc[1],
				Confidence: 0.80,
			})
		}
	}

	for _, p := range patterns {
		for _, loc := range p.re.FindAllStringIndex(text, -1) {
			results = append(results, DetectedPII{
				Type:       p.typeName,
				Value:      text[loc[0]:loc[1]],
				StartIndex: loc[0],
				EndIndex:   loc[1],
				Confidence: p.confidence,
			})
		}
	}

	return results
}

// RedactPHI replaces all detected PHI in text with [PHI REDACTED].
func RedactPHI(text string) string {
	matches := ScanForPHI(text)
	if len(matches) == 0 {
		return text
	}

	// Sort matches by start index descending to replace from end to start.
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].StartIndex > matches[i].StartIndex {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	result := text
	for _, m := range matches {
		if m.StartIndex >= 0 && m.EndIndex <= len(result) {
			result = result[:m.StartIndex] + "[PHI REDACTED]" + result[m.EndIndex:]
		}
	}

	return result
}

// ContainsPHI returns true if the text contains any PHI patterns.
func ContainsPHI(text string) bool {
	return len(ScanForPHI(text)) > 0
}
