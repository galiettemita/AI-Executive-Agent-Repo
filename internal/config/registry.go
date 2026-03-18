package config

func RequiredSecretKeys() []string {
	return []string{
		"app_secret",
		"encryption_key_current",
		"whatsapp_system_token",
		"whatsapp_app_secret",
		"imessage_client_cert",
		"imessage_api_key",
		"imessage_webhook_secret",
		"anthropic_api_key",
		"openai_api_key",
		"oauth_client_secret_google",
		"oauth_client_secret_microsoft",
		"oauth_client_secret_slack",
		"oauth_state_signing_key",
		"watermark_hmac_key",
	}
}

func RequiredEnvVars() []string {
	return []string{
		"BREVIO_ENV",
		"DATABASE_URL",
		"REDIS_URL",
		"TEMPORAL_HOST",
		"TEMPORAL_NAMESPACE",
		"SQS_INTERACTIVE_TURNS_URL",
		"S3_ATTACHMENTS_BUCKET",
		"S3_SBOMS_BUCKET",
		"WHATSAPP_PHONE_NUMBER_ID",
		"WHATSAPP_API_VERSION",
		"IMESSAGE_MSP_BASE_URL",
		"IMESSAGE_BUSINESS_ID",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OPA_URL",
		"CANVAS_WS_PORT",
		"LOG_LEVEL",
	}
}
