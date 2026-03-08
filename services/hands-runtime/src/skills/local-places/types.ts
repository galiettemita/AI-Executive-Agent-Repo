export interface LocalPlacesInput {
  query: string;
  near?: string;
  radius_km?: number;
  max_results?: number;
}

export interface LocalPlacesResult {
  name: string;
  address: string;
  distance_km: number;
  category: string;
}

export interface LocalPlacesOutput {
  provider: 'local-places';
  results: LocalPlacesResult[];
}
