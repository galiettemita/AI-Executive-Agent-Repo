package browser

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Stealth browser sessions
// ---------------------------------------------------------------------------

// StealthOptions configures a stealth browsing session.
type StealthOptions struct {
	UseProxy        bool
	CustomUserAgent string
	BlockTrackers   bool
	ScreenSize      string
}

// StealthSession represents an active headless browser session with
// anti-fingerprinting measures.
type StealthSession struct {
	ID          string
	WorkspaceID string
	URL         string
	UserAgent   string
	ProxyURL    string
	Fingerprint string
	Status      string // active, completed, failed
	CreatedAt   time.Time
}

// BrowserEngine manages stealth browser sessions in-memory.
type BrowserEngine struct {
	mu       sync.RWMutex
	sessions map[string]*StealthSession
	// pages stores the current HTML-like content per session for content extraction / form fill.
	pages map[string]map[string]string
}

// NewBrowserEngine returns a ready-to-use BrowserEngine.
func NewBrowserEngine() *BrowserEngine {
	return &BrowserEngine{
		sessions: make(map[string]*StealthSession),
		pages:    make(map[string]map[string]string),
	}
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var defaultUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_4) AppleWebKit/605.1.15 Safari/17.4",
	"Mozilla/5.0 (X11; Linux x86_64; rv:125.0) Gecko/20100101 Firefox/125.0",
}

func randomUserAgent() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(defaultUserAgents))))
	return defaultUserAgents[n.Int64()]
}

func generateFingerprint() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "fp-" + hex.EncodeToString(b)
}

// StartStealthSession opens a new stealth browser session targeting url.
func (e *BrowserEngine) StartStealthSession(workspaceID, url string, opts StealthOptions) (*StealthSession, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	if url == "" {
		return nil, errors.New("url is required")
	}

	ua := opts.CustomUserAgent
	if ua == "" {
		ua = randomUserAgent()
	}

	var proxy string
	if opts.UseProxy {
		proxy = "socks5://proxy.brevio.internal:1080"
	}

	sess := &StealthSession{
		ID:          generateID(),
		WorkspaceID: workspaceID,
		URL:         url,
		UserAgent:   ua,
		ProxyURL:    proxy,
		Fingerprint: generateFingerprint(),
		Status:      "active",
		CreatedAt:   time.Now().UTC(),
	}

	e.mu.Lock()
	e.sessions[sess.ID] = sess
	e.pages[sess.ID] = make(map[string]string)
	e.mu.Unlock()
	return sess, nil
}

// NavigateTo changes the URL of an active session.
func (e *BrowserEngine) NavigateTo(sessionID, url string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	sess, ok := e.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}
	if sess.Status != "active" {
		return fmt.Errorf("session %s is not active (status=%s)", sessionID, sess.Status)
	}
	if url == "" {
		return errors.New("url is required")
	}
	sess.URL = url
	// reset page content for the new navigation
	e.pages[sessionID] = make(map[string]string)
	return nil
}

// ExtractContent retrieves text content from the current page for the given
// CSS selectors.  Because this engine is an in-memory simulation (no real
// browser), it returns placeholder data keyed by selector.
func (e *BrowserEngine) ExtractContent(sessionID string, selectors []string) (map[string]string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	sess, ok := e.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	if sess.Status != "active" {
		return nil, fmt.Errorf("session %s is not active", sessionID)
	}
	if len(selectors) == 0 {
		return nil, errors.New("at least one selector is required")
	}

	result := make(map[string]string, len(selectors))
	page := e.pages[sessionID]
	for _, sel := range selectors {
		if v, found := page[sel]; found {
			result[sel] = v
		} else {
			// Simulated extraction: produce content derived from the selector name.
			result[sel] = fmt.Sprintf("content[%s@%s]", sel, sess.URL)
		}
	}
	return result, nil
}

// FillForm fills form fields on the current page.
func (e *BrowserEngine) FillForm(sessionID string, fields map[string]string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	sess, ok := e.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}
	if sess.Status != "active" {
		return fmt.Errorf("session %s is not active", sessionID)
	}
	if len(fields) == 0 {
		return errors.New("at least one field is required")
	}
	page := e.pages[sessionID]
	for k, v := range fields {
		page[k] = v
	}
	return nil
}

