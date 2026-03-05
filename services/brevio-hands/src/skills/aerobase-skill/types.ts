export type AerobaseAction = 'search_flights' | 'compare_itineraries';

export interface AerobaseInput {
  action: AerobaseAction;
  origin?: string;
  destination?: string;
  depart_date?: string;
}

export interface AerobaseItinerary {
  flight_number: string;
  duration_minutes: number;
  price_usd: number;
  jetlag_score: number;
}

export interface AerobaseOutput {
  provider: 'aerobase-skill';
  action: AerobaseAction;
  itineraries: AerobaseItinerary[];
  summary: string;
}
