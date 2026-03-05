export interface Track17Input {
  tracking_number: string;
  carrier_code?: string;
  request_locale?: string;
}

export interface Track17Checkpoint {
  timestamp: string;
  location: string;
  status: string;
}

export interface Track17Output {
  provider: '17track';
  tracking_number: string;
  carrier: string;
  status: 'not_found' | 'in_transit' | 'out_for_delivery' | 'delivered';
  checkpoints: Track17Checkpoint[];
}
