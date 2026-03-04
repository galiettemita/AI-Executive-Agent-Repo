export interface RestaurantReservationsInput {
  action: 'search' | 'hold' | 'book' | 'reservation_status';
  city?: string;
  date?: string;
  time?: string;
  party_size?: number;
  cuisine?: string;
  restaurant_id?: string;
  hold_id?: string;
  reservation_id?: string;
  confirmed?: boolean;
}

export interface ReservationOption {
  restaurant_id: string;
  name: string;
  cuisine: string;
  available_time: string;
  estimated_total_cents: number;
}

export interface RestaurantReservationsOutput {
  provider: 'restaurant-reservations';
  action: RestaurantReservationsInput['action'];
  options?: ReservationOption[];
  hold_id?: string;
  reservation_id?: string;
  status?: 'pending' | 'confirmed' | 'cancelled';
  partnership_status: 'awaiting_api_partnership';
}
