import type { GroceryListInput, GroceryListItem, GroceryListOutput } from './types.js';

const BASE_LIST: GroceryListItem[] = [
  {
    item_id: 'groc_001',
    name: 'Eggs',
    quantity: 1,
    section: 'dairy',
    checked: false
  },
  {
    item_id: 'groc_002',
    name: 'Spinach',
    quantity: 1,
    section: 'produce',
    checked: false
  }
];

function normalizeSection(name: string): GroceryListItem['section'] {
  const lower = name.toLowerCase();
  if (lower.includes('milk') || lower.includes('egg') || lower.includes('yogurt')) {
    return 'dairy';
  }
  if (lower.includes('apple') || lower.includes('spinach') || lower.includes('avocado')) {
    return 'produce';
  }
  return 'other';
}

export async function runClient(input: GroceryListInput): Promise<GroceryListOutput> {
  const list_id = input.list_id ?? 'grocery_default';

  if (input.action === 'add_items') {
    const newItems: GroceryListItem[] = (input.items ?? []).map((item, index) => ({
      item_id: `groc_new_${index + 1}`,
      name: item.name,
      quantity: item.quantity ?? 1,
      section: item.section ?? normalizeSection(item.name),
      checked: false
    }));

    const items = [...BASE_LIST, ...newItems];
    return {
      provider: 'grocery-list',
      action: input.action,
      list_id,
      items,
      total_items: items.length
    };
  }

  if (input.action === 'remove_items') {
    const filtered = BASE_LIST.filter((item) => !(input.item_ids ?? []).includes(item.item_id));
    return {
      provider: 'grocery-list',
      action: input.action,
      list_id,
      items: filtered,
      total_items: filtered.length
    };
  }

  if (input.action === 'organize_by_section') {
    const sorted = [...BASE_LIST].sort((left, right) => left.section.localeCompare(right.section));
    return {
      provider: 'grocery-list',
      action: input.action,
      list_id,
      items: sorted,
      total_items: sorted.length
    };
  }

  if (input.action === 'clear_list') {
    return {
      provider: 'grocery-list',
      action: input.action,
      list_id,
      items: [],
      total_items: 0
    };
  }

  return {
    provider: 'grocery-list',
    action: input.action,
    list_id,
    items: BASE_LIST,
    total_items: BASE_LIST.length
  };
}
