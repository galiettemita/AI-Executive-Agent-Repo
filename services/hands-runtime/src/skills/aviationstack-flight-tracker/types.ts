export interface AviationstackFlightTrackerInput {
  flight_iata?: string;
  flight_icao?: string;
  airline_iata?: string;
  date?: string;
}

export interface AviationstackFlightRecord {
  flight: string;
  airline: string;
  status: 'scheduled' | 'active' | 'landed' | 'delayed';
  departure_airport: string;
  arrival_airport: string;
  gate?: string;
  terminal?: string;
  delay_minutes?: number;
}

export interface AviationstackFlightTrackerOutput {
  provider: 'aviationstack';
  flights: AviationstackFlightRecord[];
  queried_at_utc: string;
}
