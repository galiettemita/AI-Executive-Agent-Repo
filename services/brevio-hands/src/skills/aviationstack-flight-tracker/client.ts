import type {
  AviationstackFlightRecord,
  AviationstackFlightTrackerInput,
  AviationstackFlightTrackerOutput
} from './types.js';

const FLIGHTS: AviationstackFlightRecord[] = [
  {
    flight: 'AA100',
    airline: 'American Airlines',
    status: 'active',
    departure_airport: 'John F. Kennedy Intl',
    arrival_airport: 'Los Angeles Intl',
    gate: '31',
    terminal: '8'
  },
  {
    flight: 'UA201',
    airline: 'United Airlines',
    status: 'delayed',
    departure_airport: 'San Francisco Intl',
    arrival_airport: 'Chicago O\'Hare Intl',
    delay_minutes: 42,
    gate: 'F12',
    terminal: '3'
  },
  {
    flight: 'DL330',
    airline: 'Delta Air Lines',
    status: 'scheduled',
    departure_airport: 'Hartsfield-Jackson Atlanta Intl',
    arrival_airport: 'Miami Intl',
    gate: 'A11',
    terminal: 'S'
  }
];

export async function runClient(
  input: AviationstackFlightTrackerInput
): Promise<AviationstackFlightTrackerOutput> {
  const flightFilter = (input.flight_iata ?? input.flight_icao ?? '').toUpperCase();
  const airlineFilter = input.airline_iata?.toUpperCase();

  const flights = FLIGHTS.filter((flight) => {
    if (flightFilter && flight.flight !== flightFilter) {
      return false;
    }
    if (airlineFilter && !flight.airline.toUpperCase().includes(airlineFilter)) {
      return false;
    }
    return true;
  });

  return {
    provider: 'aviationstack',
    flights,
    queried_at_utc: new Date().toISOString()
  };
}
