export interface FlightTrackerInput {
  callsign?: string;
  icao24?: string;
  origin_iata?: string;
  destination_iata?: string;
}

export interface FlightTrackerRecord {
  callsign: string;
  icao24: string;
  origin: string;
  destination: string;
  altitude_m: number;
  speed_kts: number;
  status: 'airborne' | 'scheduled' | 'landed';
}

export interface FlightTrackerOutput {
  provider: 'opensky';
  flights: FlightTrackerRecord[];
  queried_at_utc: string;
}
