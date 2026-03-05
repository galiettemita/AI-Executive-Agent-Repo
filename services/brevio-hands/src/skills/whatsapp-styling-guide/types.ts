export interface WhatsAppStylingGuideInput {
  text: string;
  style?: 'default' | 'bullet' | 'numbered' | 'emphasis';
}

export interface WhatsAppStylingGuideOutput {
  provider: 'whatsapp-styling-guide';
  formatted_text: string;
  applied_rules: string[];
  char_count: number;
  latency_budget_ms: 10;
}
