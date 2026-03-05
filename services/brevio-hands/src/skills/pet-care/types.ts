export interface PetCareInput {
  action: 'providers' | 'book_visit' | 'booking_status';
  pet_type?: string;
  service_type?: string;
  provider_id?: string;
  booking_id?: string;
  confirmed?: boolean;
}

export interface PetCareProvider {
  provider_id: string;
  name: string;
  service_type: string;
  rating: number;
}

export interface PetCareOutput {
  provider: 'pet-care';
  action: PetCareInput['action'];
  providers?: PetCareProvider[];
  booking_id?: string;
  status?: 'scheduled' | 'in_progress' | 'completed' | 'cancelled';
  partnership_status: 'awaiting_api_partnership';
}
