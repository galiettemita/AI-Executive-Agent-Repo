export interface HomeAssistantInput {
  entity_id: string;
  action: string;
  value?: string | number | boolean;
  two_factor_code?: string;
}

export interface HomeAssistantOutput {
  state: string;
  attributes: Record<string, string | number | boolean>;
}
