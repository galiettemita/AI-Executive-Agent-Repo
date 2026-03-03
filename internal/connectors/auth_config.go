package connectors

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	authConfigSegmentPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

func CanonicalAuthSecretName(environment, service, field string) (string, error) {
	env := strings.ToLower(strings.TrimSpace(environment))
	svc := strings.ToLower(strings.TrimSpace(service))
	fld := strings.ToLower(strings.TrimSpace(field))

	if !authConfigSegmentPattern.MatchString(env) {
		return "", fmt.Errorf("invalid environment segment: %q", environment)
	}
	if !authConfigSegmentPattern.MatchString(svc) {
		return "", fmt.Errorf("invalid service segment: %q", service)
	}
	if fld != "client_id" && fld != "client_secret" {
		return "", fmt.Errorf("invalid secret field: %q", field)
	}

	return fmt.Sprintf("brevio/%s/%s/%s", env, svc, fld), nil
}

func OAuthRedirectURI(service string) (string, error) {
	svc := strings.ToLower(strings.TrimSpace(service))
	if !authConfigSegmentPattern.MatchString(svc) {
		return "", fmt.Errorf("invalid service segment: %q", service)
	}
	return fmt.Sprintf("https://auth.brevio.app/callback/%s", svc), nil
}

func PKCERequiredForOAuthFlows() bool {
	return true
}
