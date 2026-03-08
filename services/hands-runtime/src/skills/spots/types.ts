export interface SpotsInput {
  query: string;
  area?: string;
  grid_density?: 'low' | 'medium' | 'high';
  max_results?: number;
}

export interface SpotsResult {
  name: string;
  address: string;
  category: string;
  lat: number;
  lng: number;
}

export interface SpotsOutput {
  provider: 'spots';
  grid_density: 'low' | 'medium' | 'high';
  results: SpotsResult[];
}
