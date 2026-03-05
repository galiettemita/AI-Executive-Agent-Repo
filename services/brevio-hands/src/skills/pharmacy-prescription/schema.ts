import { z } from 'zod';

const ActionSchema = z.enum(['medication_lookup', 'refill_request', 'refill_status']);

const MedicationSchema = z.object({
  medication_name: z.string(),
  dosage: z.string(),
  refill_eligible: z.boolean()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    medication_name: z.string().min(2).max(120).optional(),
    prescription_id: z.string().min(2).max(120).optional(),
    pharmacy_name: z.string().min(2).max(120).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'medication_lookup' && !value.medication_name) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'PHARMACY_MEDICATION_NAME_REQUIRED'
      });
    }

    if (value.action === 'refill_request' && (!value.prescription_id || !value.pharmacy_name)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'PHARMACY_REFILL_FIELDS_REQUIRED'
      });
    }

    if (value.action === 'refill_status' && !value.prescription_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'PHARMACY_PRESCRIPTION_ID_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('pharmacy-prescription'),
    action: ActionSchema,
    medications: z.array(MedicationSchema).optional(),
    prescription_id: z.string().optional(),
    status: z.enum(['pending', 'processing', 'ready', 'cancelled']).optional(),
    partnership_status: z.literal('awaiting_api_partnership')
  })
  .strict();
