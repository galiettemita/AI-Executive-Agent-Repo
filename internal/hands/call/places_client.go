package call

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PhoneVerificationResult represents the outcome of verifying a phone number.
type PhoneVerificationResult struct {
	Verified     bool   `json:"verified"`
	Source       string `json:"source"` // google_places, manual_override, pre_verified
	BusinessName string `json:"business_name,omitempty"`
	Address      string `json:"address,omitempty"`
	PlaceID      string `json:"place_id,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

// PhoneVerifier verifies that a phone number belongs to a legitimate business.
// NNR: Unverified phone numbers MUST be rejected.
type PhoneVerifier interface {
	VerifyPhone(ctx context.Context, phoneNumber, businessQuery string) (*PhoneVerificationResult, error)
}

// GooglePlacesClient verifies phone numbers via the Google Places API.
type GooglePlacesClient struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

// NewGooglePlacesClient creates a client for Google Places phone verification.
func NewGooglePlacesClient(apiKey string) *GooglePlacesClient {
	return &GooglePlacesClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiKey:     apiKey,
		baseURL:    "https://maps.googleapis.com/maps/api/place",
	}
}

// VerifyPhone looks up a business by query, then verifies the phone number
// matches one of the results. Returns verified=true only if the number is
// confirmed via Google Places.
func (c *GooglePlacesClient) VerifyPhone(ctx context.Context, phoneNumber, businessQuery string) (*PhoneVerificationResult, error) {
	if c.apiKey == "" {
		return &PhoneVerificationResult{
			Verified: false,
			Source:   "google_places",
			Reason:   "GOOGLE_PLACES_API_KEY_NOT_SET",
		}, nil
	}

	if phoneNumber == "" {
		return &PhoneVerificationResult{
			Verified: false,
			Source:   "google_places",
			Reason:   "EMPTY_PHONE_NUMBER",
		}, nil
	}

	// Step 1: Find the place by business name/query.
	placeID, businessName, err := c.findPlace(ctx, businessQuery)
	if err != nil {
		return &PhoneVerificationResult{
			Verified: false,
			Source:   "google_places",
			Reason:   fmt.Sprintf("PLACES_LOOKUP_FAILED: %v", err),
		}, nil
	}
	if placeID == "" {
		return &PhoneVerificationResult{
			Verified: false,
			Source:   "google_places",
			Reason:   "NO_PLACE_FOUND: no matching business found",
		}, nil
	}

	// Step 2: Get place details to verify the phone number.
	details, err := c.getPlaceDetails(ctx, placeID)
	if err != nil {
		return &PhoneVerificationResult{
			Verified:     false,
			Source:       "google_places",
			BusinessName: businessName,
			PlaceID:      placeID,
			Reason:       fmt.Sprintf("DETAILS_LOOKUP_FAILED: %v", err),
		}, nil
	}

	// Step 3: Check if the provided phone matches the place's phone.
	normalizedInput := normalizePhone(phoneNumber)
	for _, placePhone := range details.PhoneNumbers {
		if normalizePhone(placePhone) == normalizedInput {
			return &PhoneVerificationResult{
				Verified:     true,
				Source:       "google_places",
				BusinessName: details.Name,
				Address:      details.Address,
				PlaceID:      placeID,
			}, nil
		}
	}

	return &PhoneVerificationResult{
		Verified:     false,
		Source:       "google_places",
		BusinessName: details.Name,
		PlaceID:      placeID,
		Reason:       "PHONE_MISMATCH: provided number does not match business records",
	}, nil
}

type placeDetails struct {
	Name         string
	Address      string
	PhoneNumbers []string
}

func (c *GooglePlacesClient) findPlace(ctx context.Context, query string) (placeID, name string, err error) {
	u := fmt.Sprintf("%s/findplacefromtext/json?input=%s&inputtype=textquery&fields=place_id,name&key=%s",
		c.baseURL, url.QueryEscape(query), c.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read response: %w", err)
	}

	var result struct {
		Candidates []struct {
			PlaceID string `json:"place_id"`
			Name    string `json:"name"`
		} `json:"candidates"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("unmarshal: %w", err)
	}

	if result.Status != "OK" || len(result.Candidates) == 0 {
		return "", "", nil
	}

	return result.Candidates[0].PlaceID, result.Candidates[0].Name, nil
}

func (c *GooglePlacesClient) getPlaceDetails(ctx context.Context, placeID string) (*placeDetails, error) {
	u := fmt.Sprintf("%s/details/json?place_id=%s&fields=name,formatted_address,formatted_phone_number,international_phone_number&key=%s",
		c.baseURL, url.QueryEscape(placeID), c.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result struct {
		Result struct {
			Name                    string `json:"name"`
			FormattedAddress        string `json:"formatted_address"`
			FormattedPhoneNumber    string `json:"formatted_phone_number"`
			InternationalPhoneNumber string `json:"international_phone_number"`
		} `json:"result"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if result.Status != "OK" {
		return nil, fmt.Errorf("places api status: %s", result.Status)
	}

	var phones []string
	if result.Result.FormattedPhoneNumber != "" {
		phones = append(phones, result.Result.FormattedPhoneNumber)
	}
	if result.Result.InternationalPhoneNumber != "" {
		phones = append(phones, result.Result.InternationalPhoneNumber)
	}

	return &placeDetails{
		Name:         result.Result.Name,
		Address:      result.Result.FormattedAddress,
		PhoneNumbers: phones,
	}, nil
}

// normalizePhone strips non-digit characters for comparison.
func normalizePhone(phone string) string {
	var b strings.Builder
	for _, c := range phone {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// DeterministicPhoneVerifier is a test stub that returns deterministic verification
// results based on phone number patterns. Used in tests and degraded mode.
type DeterministicPhoneVerifier struct{}

// NewDeterministicPhoneVerifier creates a test phone verifier.
func NewDeterministicPhoneVerifier() *DeterministicPhoneVerifier {
	return &DeterministicPhoneVerifier{}
}

// VerifyPhone returns verified=true for numbers starting with "+1" (US numbers)
// and verified=false for all others. Deterministic for replay safety.
func (d *DeterministicPhoneVerifier) VerifyPhone(_ context.Context, phoneNumber, businessQuery string) (*PhoneVerificationResult, error) {
	if strings.HasPrefix(phoneNumber, "+1") && len(phoneNumber) >= 11 {
		return &PhoneVerificationResult{
			Verified:     true,
			Source:       "pre_verified",
			BusinessName: businessQuery,
		}, nil
	}
	return &PhoneVerificationResult{
		Verified: false,
		Source:   "pre_verified",
		Reason:   "UNVERIFIED_NUMBER: phone number could not be verified",
	}, nil
}
