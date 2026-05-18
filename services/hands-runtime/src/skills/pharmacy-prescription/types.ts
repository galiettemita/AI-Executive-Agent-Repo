export interface PharmacyPrescriptionInput {
  action: 'medication_lookup' | 'refill_request' | 'refill_status';
  medication_name?: string;
  prescription_id?: string;
  pharmacy_name?: string;
  confirmed?: boolean;
}

export interface MedicationOption {
  medication_name: string;
  dosage: string;
  refill_eligible: boolean;
}

export interface PharmacyPrescriptionOutput {
  provider: 'pharmacy-prescription';
  action: PharmacyPrescriptionInput['action'];
  medications?: MedicationOption[];
  prescription_id?: string;
  status?: 'pending' | 'processing' | 'ready' | 'cancelled';
  partnership_status: 'awaiting_api_partnership';
}