// TakeScreenshot returns a synthetic PNG-header byte slice representing a
// screenshot of the current page.
func (e *BrowserEngine) TakeScreenshot(sessionID string) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	sess, ok := e.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	if sess.Status != "active" {
		return nil, fmt.Errorf("session %s is not active", sessionID)
	}
	// Return a minimal PNG header (8-byte magic) + URL-derived payload.
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	payload := []byte(fmt.Sprintf("screenshot:%s:%s", sessionID, sess.URL))
	return append(pngHeader, payload...), nil
}

// CloseSession terminates a stealth session.
func (e *BrowserEngine) CloseSession(sessionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	sess, ok := e.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}
	sess.Status = "completed"
	delete(e.pages, sessionID)
	return nil
}

// ---------------------------------------------------------------------------
// Scraping
// ---------------------------------------------------------------------------

// ScrapeResult holds the result of a web scrape operation.
type ScrapeResult struct {
	URL           string
	ExtractedData map[string]string
	StatusCode    int
	LoadTimeMs    int
}

// ScrapingService provides one-shot scraping without managing sessions.
type ScrapingService struct {
	engine *BrowserEngine
}

// NewScrapingService creates a ScrapingService backed by the given engine.
func NewScrapingService(engine *BrowserEngine) *ScrapingService {
	return &ScrapingService{engine: engine}
}

// Scrape fetches url and extracts data according to selectors.
func (s *ScrapingService) Scrape(url string, selectors map[string]string) (*ScrapeResult, error) {
	if url == "" {
		return nil, errors.New("url is required")
	}
	if len(selectors) == 0 {
		return nil, errors.New("at least one selector is required")
	}

	start := time.Now()

	// Open an ephemeral session, extract, close.
	sess, err := s.engine.StartStealthSession("scrape-ephemeral", url, StealthOptions{BlockTrackers: true})
	if err != nil {
		return nil, fmt.Errorf("start session: %w", err)
	}
	defer func() { _ = s.engine.CloseSession(sess.ID) }()

	keys := make([]string, 0, len(selectors))
	for k := range selectors {
		keys = append(keys, k)
	}
	raw, err := s.engine.ExtractContent(sess.ID, keys)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}

	// Map extracted values back to caller-supplied names.
	extracted := make(map[string]string, len(selectors))
	for key, name := range selectors {
		extracted[name] = raw[key]
	}

	return &ScrapeResult{
		URL:           url,
		ExtractedData: extracted,
		StatusCode:    200,
		LoadTimeMs:    int(time.Since(start).Milliseconds()),
	}, nil
}

// ---------------------------------------------------------------------------
// Form filler
// ---------------------------------------------------------------------------

// FormResult represents the outcome of a form fill + submit.
type FormResult struct {
	Success        bool
	SubmissionID   string
	ResponseStatus int
}

// FormFillerService fills and submits web forms.
type FormFillerService struct {
	engine *BrowserEngine
}

// NewFormFillerService creates a FormFillerService.
func NewFormFillerService(engine *BrowserEngine) *FormFillerService {
	return &FormFillerService{engine: engine}
}

// FillAndSubmit navigates to url, fills formData, and submits.
func (f *FormFillerService) FillAndSubmit(url string, formData map[string]string) (*FormResult, error) {
	if url == "" {
		return nil, errors.New("url is required")
	}
	if len(formData) == 0 {
		return nil, errors.New("formData is required")
	}

	sess, err := f.engine.StartStealthSession("form-ephemeral", url, StealthOptions{})
	if err != nil {
		return nil, fmt.Errorf("start session: %w", err)
	}
	defer func() { _ = f.engine.CloseSession(sess.ID) }()

	if err := f.engine.FillForm(sess.ID, formData); err != nil {
		return nil, fmt.Errorf("fill form: %w", err)
	}

	return &FormResult{
		Success:        true,
		SubmissionID:   generateID(),
		ResponseStatus: 200,
	}, nil
}

// ---------------------------------------------------------------------------
// Booking
// ---------------------------------------------------------------------------

// DateRange represents a time window.
type DateRange struct {
	Start time.Time
	End   time.Time
}

// TimeSlot represents an available booking slot.
type TimeSlot struct {
	DateTime time.Time
	Duration time.Duration
	Provider string
	Service  string
}

// Booking represents a confirmed booking.
type Booking struct {
	ID               string
	Provider         string
	Service          string
	DateTime         time.Time
	Status           string
	ConfirmationCode string
}

// BookingService searches availability and books slots.
type BookingService struct {
	mu       sync.RWMutex
	bookings map[string]*Booking
}

