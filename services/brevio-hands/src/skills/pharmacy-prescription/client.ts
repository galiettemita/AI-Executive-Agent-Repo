import type {
  MedicationOption,
  PharmacyPrescriptionInput,
  PharmacyPrescriptionOutput
} from './types.js';

const MEDICATIONS: MedicationOption[] = [
  {
    medication_name: 'Atorvastatin',
    dosage: '20mg',
    refill_eligible: true
  },
  {
    medication_name: 'Lisinopril',
    dosage: '10mg',
    refill_eligible: true
  }
];

export async function runClient(
  input: PharmacyPrescriptionInput
): Promise<PharmacyPrescriptionOutput> {
  if (input.action === 'medication_lookup') {
    return {
      provider: 'pharmacy-prescription',
      action: 'medication_lookup',
      medications: MEDICATIONS,
      partnership_status: 'awaiting_api_partnership'
    };
  }

  if (input.action === 'refill_request') {
    return {
      provider: 'pharmacy-prescription',
      action: 'refill_request',
      prescription_id: input.prescription_id,
      status: 'processing',
      partnership_status: 'awaiting_api_partnership'
    };
  }

  return {
    provider: 'pharmacy-prescription',
    action: 'refill_status',
    prescription_id: input.prescription_id,
    status: 'ready',
    partnership_status: 'awaiting_api_partnership'
  };
}
