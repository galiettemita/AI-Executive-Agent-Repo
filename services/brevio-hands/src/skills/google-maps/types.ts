export type TravelMode = 'driving' | 'walking' | 'bicycling' | 'transit';

export interface GoogleMapsInput {
  origin: string;
  destination: string;
  mode?: TravelMode;
}

export interface RouteStep {
  instruction: string;
  distance_m: number;
}

export interface GoogleMapsOutput {
  distance_m: number;
  duration_s: number;
  mode: TravelMode;
  steps: RouteStep[];
}
