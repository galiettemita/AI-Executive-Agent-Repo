package marketing

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// Campaign engine
// ---------------------------------------------------------------------------

// CampaignMetrics holds aggregate metrics for a campaign.
type CampaignMetrics struct {
	Sent      int
	Delivered int
	Opened    int
	Clicked   int
	Converted int
	OpenRate  float64
	ClickRate float64
}

// Campaign represents a marketing campaign.
type Campaign struct {
	ID          string
	WorkspaceID string
	Name        string
	Type        string // email, social, seo, newsletter
	Status      string // draft, active, paused, completed
	Audience    []string
	Content     string
	Schedule    string
	Metrics     CampaignMetrics
	CreatedAt   time.Time
}

// CampaignEngine manages campaign lifecycle.
type CampaignEngine struct {
	mu        sync.RWMutex
	campaigns map[string]*Campaign
}

// NewCampaignEngine creates a CampaignEngine.
func NewCampaignEngine() *CampaignEngine {
	return &CampaignEngine{campaigns: make(map[string]*Campaign)}
}

var validCampaignTypes = map[string]bool{
	"email": true, "social": true, "seo": true, "newsletter": true,
}

// CreateCampaign creates a new campaign in draft status.
func (ce *CampaignEngine) CreateCampaign(workspaceID string, campaign Campaign) (*Campaign, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	if campaign.Name == "" {
		return nil, errors.New("campaign name is required")
	}
	if !validCampaignTypes[campaign.Type] {
		return nil, fmt.Errorf("invalid campaign type %q: must be email, social, seo, or newsletter", campaign.Type)
	}

	campaign.ID = generateID()
	campaign.WorkspaceID = workspaceID
	campaign.Status = "draft"
	campaign.CreatedAt = time.Now().UTC()

	ce.mu.Lock()
	ce.campaigns[campaign.ID] = &campaign
	ce.mu.Unlock()
	return &campaign, nil
}

// LaunchCampaign transitions a campaign from draft to active.
func (ce *CampaignEngine) LaunchCampaign(campaignID string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	c, ok := ce.campaigns[campaignID]
	if !ok {
		return fmt.Errorf("campaign %s not found", campaignID)
	}
	if c.Status != "draft" && c.Status != "paused" {
		return fmt.Errorf("campaign %s cannot be launched from status %s", campaignID, c.Status)
	}
	c.Status = "active"

	// Simulate initial sending for email campaigns.
	if c.Type == "email" {
		sent := len(c.Audience)
		if sent == 0 {
			sent = 100
		}
		c.Metrics.Sent = sent
		c.Metrics.Delivered = int(float64(sent) * 0.97)
	}
	return nil
}

// PauseCampaign pauses an active campaign.
func (ce *CampaignEngine) PauseCampaign(campaignID string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	c, ok := ce.campaigns[campaignID]
	if !ok {
		return fmt.Errorf("campaign %s not found", campaignID)
	}
	if c.Status != "active" {
		return fmt.Errorf("campaign %s is not active (status=%s)", campaignID, c.Status)
	}
	c.Status = "paused"
	return nil
}

// GetCampaignMetrics returns the current metrics, simulating engagement.
func (ce *CampaignEngine) GetCampaignMetrics(campaignID string) (*CampaignMetrics, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	c, ok := ce.campaigns[campaignID]
	if !ok {
		return nil, fmt.Errorf("campaign %s not found", campaignID)
	}

	m := c.Metrics
	if c.Status == "active" && m.Delivered > 0 {
		m.Opened = int(float64(m.Delivered) * 0.35)
		m.Clicked = int(float64(m.Opened) * 0.12)
		m.Converted = int(float64(m.Clicked) * 0.03)
		m.OpenRate = float64(m.Opened) / float64(m.Delivered)
		m.ClickRate = float64(m.Clicked) / float64(m.Opened)
	}
	return &m, nil
}

// ---------------------------------------------------------------------------
// Social posting
// ---------------------------------------------------------------------------

// PostEngagement tracks social media engagement.
type PostEngagement struct {
	Likes       int
	Shares      int
	Comments    int
	Impressions int
}

// Post represents a social media post.
type Post struct {
	ID          string
	Platform    string // twitter, linkedin, instagram, facebook
	Content     string
	MediaURLs   []string
	ScheduledAt time.Time
	PostedAt    time.Time
	Status      string // draft, scheduled, published, failed
	Engagement  PostEngagement
}