// NewBookingService creates a BookingService.
func NewBookingService() *BookingService {
	return &BookingService{bookings: make(map[string]*Booking)}
}

// SearchAvailability returns available slots for a provider/service in the
// given date range.  Since we have no real provider integration, we generate
// synthetic slots at 30-minute intervals during business hours (09-17).
func (b *BookingService) SearchAvailability(provider, service string, dateRange DateRange) ([]TimeSlot, error) {
	if provider == "" || service == "" {
		return nil, errors.New("provider and service are required")
	}
	if dateRange.End.Before(dateRange.Start) {
		return nil, errors.New("end must be after start")
	}

	var slots []TimeSlot
	for d := dateRange.Start; d.Before(dateRange.End); d = d.Add(24 * time.Hour) {
		for hour := 9; hour < 17; hour++ {
			for min := 0; min < 60; min += 30 {
				t := time.Date(d.Year(), d.Month(), d.Day(), hour, min, 0, 0, time.UTC)
				if t.Before(dateRange.Start) || t.After(dateRange.End) {
					continue
				}
				slots = append(slots, TimeSlot{
					DateTime: t,
					Duration: 30 * time.Minute,
					Provider: provider,
					Service:  service,
				})
			}
		}
	}
	return slots, nil
}

// BookSlot confirms a booking for the given slot.
func (b *BookingService) BookSlot(provider string, slot TimeSlot, details map[string]string) (*Booking, error) {
	if provider == "" {
		return nil, errors.New("provider is required")
	}

	booking := &Booking{
		ID:               generateID(),
		Provider:         provider,
		Service:          slot.Service,
		DateTime:         slot.DateTime,
		Status:           "confirmed",
		ConfirmationCode: strings.ToUpper(generateID()[:8]),
	}

	b.mu.Lock()
	b.bookings[booking.ID] = booking
	b.mu.Unlock()
	return booking, nil
}

// ---------------------------------------------------------------------------
// Price tracking
// ---------------------------------------------------------------------------

// PriceWatch represents a watched product price.
type PriceWatch struct {
	ID           string
	URL          string
	ProductName  string
	CurrentPrice float64
	TargetPrice  float64
	Status       string // watching, triggered, expired
}

// PriceCheck is the result of checking a watched price.
type PriceCheck struct {
	Price       float64
	Changed     bool
	BelowTarget bool
}

// PriceTracker monitors product prices.
type PriceTracker struct {
	mu      sync.RWMutex
	watches map[string]*PriceWatch
}

// NewPriceTracker creates a PriceTracker.
func NewPriceTracker() *PriceTracker {
	return &PriceTracker{watches: make(map[string]*PriceWatch)}
}

// TrackPrice begins watching a product at url for the target price.
func (p *PriceTracker) TrackPrice(url, productName string, targetPrice float64) (*PriceWatch, error) {
	if url == "" {
		return nil, errors.New("url is required")
	}
	if productName == "" {
		return nil, errors.New("productName is required")
	}
	if targetPrice <= 0 {
		return nil, errors.New("targetPrice must be positive")
	}

	// Simulate an initial price fetch: set current price above target.
	initialPrice := targetPrice * 1.2

	watch := &PriceWatch{
		ID:           generateID(),
		URL:          url,
		ProductName:  productName,
		CurrentPrice: initialPrice,
		TargetPrice:  targetPrice,
		Status:       "watching",
	}

	p.mu.Lock()
	p.watches[watch.ID] = watch
	p.mu.Unlock()
	return watch, nil
}

// CheckPrices checks the current price against the target for a watched
// product.  In a real implementation this would scrape the product page; here
// we simulate a small price drop each call.
func (p *PriceTracker) CheckPrices(watchID string) (*PriceCheck, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	w, ok := p.watches[watchID]
	if !ok {
		return nil, fmt.Errorf("watch %s not found", watchID)
	}
	if w.Status != "watching" {
		return nil, fmt.Errorf("watch %s is %s", watchID, w.Status)
	}

	// Simulate a 5 % price drop.
	oldPrice := w.CurrentPrice
	w.CurrentPrice = oldPrice * 0.95

	belowTarget := w.CurrentPrice <= w.TargetPrice
	if belowTarget {
		w.Status = "triggered"
	}

	return &PriceCheck{
		Price:       w.CurrentPrice,
		Changed:     w.CurrentPrice != oldPrice,
		BelowTarget: belowTarget,
	}, nil
}
