import type { Channel } from './types.js';

function trimToMax(text: string, maxChars: number): string {
  if (text.length <= maxChars) {
    return text;
  }
  return `${text.slice(0, Math.max(0, maxChars - 1)).trimEnd()}…`;
}

function formatWhatsApp(text: string): string {
  const normalized = text
    .replace(/\r\n/g, '\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim();

  return trimToMax(normalized, 4096);
}

function formatIMessage(text: string): string {
  return trimToMax(text.replace(/\r\n/g, '\n').trim(), 4096);
}

export function formatOutboundText(channel: Channel, text: string): string {
  if (channel === 'WHATSAPP') {
    return formatWhatsApp(text);
  }
  if (channel === 'IMESSAGE') {
    return formatIMessage(text);
  }
  return trimToMax(text.trim(), 4096);
}
