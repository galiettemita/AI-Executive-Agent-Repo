package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	messageEnvelopeTextMaxChars    = 4096
	messageEnvelopeVoiceDurationMS = 120000
)

type MessageEnvelope struct {
	ID        string                 `json:"id"`
	Channel   string                 `json:"channel"`
	UserID    string                 `json:"user_id"`
	Timestamp string                 `json:"timestamp"`
	Content   MessageEnvelopeContent `json:"content"`
	Metadata  MessageMetadata        `json:"metadata"`
	Context   MessageContext         `json:"context"`
	Routing   *MessageRouting        `json:"routing,omitempty"`
}

type MessageEnvelopeContent struct {
	Type            string               `json:"type"`
	Text            string               `json:"text,omitempty"`
	MediaURL        string               `json:"media_url,omitempty"`
	VoiceDurationMS *int                 `json:"voice_duration_ms,omitempty"`
	Parts           []MessageContentPart `json:"parts,omitempty"`
	MediaAssets     []MediaAsset         `json:"media_assets,omitempty"`
}

type MessageContentPart struct {
	Type    string      `json:"type"`
	Text    string      `json:"text,omitempty"`
	AssetID string      `json:"asset_id,omitempty"`
	Media   *MediaAsset `json:"media,omitempty"`
}

