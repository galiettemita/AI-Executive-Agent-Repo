export type SwissweatherAction = 'forecast';

export interface SwissweatherInput {
  action: SwissweatherAction;
  location?: string;
}

export interface SwissweatherForecast {
  day: string;
  condition: string;
  high_c: number;
  low_c: number;
}

export interface SwissweatherOutput {
  provider: 'swissweather';
  action: SwissweatherAction;
  forecasts: SwissweatherForecast[];
  summary: string;
}
