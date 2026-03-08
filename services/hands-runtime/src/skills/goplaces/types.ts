export interface GoPlacesInput {
  query: string;
  location?: {
    lat: number;
    lng: number;
    radius_m?: number;
  };
  open_now?: boolean;
  max_results?: number;
}

export interface GoPlacesResult {
  place_id: string;
  name: string;
  formatted_address: string;
  rating?: number;
  open_now?: boolean;
}

export interface GoPlacesOutput {
  provider: 'goplaces';
  results: GoPlacesResult[];
}
