import type { WhatsAppStylingGuideInput, WhatsAppStylingGuideOutput } from './types.js';

function splitLines(text: string): string[] {
  return text
    .split(/\n|;/)
    .map((line) => line.trim())
    .filter((line) => line.length > 0);
}

function applyStyle(input: WhatsAppStylingGuideInput): { formatted_text: string; applied_rules: string[] } {
  const style = input.style ?? 'default';
  const lines = splitLines(input.text);

  if (style === 'bullet') {
    return {
      formatted_text: lines.map((line) => `• ${line}`).join('\n'),
      applied_rules: ['bullet_list', 'trim_whitespace']
    };
  }

  if (style === 'numbered') {
    return {
      formatted_text: lines.map((line, index) => `${index + 1}. ${line}`).join('\n'),
      applied_rules: ['numbered_list', 'trim_whitespace']
    };
  }

  if (style === 'emphasis') {
    const emphasized = lines.map((line, index) => (index === 0 ? `*${line}*` : line));
    return {
      formatted_text: emphasized.join('\n'),
      applied_rules: ['first_line_emphasis', 'trim_whitespace']
    };
  }

  return {
    formatted_text: lines.join('\n'),
    applied_rules: ['trim_whitespace']
  };
}

export async function runClient(input: WhatsAppStylingGuideInput): Promise<WhatsAppStylingGuideOutput> {
  const { formatted_text, applied_rules } = applyStyle(input);
  return {
    provider: 'whatsapp-styling-guide',
    formatted_text,
    applied_rules,
    char_count: formatted_text.length,
    latency_budget_ms: 10
  };
}
