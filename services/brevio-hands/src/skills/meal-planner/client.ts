import type { MealPlannerInput, MealPlannerOutput, PlannedMeal } from './types.js';

const WEEK_DAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];

function mealTemplates(preferences: string[]): { breakfast: string; lunch: string; dinner: string } {
  const vegetarian = preferences.some((preference) => /vegetarian|vegan/i.test(preference));
  if (vegetarian) {
    return {
      breakfast: 'Greek yogurt with berries',
      lunch: 'Quinoa bowl with roasted vegetables',
      dinner: 'Lentil curry with brown rice'
    };
  }

  return {
    breakfast: 'Egg scramble with spinach',
    lunch: 'Chicken grain bowl',
    dinner: 'Salmon with roasted vegetables'
  };
}

function buildMeals(input: MealPlannerInput): PlannedMeal[] {
  const preferences = input.dietary_preferences ?? [];
  const templates = mealTemplates(preferences);
  const targetCalories = input.calorie_target_per_person ?? 2200;

  return WEEK_DAYS.flatMap((day) => [
    {
      day,
      meal_slot: 'breakfast' as const,
      name: templates.breakfast,
      calories_per_serving: Math.round(targetCalories * 0.25)
    },
    {
      day,
      meal_slot: 'lunch' as const,
      name: templates.lunch,
      calories_per_serving: Math.round(targetCalories * 0.35)
    },
    {
      day,
      meal_slot: 'dinner' as const,
      name: templates.dinner,
      calories_per_serving: Math.round(targetCalories * 0.4)
    }
  ]);
}

function buildGroceryItems(input: MealPlannerInput): MealPlannerOutput['grocery_items'] {
  const householdSize = input.household_size ?? 1;
  return [
    { item: 'Eggs', quantity: `${householdSize * 12} count`, section: 'Dairy' },
    { item: 'Spinach', quantity: `${householdSize * 2} bags`, section: 'Produce' },
    { item: 'Chicken breast', quantity: `${householdSize * 2} lb`, section: 'Protein' },
    { item: 'Quinoa', quantity: `${householdSize} bags`, section: 'Pantry' },
    { item: 'Salmon fillets', quantity: `${householdSize * 4} pieces`, section: 'Protein' }
  ];
}

export async function runClient(input: MealPlannerInput): Promise<MealPlannerOutput> {
  const meals = buildMeals(input);
  const grocery_items = buildGroceryItems(input);

  return {
    provider: 'meal-planner',
    action: input.action,
    meals,
    grocery_items,
    summary: `Generated a 7-day meal plan and ${grocery_items.length} grocery rollup items for household size ${input.household_size}.`
  };
}