// SocialPostingService manages social media posts.
type SocialPostingService struct {
	mu    sync.RWMutex
	posts map[string]*Post
}

// NewSocialPostingService creates a SocialPostingService.
func NewSocialPostingService() *SocialPostingService {
	return &SocialPostingService{posts: make(map[string]*Post)}
}

var validPlatforms = map[string]bool{
	"twitter": true, "linkedin": true, "instagram": true, "facebook": true,
}

// SchedulePost schedules a post for future publication.
func (s *SocialPostingService) SchedulePost(workspaceID string, post Post) (*Post, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	if post.Content == "" {
		return nil, errors.New("post content is required")
	}
	if !validPlatforms[post.Platform] {
		return nil, fmt.Errorf("unsupported platform %q", post.Platform)
	}
	if post.ScheduledAt.IsZero() {
		return nil, errors.New("scheduledAt is required")
	}

	post.ID = generateID()
	post.Status = "scheduled"

	s.mu.Lock()
	s.posts[post.ID] = &post
	s.mu.Unlock()
	return &post, nil
}

// PublishPost immediately publishes a scheduled post.
func (s *SocialPostingService) PublishPost(postID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.posts[postID]
	if !ok {
		return fmt.Errorf("post %s not found", postID)
	}
	if p.Status != "scheduled" && p.Status != "draft" {
		return fmt.Errorf("post %s cannot be published from status %s", postID, p.Status)
	}
	p.Status = "published"
	p.PostedAt = time.Now().UTC()
	return nil
}

// GetEngagement returns engagement metrics for a published post.
func (s *SocialPostingService) GetEngagement(postID string) (*PostEngagement, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.posts[postID]
	if !ok {
		return nil, fmt.Errorf("post %s not found", postID)
	}
	if p.Status != "published" {
		return nil, fmt.Errorf("post %s is not published", postID)
	}
	// Simulate engagement numbers.
	eng := PostEngagement{
		Impressions: 1200,
		Likes:       84,
		Shares:      12,
		Comments:    7,
	}
	p.Engagement = eng
	return &eng, nil
}

// ---------------------------------------------------------------------------
// Lead nurture
// ---------------------------------------------------------------------------

// Lead represents a marketing lead.
type Lead struct {
	ID              string
	WorkspaceID     string
	Email           string
	Name            string
	Company         string
	Score           int
	Stage           string // cold, warm, hot, customer
	LastContactedAt time.Time
	Tags            []string
}

// LeadNurtureService manages lead scoring and nurturing.
type LeadNurtureService struct {
	mu    sync.RWMutex
	leads map[string]*Lead
}

// NewLeadNurtureService creates a LeadNurtureService.
func NewLeadNurtureService() *LeadNurtureService {
	return &LeadNurtureService{leads: make(map[string]*Lead)}
}

// AddLead adds a new lead.
func (l *LeadNurtureService) AddLead(workspaceID string, lead Lead) (*Lead, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	if lead.Email == "" {
		return nil, errors.New("lead email is required")
	}

	lead.ID = generateID()
	lead.WorkspaceID = workspaceID
	if lead.Stage == "" {
		lead.Stage = "cold"
	}

	l.mu.Lock()
	l.leads[lead.ID] = &lead
	l.mu.Unlock()
	return &lead, nil
}

// ScoreLead computes an engagement score (0-100) based on available signals.
func (l *LeadNurtureService) ScoreLead(leadID string) (*Lead, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ld, ok := l.leads[leadID]
	if !ok {
		return nil, fmt.Errorf("lead %s not found", leadID)
	}

	score := 10 // base score
	if ld.Company != "" {
		score += 15
	}
	if ld.Name != "" {
		score += 10
	}
	if !ld.LastContactedAt.IsZero() {
		daysSince := int(time.Since(ld.LastContactedAt).Hours() / 24)
		if daysSince < 7 {
			score += 30
		} else if daysSince < 30 {
			score += 15
		}
	}
	score += len(ld.Tags) * 5
	if score > 100 {
		score = 100
	}
	ld.Score = score

	// Update stage based on score.
	switch {
	case score >= 70:
		ld.Stage = "hot"
	case score >= 40:
		ld.Stage = "warm"
	default:
		ld.Stage = "cold"
	}
	return ld, nil
}

