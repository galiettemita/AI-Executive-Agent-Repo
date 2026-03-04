import type { Track17Input, Track17Output } from './types.js';

const CHECKPOINTS = [
  {
    timestamp: '2026-03-01T09:10:00.000Z',
    location: 'Shenzhen, CN',
    status: 'Shipment accepted'
  },
  {
    timestamp: '2026-03-02T02:45:00.000Z',
    location: 'Hong Kong, HK',
    status: 'Departed processing center'
  },
  {
    timestamp: '2026-03-03T11:20:00.000Z',
    location: 'Los Angeles, US',
    status: 'Customs clearance complete'
  }
];

export async function runClient(input: Track17Input): Promise<Track17Output> {
  const trackingNumber = input.tracking_number.trim().toUpperCase();
  const carrier = input.carrier_code?.toUpperCase() ?? '17TRACK_AUTO';

  return {
    provider: '17track',
    tracking_number: trackingNumber,
    carrier,
    status: 'in_transit',
    checkpoints: CHECKPOINTS
  };
}
