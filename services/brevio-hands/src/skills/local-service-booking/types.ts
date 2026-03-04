export interface LocalServiceBookingInput {
  action: 'search_providers' | 'request_quote' | 'book_service' | 'booking_status';
  service_type?: string;
  zip_code?: string;
  provider_id?: string;
  booking_id?: string;
  preferred_time?: string;
  confirmed?: boolean;
}

export interface ServiceProvider {
  provider_id: string;
  name: string;
  service_type: string;
  rating: number;
  estimated_start_cents: number;
}

export interface LocalServiceBookingOutput {
  provider: 'local-service-booking';
  action: LocalServiceBookingInput['action'];
  providers?: ServiceProvider[];
  booking_id?: string;
  status?: 'quote_pending' | 'scheduled' | 'completed' | 'cancelled';
  partnership_status: 'awaiting_api_partnership';
}