// NurtureLead performs a nurture action (email, call, etc.).
func (l *LeadNurtureService) NurtureLead(leadID string, action string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	ld, ok := l.leads[leadID]
	if !ok {
		return fmt.Errorf("lead %s not found", leadID)
	}
	if action == "" {
		return errors.New("action is required")
	}
	ld.LastContactedAt = time.Now().UTC()
	// Append action as a tag for tracking.
	ld.Tags = append(ld.Tags, "nurture:"+action)
	return nil
}

// ---------------------------------------------------------------------------
// SEO audit
// ---------------------------------------------------------------------------

// SEOIssue represents a single SEO issue.
type SEOIssue struct {
	Type        string // missing_meta, slow_load, broken_link, no_alt_text
	Severity    string // low, medium, high, critical
	Description string
}

// SEOReport holds the results of an SEO audit.
type SEOReport struct {
	URL             string
	Score           int
	Issues          []SEOIssue
	Recommendations []string
}

// SEOAuditService audits websites for SEO issues.
type SEOAuditService struct{}

// NewSEOAuditService creates an SEOAuditService.
func NewSEOAuditService() *SEOAuditService { return &SEOAuditService{} }

// AuditSite performs an SEO audit of the given URL.
func (s *SEOAuditService) AuditSite(url string) (*SEOReport, error) {
	if url == "" {
		return nil, errors.New("url is required")
	}

	// Generate a realistic audit based on common issues.
	issues := []SEOIssue{
		{Type: "missing_meta", Severity: "high", Description: "Missing meta description tag"},
		{Type: "slow_load", Severity: "medium", Description: "Page load time exceeds 3 seconds"},
		{Type: "no_alt_text", Severity: "low", Description: "3 images missing alt text"},
	}

	// If the URL doesn't use HTTPS, add a critical issue.
	if !strings.HasPrefix(url, "https://") {
		issues = append(issues, SEOIssue{
			Type: "broken_link", Severity: "critical", Description: "Site not served over HTTPS",
		})
	}

	score := 100 - len(issues)*15
	if score < 0 {
		score = 0
	}

	recs := []string{
		"Add meta description to all pages",
		"Optimize images and enable lazy loading",
		"Add alt text to all images",
		"Implement structured data (JSON-LD)",
	}

	return &SEOReport{
		URL:             url,
		Score:           score,
		Issues:          issues,
		Recommendations: recs,
	}, nil
}

// ---------------------------------------------------------------------------
// Newsletter
// ---------------------------------------------------------------------------

// Newsletter represents an email newsletter.
type Newsletter struct {
	ID          string
	WorkspaceID string
	Subject     string
	Body        string
	Recipients  []string
	SentAt      time.Time
	OpenRate    float64
}

// NewsletterService manages newsletter creation and sending.
type NewsletterService struct {
	mu          sync.RWMutex
	newsletters map[string]*Newsletter
}

// NewNewsletterService creates a NewsletterService.
func NewNewsletterService() *NewsletterService {
	return &NewsletterService{newsletters: make(map[string]*Newsletter)}
}

// CreateNewsletter creates a new newsletter draft.
func (n *NewsletterService) CreateNewsletter(workspaceID, subject, body string, recipients []string) (*Newsletter, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	if subject == "" {
		return nil, errors.New("subject is required")
	}
	if body == "" {
		return nil, errors.New("body is required")
	}
	if len(recipients) == 0 {
		return nil, errors.New("at least one recipient is required")
	}

	nl := &Newsletter{
		ID:          generateID(),
		WorkspaceID: workspaceID,
		Subject:     subject,
		Body:        body,
		Recipients:  recipients,
	}

	n.mu.Lock()
	n.newsletters[nl.ID] = nl
	n.mu.Unlock()
	return nl, nil
}

// SendNewsletter sends a previously created newsletter.
func (n *NewsletterService) SendNewsletter(newsletterID string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	nl, ok := n.newsletters[newsletterID]
	if !ok {
		return fmt.Errorf("newsletter %s not found", newsletterID)
	}
	if !nl.SentAt.IsZero() {
		return fmt.Errorf("newsletter %s already sent", newsletterID)
	}
	nl.SentAt = time.Now().UTC()
	nl.OpenRate = 0.0 // Will accumulate over time.
	return nil
}
