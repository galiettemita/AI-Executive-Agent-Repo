export interface SmtpSendInput {
  to: string[];
  subject: string;
  body: string;
  html?: string;
  confirmed?: boolean;
}

export interface SmtpSendOutput {
  message_id: string;
  sent: boolean;
  confirmation_required: boolean;
  recipients: string[];
}
