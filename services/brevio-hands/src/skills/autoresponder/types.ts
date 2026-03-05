export interface AutoresponderInput {
  action: 'enable' | 'disable' | 'intercept';
  ruleset_name?: string;
  incoming_text?: string;
  channel?: 'whatsapp' | 'imessage';
  delegation_enabled?: boolean;
}

export interface AutoresponderOutput {
  provider: 'autoresponder';
  action: 'enable' | 'disable' | 'intercept';
  status: 'enabled' | 'disabled' | 'responded';
  delegated_to_brain: boolean;
  response_text?: string;
  latency_budget_ms: 8000;
}
