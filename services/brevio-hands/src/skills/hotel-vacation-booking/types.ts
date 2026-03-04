export interface HotelVacationBookingInput {
  action: 'search_hotels' | 'hold_room' | 'book_room' | 'reservation_status';
  city?: string;
  check_in?: string;
  check_out?: string;
  guests?: number;
  hotel_id?: string;
  hold_id?: string;
  reservation_id?: string;
  confirmed?: boolean;
}

export interface HotelOption {
  hotel_id: string;
  name: string;
  nightly_rate_cents: number;
  total_cents: number;
  refundable: boolean;
}

export interface HotelVacationBookingOutput {
  provider: 'hotel-vacation-booking';
  action: HotelVacationBookingInput['action'];
  hotels?: HotelOption[];
  hold_id?: string;
  reservation_id?: string;
  status?: 'pending' | 'confirmed' | 'cancelled';
  partnership_status: 'awaiting_api_partnership';
}
