// Plan §6 step 24 — Real nodemailer SMTP
// Env vars: SMTP_HOST, SMTP_USER, SMTP_PASS (all three required per plan)

import * as nodemailer from 'nodemailer';

import type { SmtpSendInput, SmtpSendOutput } from './types.js';

// ctx accepted for interface compatibility (plan §4 arch); SMTP does not use OAuth
export async function runClient(
  input: SmtpSendInput,
  ctx?: { token?: string }
): Promise<SmtpSendOutput> {
  // Plan §6 step 24: "check SMTP_HOST+SMTP_USER+SMTP_PASS"
  const { SMTP_HOST, SMTP_USER, SMTP_PASS } = process.env;
  if (!SMTP_HOST || !SMTP_USER || !SMTP_PASS) {
    throw new Error('smtp-send: SMTP_HOST, SMTP_USER, and SMTP_PASS must be set');
  }

  // Plan §6 step 24: "check input.confirmed===true"
  if (!input.confirmed) {
    return {
      message_id: '',
      sent: false,
      confirmation_required: true,
      recipients: input.to,
    };
  }

  // Plan §6 step 24: createTransport({host, port:587, auth:{user,pass}})
  const transporter = nodemailer.createTransport({
    host: SMTP_HOST,
    port: 587,
    secure: false,
    auth: {
      user: SMTP_USER,
      pass: SMTP_PASS,
    },
  });

  // Plan §6 step 24: sendMail({from:user, to, subject, text:body, html})
  const info = await transporter.sendMail({
    from: SMTP_USER,
    to: input.to.join(', '),
    subject: input.subject,
    text: input.body,
    html: input.html,
  });

  // Plan §6 step 24: return {message_id:info.messageId, sent:true}
  return {
    message_id: info.messageId,
    sent: true,
    confirmation_required: false,
    recipients: input.to,
  };
}
