import { createHash } from 'node:crypto';

import type { SmtpSendInput, SmtpSendOutput } from './types.js';

function buildMessageId(input: SmtpSendInput): string {
  const digest = createHash('sha256')
    .update(input.to.join(','))
    .update('|')
    .update(input.subject)
    .update('|')
    .update(input.body.slice(0, 200))
    .digest('hex')
    .slice(0, 16);

  return `msg_${digest}`;
}

export async function runClient(input: SmtpSendInput): Promise<SmtpSendOutput> {
  const messageId = buildMessageId(input);

  if (!input.confirmed) {
    return {
      message_id: messageId,
      sent: false,
      confirmation_required: true,
      recipients: input.to
    };
  }

  return {
    message_id: messageId,
    sent: true,
    confirmation_required: false,
    recipients: input.to
  };
}
