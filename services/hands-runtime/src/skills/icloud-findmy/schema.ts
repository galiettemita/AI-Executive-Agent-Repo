import { z } from 'zod';

const DeviceSchema = z.object({
  name: z.string(),
  latitude: z.number().min(-90).max(90),
  longitude: z.number().min(-180).max(180),
  battery: z.number().min(0).max(100)
});

export const InputSchema = z
  .object({
    device_name: z.string().min(1).max(120).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('icloud-findmy'),
    devices: z.array(DeviceSchema)
  })
  .strict();