type MediaAsset struct {
	AssetID      string         `json:"asset_id"`
	MIMEType     string         `json:"mime_type"`
	SizeBytes    int64          `json:"size_bytes,omitempty"`
	SHA256       string         `json:"sha256,omitempty"`
	StorageURI   string         `json:"storage_uri,omitempty"`
	SourceURI    string         `json:"source_uri,omitempty"`
	Filename     string         `json:"filename,omitempty"`
	DurationMS   *int           `json:"duration_ms,omitempty"`
	Width        *int           `json:"width,omitempty"`
	Height       *int           `json:"height,omitempty"`
	PageCount    *int           `json:"page_count,omitempty"`
	Codec        string         `json:"codec,omitempty"`
	Provenance   string         `json:"provenance,omitempty"`
	SafetyLabels []string       `json:"safety_labels,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type MessageMetadata struct {
	ChannelMessageID string `json:"channel_message_id"`
	ReplyTo          string `json:"reply_to,omitempty"`
	SessionID        string `json:"session_id"`
}

type MessageContext struct {
	UserProfileHash string   `json:"user_profile_hash"`
	ActiveSkills    []string `json:"active_skills,omitempty"`
}

type MessageRouting struct {
	Intent    string         `json:"intent,omitempty"`
	SkillIDs  []string       `json:"skill_ids,omitempty"`
	TaskGraph map[string]any `json:"task_graph,omitempty"`
}

type BuildMessageEnvelopeInput struct {
	ID               uuid.UUID
	Channel          string
	UserID           uuid.UUID
	Timestamp        time.Time
	MessageText      string
	Transcript       string
	AudioURL         string
	VoiceDurationMS  int
	Attachments      []AttachmentReference
	ChannelMessageID string
	ReplyTo          string
	SessionID        uuid.UUID
	UserProfileHash  string
	ActiveSkills     []string
}

func BuildMessageEnvelope(input BuildMessageEnvelopeInput) (MessageEnvelope, error) {
	channel, err := canonicalMessageChannel(input.Channel)
	if err != nil {
		return MessageEnvelope{}, err
	}
	if input.ID == uuid.Nil {
		return MessageEnvelope{}, fmt.Errorf("id is required")
	}
	if input.UserID == uuid.Nil {
		return MessageEnvelope{}, fmt.Errorf("user_id is required")
	}
	if input.SessionID == uuid.Nil {
		return MessageEnvelope{}, fmt.Errorf("session_id is required")
	}
	timestamp := input.Timestamp.UTC()
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	content, err := buildEnvelopeContent(input)
	if err != nil {
		return MessageEnvelope{}, err
	}

	replyTo := strings.TrimSpace(input.ReplyTo)
	if replyTo != "" {
		if _, err := uuid.Parse(replyTo); err != nil {
			return MessageEnvelope{}, fmt.Errorf("reply_to must be uuid when provided: %w", err)
		}
	}

	channelMessageID := strings.TrimSpace(input.ChannelMessageID)
	if channelMessageID == "" {
		channelMessageID = input.ID.String()
	}

	profileHash := strings.TrimSpace(input.UserProfileHash)
	if profileHash == "" {
		profileHash = DeriveUserProfileHash(input.UserID, input.SessionID)
	}

	envelope := MessageEnvelope{
		ID:        input.ID.String(),
		Channel:   channel,
		UserID:    input.UserID.String(),
		Timestamp: timestamp.Format(time.RFC3339Nano),
		Content:   content,
		Metadata: MessageMetadata{
			ChannelMessageID: channelMessageID,
			ReplyTo:          replyTo,
			SessionID:        input.SessionID.String(),
		},
		Context: MessageContext{
			UserProfileHash: profileHash,
			ActiveSkills:    input.ActiveSkills,
		},
	}
	if err := validateMessageEnvelope(envelope); err != nil {
		return MessageEnvelope{}, err
	}
	return envelope, nil
}

func DecodeMessageEnvelope(raw []byte) (MessageEnvelope, error) {
	var envelope MessageEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return MessageEnvelope{}, err
	}
	if err := validateMessageEnvelope(envelope); err != nil {
		return MessageEnvelope{}, err
	}
	return envelope, nil
}

func (m MessageEnvelope) ContentText() string {
	return strings.TrimSpace(m.Content.Text)
}

func DeriveUserProfileHash(leftID, rightID uuid.UUID) string {
	sum := sha256.Sum256([]byte(leftID.String() + "::" + rightID.String()))
	return hex.EncodeToString(sum[:])
}

func buildEnvelopeContent(input BuildMessageEnvelopeInput) (MessageEnvelopeContent, error) {
	text := strings.TrimSpace(input.MessageText)
	if transcript := strings.TrimSpace(input.Transcript); transcript != "" {
		text = transcript
	}

	mediaURL := ""
	contentType := "TEXT"
	parts := make([]MessageContentPart, 0, 1+len(input.Attachments))
	assets := make([]MediaAsset, 0, len(input.Attachments)+1)
	if text != "" {
		parts = append(parts, MessageContentPart{
			Type: "text",
			Text: text,
		})
	}

	audioURL := normalizeHTTPSURL(input.AudioURL)
	if audioURL != "" {
		contentType = "VOICE"
		mediaURL = audioURL
		asset := MediaAsset{
			AssetID:    "voice-" + DeriveStableAssetID(audioURL),
			MIMEType:   "audio/ogg",
			SourceURI:  audioURL,
			Provenance: "user_message",
		}
		if input.VoiceDurationMS > 0 {
			duration := input.VoiceDurationMS
			asset.DurationMS = &duration
		}
		assets = append(assets, asset)
		parts = append(parts, MessageContentPart{
			Type:    "audio",
			AssetID: asset.AssetID,
			Media:   &asset,
		})
	} else if len(input.Attachments) > 0 {
		contentType = legacyContentTypeForMime(input.Attachments[0].MIMEType)
		mediaURL = normalizeHTTPSURL(input.Attachments[0].SourceURL)
		for _, attachment := range input.Attachments {
			asset := mediaAssetFromAttachment(attachment)
			assets = append(assets, asset)
			parts = append(parts, MessageContentPart{
				Type:    contentPartTypeForMime(attachment.MIMEType),
				AssetID: asset.AssetID,
				Media:   &asset,
			})
		}
	}
	if len(parts) > 1 && contentType != "TEXT" && audioURL == "" {
		contentType = "MULTIMODAL"
	}

	content := MessageEnvelopeContent{
		Type:        contentType,
		Text:        text,
		MediaURL:    mediaURL,
		Parts:       parts,
		MediaAssets: assets,
	}
	if input.VoiceDurationMS > 0 {
		duration := input.VoiceDurationMS
		content.VoiceDurationMS = &duration
	}
	return content, nil
}

func DeriveStableAssetID(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])[:24]
}

func mediaAssetFromAttachment(attachment AttachmentReference) MediaAsset {
	assetID := strings.TrimSpace(attachment.AssetID)
	if assetID == "" {
		assetID = strings.TrimSpace(attachment.ID)
	}
	if assetID == "" {
		assetID = DeriveStableAssetID(attachment.SourceURL + attachment.S3URI)
	}
	return MediaAsset{
		AssetID:      assetID,
		MIMEType:     strings.ToLower(strings.TrimSpace(attachment.MIMEType)),
		SizeBytes:    attachment.SizeBytes,
		SHA256:       strings.TrimSpace(attachment.SHA256),
		StorageURI:   strings.TrimSpace(attachment.S3URI),
		SourceURI:    normalizeHTTPSURL(attachment.SourceURL),
		Filename:     strings.TrimSpace(attachment.Filename),
		Provenance:   "user_message",
		SafetyLabels: []string{},
	}
}

func contentPartTypeForMime(mime string) string {
	normalized := strings.ToLower(strings.TrimSpace(mime))
	switch {
	case strings.HasPrefix(normalized, "image/"):
		return "image"
	case strings.HasPrefix(normalized, "audio/"):
		return "audio"
	case strings.HasPrefix(normalized, "video/"):
		return "video"
	case normalized == "application/pdf" || strings.HasPrefix(normalized, "text/") || strings.Contains(normalized, "spreadsheet") || strings.Contains(normalized, "wordprocessingml"):
		return "document"
	default:
		return "file"
	}
}

func legacyContentTypeForMime(mime string) string {
	switch contentPartTypeForMime(mime) {
	case "image":
		return "IMAGE"
	case "audio":
		return "VOICE"
	case "video":
		return "VIDEO"
	default:
		return "DOCUMENT"
	}
}

func validateMessageEnvelope(envelope MessageEnvelope) error {
	if _, err := uuid.Parse(strings.TrimSpace(envelope.ID)); err != nil {
		return fmt.Errorf("invalid id: %w", err)
	}
	if _, err := canonicalMessageChannel(envelope.Channel); err != nil {
		return err
	}
	if _, err := uuid.Parse(strings.TrimSpace(envelope.UserID)); err != nil {
		return fmt.Errorf("invalid user_id: %w", err)
	}
	if _, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(envelope.Timestamp)); err != nil {
		if _, fallbackErr := time.Parse(time.RFC3339, strings.TrimSpace(envelope.Timestamp)); fallbackErr != nil {
			return fmt.Errorf("invalid timestamp: %w", err)
		}
	}
	switch envelope.Content.Type {
	case "TEXT", "VOICE", "IMAGE", "VIDEO", "DOCUMENT", "LOCATION", "MULTIMODAL":
	default:
		return fmt.Errorf("invalid content.type")
	}
	if len(envelope.Content.Text) > messageEnvelopeTextMaxChars {
		return fmt.Errorf("content.text exceeds %d characters", messageEnvelopeTextMaxChars)
	}
	if strings.TrimSpace(envelope.Content.MediaURL) != "" {
		parsed, err := url.Parse(envelope.Content.MediaURL)
		if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" {
			return fmt.Errorf("content.media_url must be https url")
		}
	}
	if envelope.Content.VoiceDurationMS != nil {
		if *envelope.Content.VoiceDurationMS < 0 || *envelope.Content.VoiceDurationMS > messageEnvelopeVoiceDurationMS {
			return fmt.Errorf("content.voice_duration_ms must be <= %d", messageEnvelopeVoiceDurationMS)
		}
	}
	for _, part := range envelope.Content.Parts {
		if err := validateMessageContentPart(part); err != nil {
			return err
		}
	}
	for _, asset := range envelope.Content.MediaAssets {
		if err := validateMediaAsset(asset); err != nil {
			return err
		}
	}
	if strings.TrimSpace(envelope.Content.Text) == "" && strings.TrimSpace(envelope.Content.MediaURL) == "" && len(envelope.Content.Parts) == 0 {
		return fmt.Errorf("content must include text, media_url, or parts")
	}
	if strings.TrimSpace(envelope.Metadata.ChannelMessageID) == "" {
		return fmt.Errorf("metadata.channel_message_id is required")
	}
	if _, err := uuid.Parse(strings.TrimSpace(envelope.Metadata.SessionID)); err != nil {
		return fmt.Errorf("invalid metadata.session_id: %w", err)
	}
	if strings.TrimSpace(envelope.Metadata.ReplyTo) != "" {
		if _, err := uuid.Parse(strings.TrimSpace(envelope.Metadata.ReplyTo)); err != nil {
			return fmt.Errorf("invalid metadata.reply_to: %w", err)
		}
	}
	profileHash := strings.TrimSpace(envelope.Context.UserProfileHash)
	if len(profileHash) != 64 {
		return fmt.Errorf("context.user_profile_hash must be 64 chars")
	}
	if _, err := hex.DecodeString(profileHash); err != nil {
		return fmt.Errorf("context.user_profile_hash must be hex")
	}
	if len(envelope.Context.ActiveSkills) > 50 {
		return fmt.Errorf("context.active_skills exceeds max length 50")
	}
	return nil
}

func validateMessageContentPart(part MessageContentPart) error {
	switch strings.TrimSpace(part.Type) {
	case "text":
		if strings.TrimSpace(part.Text) == "" {
			return fmt.Errorf("content.parts text part requires text")
		}
	case "image", "audio", "video", "document", "location", "tool_result", "generated_asset", "file":
		if strings.TrimSpace(part.AssetID) == "" && part.Media == nil {
			return fmt.Errorf("content.parts media part requires asset_id or media")
		}
	default:
		return fmt.Errorf("invalid content.parts.type")
	}
	if part.Media != nil {
		if err := validateMediaAsset(*part.Media); err != nil {
			return err
		}
	}
	return nil
}

func validateMediaAsset(asset MediaAsset) error {
	if strings.TrimSpace(asset.AssetID) == "" {
		return fmt.Errorf("media_assets.asset_id is required")
	}
	if strings.TrimSpace(asset.MIMEType) == "" {
		return fmt.Errorf("media_assets.mime_type is required")
	}
	for _, raw := range []string{asset.SourceURI} {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" {
			return fmt.Errorf("media_assets.source_uri must be https url")
		}
	}
	if asset.SizeBytes < 0 {
		return fmt.Errorf("media_assets.size_bytes must be non-negative")
	}
	if strings.TrimSpace(asset.SHA256) != "" {
		if len(asset.SHA256) != 64 {
			return fmt.Errorf("media_assets.sha256 must be 64 chars")
		}
		if _, err := hex.DecodeString(asset.SHA256); err != nil {
			return fmt.Errorf("media_assets.sha256 must be hex")
		}
	}
	return nil
}

func normalizeHTTPSURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" {
		return ""
	}
	return trimmed
}

func canonicalMessageChannel(raw string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "WHATSAPP":
		return "WHATSAPP", nil
	case "IMESSAGE":
		return "IMESSAGE", nil
	case "API":
		return "API", nil
	default:
		return "", fmt.Errorf("invalid channel")
	}
}
