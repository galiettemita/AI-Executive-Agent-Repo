export interface RideHailingInput {
  action: 'estimate' | 'request_ride' | 'ride_status' | 'cancel_ride';
  origin?: string;
  destination?: string;
  service_tier?: 'economy' | 'comfort' | 'xl';
  ride_id?: string;
  confirmed?: boolean;
}

export interface RideEstimate {
  service_tier: 'economy' | 'comfort' | 'xl';
  eta_minutes: number;
  fare_low_cents: number;
  fare_high_cents: number;
}

export interface RideHailingOutput {
  provider: 'ride-hailing';
  action: RideHailingInput['action'];
  estimates?: RideEstimate[];
  ride_id?: string;
  status?: 'requested' | 'driver_assigned' | 'arriving' | 'in_progress' | 'completed' | 'cancelled';
  partnership_status: 'awaiting_api_partnership';
}
