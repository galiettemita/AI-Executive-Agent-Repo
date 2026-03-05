export type Last30DaysAction = 'scan_topic';

export interface Last30DaysInput {
  action: Last30DaysAction;
  query?: string;
}

export interface Last30DaysOutput {
  provider: 'last30days';
  action: Last30DaysAction;
  highlights: string[];
  sources: string[];
  summary: string;
}
