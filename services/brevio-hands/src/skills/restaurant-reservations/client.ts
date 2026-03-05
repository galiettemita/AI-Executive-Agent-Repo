import type {
  ReservationOption,
  RestaurantReservationsInput,
  RestaurantReservationsOutput
} from './types.js';

const OPTIONS: ReservationOption[] = [
  {
    restaurant_id: 'rest_001',
    name: 'Oak & Ember',
    cuisine: 'New American',
    available_time: '19:00',
    estimated_total_cents: 9200
  },
  {
    restaurant_id: 'rest_002',
    name: 'Saffron Table',
    cuisine: 'Mediterranean',
    available_time: '19:30',
    estimated_total_cents: 8700
  }
];

export async function runClient(
  input: RestaurantReservationsInput
): Promise<RestaurantReservationsOutput> {
  if (input.action === 'search') {
    return {
      provider: 'restaurant-reservations',
      action: 'search',
      options: OPTIONS,
      partnership_status: 'awaiting_api_partnership'
    };
  }

  if (input.action === 'hold') {
    return {
      provider: 'restaurant-reservations',
      action: 'hold',
      hold_id: 'hold_rest_001',
      status: 'pending',
      partnership_status: 'awaiting_api_partnership'
    };
  }

  if (input.action === 'book') {
    return {
      provider: 'restaurant-reservations',
      action: 'book',
      reservation_id: 'reservation_rest_001',
      status: 'confirmed',
      partnership_status: 'awaiting_api_partnership'
    };
  }

  return {
    provider: 'restaurant-reservations',
    action: 'reservation_status',
    reservation_id: input.reservation_id,
    status: 'confirmed',
    partnership_status: 'awaiting_api_partnership'
  };
}
