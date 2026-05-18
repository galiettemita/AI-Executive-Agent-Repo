import { z } from 'zod';

const ActionSchema = z.enum(['search_restaurants', 'build_cart', 'checkout', 'order_status']);

const RestaurantSchema = z.object({
  restaurant_id: z.string(),
  name: z.string(),
  cuisine: z.string(),
  eta_minutes: z.number().int().positive()
});

const ItemSchema = z.object({
  item_id: z.string().min(2).max(80),
  quantity: z.number().int().min(1).max(20)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    address: z.string().min(5).max(200).optional(),
    cuisine: z.string().min(2).max(80).optional(),
    restaurant_id: z.string().min(2).max(120).optional(),
    items: z.array(ItemSchema).max(100).optional(),
    cart_id: z.string().min(2).max(120).optional(),
    order_id: z.string().min(2).max(120).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'search_restaurants' && !value.address) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'FOOD_DELIVERY_ADDRESS_REQUIRED'
      });
    }

    if (value.action === 'build_cart' && (!value.restaurant_id || !value.items?.length)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'FOOD_DELIVERY_CART_FIELDS_REQUIRED'
      });
    }

    if (value.action === 'checkout' && !value.cart_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'FOOD_DELIVERY_CART_ID_REQUIRED'
      });
    }

    if (value.action === 'order_status' && !value.order_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'FOOD_DELIVERY_ORDER_ID_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('food-delivery-ordering'),
    action: ActionSchema,
    restaurants: z.array(RestaurantSchema).optional(),
    cart_id: z.string().optional(),
    order_id: z.string().optional(),
    status: z.enum(['pending', 'confirmed', 'delivered', 'cancelled']).optional(),
    estimated_total_cents: z.number().int().nonnegative().optional(),
    partnership_status: z.literal('awaiting_api_partnership')
  })
  .strict();
