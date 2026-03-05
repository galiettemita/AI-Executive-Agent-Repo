import type { HotelOption, HotelVacationBookingInput, HotelVacationBookingOutput } from './types.js';

const HOTELS: HotelOption[] = [
  {
    hotel_id: 'hotel_001',
    name: 'Riverside Suites',
    nightly_rate_cents: 18900,
    total_cents: 37800,
    refundable: true
  },
  {
    hotel_id: 'hotel_002',
    name: 'Summit Grand Hotel',
    nightly_rate_cents: 22900,
    total_cents: 45800,
    refundable: false
  }
];

export async function runClient(
  input: HotelVacationBookingInput
): Promise<HotelVacationBookingOutput> {
  if (input.action === 'search_hotels') {
    return {
      provider: 'hotel-vacation-booking',
      action: 'search_hotels',
      hotels: HOTELS,
      partnership_status: 'awaiting_api_partnership'
    };
  }

  if (input.action === 'hold_room') {
    return {
      provider: 'hotel-vacation-booking',
      action: 'hold_room',
      hold_id: 'hold_hotel_001',
      status: 'pending',
      partnership_status: 'awaiting_api_partnership'
    };
  }

  if (input.action === 'book_room') {
    return {
      provider: 'hotel-vacation-booking',
      action: 'book_room',
      reservation_id: 'reservation_hotel_001',
      status: 'confirmed',
      partnership_status: 'awaiting_api_partnership'
    };
  }

  return {
    provider: 'hotel-vacation-booking',
    action: 'reservation_status',
    reservation_id: input.reservation_id,
    status: 'confirmed',
    partnership_status: 'awaiting_api_partnership'
  };
}
