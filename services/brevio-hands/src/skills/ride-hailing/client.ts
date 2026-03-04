import type { RideEstimate, RideHailingInput, RideHailingOutput } from './types.js';

const ESTIMATES: RideEstimate[] = [
  { service_tier: 'economy', eta_minutes: 5, fare_low_cents: 1450, fare_high_cents: 2150 },
  { service_tier: 'comfort', eta_minutes: 7, fare_low_cents: 2250, fare_high_cents: 3150 },
  { service_tier: 'xl', eta_minutes: 9, fare_low_cents: 3250, fare_high_cents: 4550 }
];

export async function runClient(input: RideHailingInput): Promise<RideHailingOutput> {
  if (input.action === 'estimate') {
    return {
      provider: 'ride-hailing',
      action: 'estimate',
      estimates: ESTIMATES,
      partnership_status: 'awaiting_api_partnership'
    };
  }

  if (input.action === 'request_ride') {
    return {
      provider: 'ride-hailing',
      action: 'request_ride',
      ride_id: 'ride_001',
      status: 'driver_assigned',
      partnership_status: 'awaiting_api_partnership'
    };
  }

  if (input.action === 'cancel_ride') {
    return {
      provider: 'ride-hailing',
      action: 'cancel_ride',
      ride_id: input.ride_id,
      status: 'cancelled',
      partnership_status: 'awaiting_api_partnership'
    };
  }

  return {
    provider: 'ride-hailing',
    action: 'ride_status',
    ride_id: input.ride_id,
    status: 'arriving',
    partnership_status: 'awaiting_api_partnership'
  };
}
