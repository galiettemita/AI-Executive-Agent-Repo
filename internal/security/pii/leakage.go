package pii

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

type FingerprintType string

const (
	FingerprintEmail FingerprintType = "email"
	FingerprintPhone FingerprintType = "phone"
	FingerprintName  FingerprintType = "full_name"
)

type LeakageMatch struct {
	UserID          string
	FingerprintType FingerprintType
	FingerprintHash string
}

var leakEmailPattern = regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`)
var leakPhonePattern = regexp.MustCompile(`\+?[0-9][0-9\-\s\(\)]{7,}[0-9]`)

func fingerprintValue(raw string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(raw))))
	return hex.EncodeToString(sum[:])
}

func BuildUserFingerprints(email, phoneE164, fullName string) map[FingerprintType]string {
	return map[FingerprintType]string{
		FingerprintEmail: fingerprintValue(email),
		FingerprintPhone: fingerprintValue(phoneE164),
		FingerprintName:  fingerprintValue(fullName),
	}
}

func DetectForeignPIILeakage(responseText, currentUserID string, allFingerprints map[string]map[FingerprintType]string, falsePositiveHashes map[string]bool) (bool, *LeakageMatch) {
	candidateFingerprints := map[FingerprintType][]string{
		FingerprintEmail: {},
		FingerprintPhone: {},
		FingerprintName:  {fingerprintValue(responseText)},
	}
	for _, email := range leakEmailPattern.FindAllString(responseText, -1) {
		candidateFingerprints[FingerprintEmail] = append(candidateFingerprints[FingerprintEmail], fingerprintValue(email))
	}
	for _, phone := range leakPhonePattern.FindAllString(responseText, -1) {
		candidateFingerprints[FingerprintPhone] = append(candidateFingerprints[FingerprintPhone], fingerprintValue(normalizePhone(phone)))
	}

	for userID, fingerprints := range allFingerprints {
		if userID == currentUserID {
			continue
		}
		for fpType, expectedHash := range fingerprints {
			if falsePositiveHashes[expectedHash] {
				continue
			}
			for _, observedHash := range candidateFingerprints[fpType] {
				if observedHash == expectedHash {
					return true, &LeakageMatch{
						UserID:          userID,
						FingerprintType: fpType,
						FingerprintHash: expectedHash,
					}
				}
			}
		}
	}
	return false, nil
}

func normalizePhone(raw string) string {
	digits := strings.Builder{}
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	return digits.String()
}
