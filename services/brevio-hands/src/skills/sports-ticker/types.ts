export type SportsTickerAction = 'get_score' | 'get_schedule';

export interface SportsTickerInput {
  action: SportsTickerAction;
  league?: 'nba' | 'nfl' | 'mlb' | 'nhl' | 'epl';
  team?: string;
}

export interface SportsTickerItem {
  title: string;
  status: string;
}

export interface SportsTickerOutput {
  provider: 'sports-ticker';
  action: SportsTickerAction;
  items: SportsTickerItem[];
  summary: string;
}
