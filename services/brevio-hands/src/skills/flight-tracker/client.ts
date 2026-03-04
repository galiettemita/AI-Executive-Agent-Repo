import type { FlightTrackerInput, FlightTrackerOutput, FlightTrackerRecord } from './types.js';

const FLIGHTS: FlightTrackerRecord[] = [
  {
    callsign: 'AAL100',
    icao24: 'a1b2c3',
    origin: 'JFK',
    destination: 'LAX',
    altitude_m: 10972,
    speed_kts: 468,
    status: 'airborne'
  },
  {
    callsign: 'UAL201',
    icao24: 'd4e5f6',
    origin: 'SFO',
    destination: 'ORD',
    altitude_m: 11200,
    speed_kts: 482,
    status: 'airborne'
  },
  {
    callsign: 'DAL330',
    icao24: '0a1b2c',
    origin: 'ATL',
    destination: 'MIA',
    altitude_m: 0,
    speed_kts: 0,
    status: 'scheduled'
  }
];

export async function runClient(input: FlightTrackerInput): Promise<FlightTrackerOutput> {
  const callsign = input.callsign?.toUpperCase();
  const icao24 = input.icao24?.toLowerCase();
  const origin = input.origin_iata?.toUpperCase();
  const destination = input.destination_iata?.toUpperCase();

  const flights = FLIGHTS.filter((flight) => {
    if (callsign && flight.callsign !== callsign) {
      return false;
    }
    if (icao24 && flight.icao24 !== icao24) {
      return false;
    }
    if (origin && flight.origin !== origin) {
      return false;
    }
    if (destination && flight.destination !== destination) {
      return false;
    }
    return true;
  });

  return {
    provider: 'opensky',
    flights,
    queried_at_utc: new Date().toISOString()
  };
}
