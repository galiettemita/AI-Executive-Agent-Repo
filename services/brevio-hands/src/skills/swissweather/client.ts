import type { SwissweatherInput, SwissweatherOutput } from './types.js';

export async function runClient(input: SwissweatherInput): Promise<SwissweatherOutput> {
  const forecasts = [
    { day: 'Thursday', condition: 'Partly cloudy', high_c: 17, low_c: 8 },
    { day: 'Friday', condition: 'Light rain', high_c: 15, low_c: 7 }
  ];

  return {
    provider: 'swissweather',
    action: 'forecast',
    forecasts,
    summary: `Prepared ${forecasts.length}-day Swiss weather forecast for ${input.location}.`
  };
}
