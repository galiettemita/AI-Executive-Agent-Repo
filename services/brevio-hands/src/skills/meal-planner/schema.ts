import { z } from 'zod';

const ActionSchema = z.enum(['weekly_plan', 'grocery_rollup']);

const PlannedMealSchema = z.object({
  day: z.string().min(3).max(12),
  meal_slot: z.enum(['breakfast', 'lunch', 'dinner']),
  name: z.string().min(2).max(120),
  calories_per_serving: z.number().int().min(100).max(1500)
});

const GroceryItemSchema = z.object({
  item: z.string().min(1).max(120),
  quantity: z.string().min(1).max(60),
  section: z.string().min(2).max(60)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    household_size: z.number().int().min(1).max(20).optional(),
    dietary_preferences: z.array(z.string().min(2).max(80)).max(20).optional(),
    calorie_target_per_person: z.number().int().min(1200).max(5000).optional(),
    meals_per_day: z.number().int().min(1).max(4).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.household_size) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'MEAL_PLANNER_HOUSEHOLD_SIZE_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('meal-planner'),
    action: ActionSchema,
    meals: z.array(PlannedMealSchema).max(42),
    grocery_items: z.array(GroceryItemSchema).max(200),
    summary: z.string().min(10).max(4096)
  })
  .strict();
