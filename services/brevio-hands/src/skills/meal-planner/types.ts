export type MealPlannerAction = 'weekly_plan' | 'grocery_rollup';

export interface MealPlannerInput {
  action: MealPlannerAction;
  household_size?: number;
  dietary_preferences?: string[];
  calorie_target_per_person?: number;
  meals_per_day?: number;
}

export interface PlannedMeal {
  day: string;
  meal_slot: 'breakfast' | 'lunch' | 'dinner';
  name: string;
  calories_per_serving: number;
}

export interface MealPlannerOutput {
  provider: 'meal-planner';
  action: MealPlannerAction;
  meals: PlannedMeal[];
  grocery_items: Array<{ item: string; quantity: string; section: string }>;
  summary: string;
}
