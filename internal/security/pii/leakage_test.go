package pii

import "testing"

func TestDetectForeignPIILeakage(t *testing.T) {
	t.Parallel()

	all := map[string]map[FingerprintType]string{
		"user_a": BuildUserFingerprints("alice@example.com", "+15551112222", "Alice Example"),
		"user_b": BuildUserFingerprints("bob@example.com", "+15553334444", "Bob Example"),
	}

	leaked, match := DetectForeignPIILeakage(
		"Please email bob@example.com about the report.",
		"user_a",
		all,
		map[string]bool{},
	)
	if !leaked || match == nil {
		t.Fatal("expected foreign PII leakage detection")
	}
	if match.UserID != "user_b" || match.FingerprintType != FingerprintEmail {
		t.Fatalf("unexpected leakage match: %+v", match)
	}
}

func TestDetectForeignPIILeakageFalsePositiveSuppress(t *testing.T) {
	t.Parallel()

	all := map[string]map[FingerprintType]string{
		"user_a": BuildUserFingerprints("alice@example.com", "+15551112222", "Alice Example"),
		"user_b": BuildUserFingerprints("bob@example.com", "+15553334444", "Bob Example"),
	}
	bobHash := all["user_b"][FingerprintEmail]
	leaked, _ := DetectForeignPIILeakage(
		"Please email bob@example.com about the report.",
		"user_a",
		all,
		map[string]bool{bobHash: true},
	)
	if leaked {
		t.Fatal("expected false-positive suppression for known fingerprint hash")
	}
}
