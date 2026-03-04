import type {
  ParcelPackageTrackingInput,
  ParcelPackageTrackingOutput,
  ParcelHistoryEvent
} from './types.js';

const HISTORY: Record<string, ParcelHistoryEvent[]> = {
  '1Z999AA10123456784': [
    {
      timestamp: '2026-02-28T13:15:00.000Z',
      location: 'Newark, NJ',
      description: 'Shipment picked up'
    },
    {
      timestamp: '2026-03-01T09:05:00.000Z',
      location: 'Louisville, KY',
      description: 'In transit'
    },
    {
      timestamp: '2026-03-02T12:25:00.000Z',
      location: 'Austin, TX',
      description: 'Out for delivery'
    }
  ]
};

function inferCarrier(trackingNumber: string): 'ups' | 'usps' | 'fedex' | 'dhl' {
  if (trackingNumber.startsWith('1Z')) {
    return 'ups';
  }
  if (/^94\d{20,}$/u.test(trackingNumber)) {
    return 'usps';
  }
  if (/^\d{12,15}$/u.test(trackingNumber)) {
    return 'fedex';
  }
  return 'dhl';
}

export async function runClient(
  input: ParcelPackageTrackingInput
): Promise<ParcelPackageTrackingOutput> {
  const normalized = input.tracking_number.trim().toUpperCase();
  const carrier = input.carrier && input.carrier !== 'auto' ? input.carrier : inferCarrier(normalized);
  const history = HISTORY[normalized] ?? [
    {
      timestamp: '2026-03-01T08:00:00.000Z',
      location: 'Carrier facility',
      description: 'Label created'
    }
  ];

  const status = history[history.length - 1]?.description.toLowerCase().includes('delivery')
    ? 'out_for_delivery'
    : 'in_transit';

  return {
    provider: 'parcel',
    tracking_number: normalized,
    carrier,
    status,
    eta: '2026-03-03T18:00:00.000Z',
    history
  };
}
