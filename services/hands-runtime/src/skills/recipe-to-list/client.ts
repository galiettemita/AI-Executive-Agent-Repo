import type { RecipeIngredient, RecipeToListInput, RecipeToListOutput } from './types.js';

function inferSection(item: string): RecipeIngredient['section'] {
  const lower = item.toLowerCase();
  if (lower.includes('chicken') || lower.includes('beef') || lower.includes('tofu')) {
    return 'protein';
  }
  if (lower.includes('milk') || lower.includes('cheese') || lower.includes('yogurt')) {
    return 'dairy';
  }
  if (lower.includes('spinach') || lower.includes('tomato') || lower.includes('onion')) {
    return 'produce';
  }
  if (lower.includes('rice') || lower.includes('pasta') || lower.includes('flour')) {
    return 'pantry';
  }
  return 'other';
}

function parseRecipeText(recipeText: string): Array<{ item: string; quantity?: string }> {
  return recipeText
    .split(/\n|,|;/)
    .map((line) => line.trim())
    .filter((line) => line.length > 0)
    .slice(0, 30)
    .map((line) => {
      const [quantity, ...itemParts] = line.split(' ');
      if (itemParts.length === 0) {
        return { item: line };
      }
      return { item: itemParts.join(' '), quantity };
    });
}

function normalizeItems(items: Array<{ item: string; quantity?: string }>): RecipeIngredient[] {
  return items.map((item) => ({
    item: item.item,
    quantity: item.quantity,
    section: inferSection(item.item)
  }));
}

export async function runClient(input: RecipeToListInput): Promise<RecipeToListOutput> {
  const parsedItems =
    input.action === 'parse_recipe'
      ? parseRecipeText(input.recipe_text ?? '')
      : (input.recipe_items ?? []).slice();

  const normalized_items = normalizeItems(parsedItems);

  if (input.action === 'sync_todoist') {
    return {
      provider: 'recipe-to-list',
      action: input.action,
      recipe_title: input.recipe_title ?? 'Recipe List',
      normalized_items,
      task_ids: normalized_items.map((_, index) => `todoist_task_${index + 1}`),
      project_name: input.project_id ? 'Configured Todoist Project' : 'Inbox'
    };
  }

  return {
    provider: 'recipe-to-list',
    action: input.action,
    recipe_title: input.recipe_title ?? 'Parsed Recipe',
    normalized_items
  };
}
