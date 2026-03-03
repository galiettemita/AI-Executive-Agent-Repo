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
	Type            string `json:"type"`
	Text            string `json:"text,omitempty"`
	MediaURL        string `json:"media_url,omitempty"`
	VoiceDurationMS *int   `json:"voice_duration_ms,omitempty"`
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

	audioURL := strings.TrimSpace(input.AudioURL)
	if audioURL != "" {
		contentType = "VOICE"
		mediaURL = audioURL
	} else if len(input.Attachments) > 0 {
		first := input.Attachments[0]
		mediaURL = strings.TrimSpace(first.SourceURL)
		mime := strings.ToLower(strings.TrimSpace(first.MIMEType))
		if strings.HasPrefix(mime, "image/") {
			contentType = "IMAGE"
		} else {
			contentType = "DOCUMENT"
		}
	}

	content := MessageEnvelopeContent{
		Type:     contentType,
		Text:     text,
		MediaURL: mediaURL,
	}
	if input.VoiceDurationMS > 0 {
		duration := input.VoiceDurationMS
		content.VoiceDurationMS = &duration
	}
	return content, nil
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
	case "TEXT", "VOICE", "IMAGE", "DOCUMENT", "LOCATION":
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
	if strings.TrimSpace(envelope.Content.Text) == "" && strings.TrimSpace(envelope.Content.MediaURL) == "" {
		return fmt.Errorf("content must include text or media_url")
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
