import type {
  LocalServiceBookingInput,
  LocalServiceBookingOutput,
  ServiceProvider
} from './types.js';

const PROVIDERS: ServiceProvider[] = [
  {
    provider_id: 'svc_001',
    name: 'Metro Handyman Co.',
    service_type: 'handyman',
    rating: 4.7,
    estimated_start_cents: 9500
  },
  {
    provider_id: 'svc_002',
    name: 'QuickSpark Electric',
    service_type: 'electrician',
    rating: 4.8,
    estimated_start_cents: 14500
  }
];

export async function runClient(
  input: LocalServiceBookingInput
): Promise<LocalServiceBookingOutput> {
  if (input.action === 'search_providers') {
    return {
      provider: 'local-service-booking',
      action: 'search_providers',
      providers: PROVIDERS,
      partnership_status: 'awaiting_api_partnership'
    };
  }

  if (input.action === 'request_quote') {
    return {
      provider: 'local-service-booking',
      action: 'request_quote',
      booking_id: 'booking_local_001',
      status: 'quote_pending',
      partnership_status: 'awaiting_api_partnership'
    };
  }

  if (input.action === 'book_service') {
    return {
      provider: 'local-service-booking',
      action: 'book_service',
      booking_id: 'booking_local_001',
      status: 'scheduled',
      partnership_status: 'awaiting_api_partnership'
    };
  }

  return {
    provider: 'local-service-booking',
    action: 'booking_status',
    booking_id: input.booking_id,
    status: 'scheduled',
    partnership_status: 'awaiting_api_partnership'
  };
}
