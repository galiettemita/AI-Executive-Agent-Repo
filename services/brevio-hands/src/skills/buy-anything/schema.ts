import { z } from 'zod';

const ActionSchema = z.enum(['search_product', 'prepare_checkout', 'place_order', 'order_status']);

const LineItemSchema = z.object({
  sku: z.string().min(2).max(80),
  title: z.string().min(2).max(200),
  quantity: z.number().int().min(1).max(50),
  unit_price_cents: z.number().int().min(1).max(500000)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    query: z.string().min(2).max(200).optional(),
    amazon_url: z.string().url().startsWith('https://').optional(),
    quantity: z.number().int().min(1).max(50).optional(),
    max_total_cents: z.number().int().min(1).max(5000000).optional(),
    shipping_address_id: z.string().min(2).max(120).optional(),
    line_items: z.array(LineItemSchema).max(30).optional(),
    order_id: z.string().min(2).max(120).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'search_product' && !value.query?.trim()) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'BUY_ANYTHING_QUERY_REQUIRED' });
    }

    if (value.action === 'prepare_checkout' && !(value.amazon_url || value.line_items?.length)) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'BUY_ANYTHING_CHECKOUT_FIELDS_REQUIRED' });
    }

    if (value.action === 'place_order' && !value.shipping_address_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'BUY_ANYTHING_SHIPPING_REQUIRED' });
    }

    if (value.action === 'order_status' && !value.order_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'BUY_ANYTHING_ORDER_ID_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('buy-anything'),
    action: ActionSchema,
    product_options: z
      .array(
        z.object({
          sku: z.string(),
          title: z.string(),
          price_cents: z.number().int().positive(),
          rating: z.number().min(0).max(5),
          prime_eligible: z.boolean(),
          url: z.string().url()
        })
      )
      .optional(),
    checkout_preview: z
      .object({
        subtotal_cents: z.number().int().nonnegative(),
        shipping_cents: z.number().int().nonnegative(),
        tax_cents: z.number().int().nonnegative(),
        total_cents: z.number().int().nonnegative()
      })
      .optional(),
    order_id: z.string().optional(),
    order_status: z.enum(['pending', 'confirmed', 'shipped', 'delivered', 'cancelled']).optional()
  })
  .strict();
