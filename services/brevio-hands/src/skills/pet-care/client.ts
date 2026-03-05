import type { PetCareInput, PetCareOutput, PetCareProvider } from './types.js';

const PROVIDERS: PetCareProvider[] = [
  {
    provider_id: 'pet_001',
    name: 'Paws & Play',
    service_type: 'daycare',
    rating: 4.8
  },
  {
    provider_id: 'pet_002',
    name: 'Happy Tails Vet Home Visit',
    service_type: 'vet_visit',
    rating: 4.9
  }
];

export async function runClient(input: PetCareInput): Promise<PetCareOutput> {
  if (input.action === 'providers') {
    return {
      provider: 'pet-care',
      action: 'providers',
      providers: PROVIDERS,
      partnership_status: 'awaiting_api_partnership'
    };
  }

  if (input.action === 'book_visit') {
    return {
      provider: 'pet-care',
      action: 'book_visit',
      booking_id: 'pet_booking_001',
      status: 'scheduled',
      partnership_status: 'awaiting_api_partnership'
    };
  }

  return {
    provider: 'pet-care',
    action: 'booking_status',
    booking_id: input.booking_id,
    status: 'scheduled',
    partnership_status: 'awaiting_api_partnership'
  };
}
