export type GroceryListAction =
  | 'add_items'
  | 'remove_items'
  | 'list_items'
  | 'organize_by_section'
  | 'clear_list';

export interface GroceryListItem {
  item_id: string;
  name: string;
  quantity: number;
  section: 'produce' | 'dairy' | 'protein' | 'pantry' | 'frozen' | 'household' | 'other';
  checked: boolean;
}

export interface GroceryListInput {
  action: GroceryListAction;
  list_id?: string;
  items?: Array<{
    name: string;
    quantity?: number;
    section?: GroceryListItem['section'];
  }>;
  item_ids?: string[];
  confirmed?: boolean;
}

export interface GroceryListOutput {
  provider: 'grocery-list';
  action: GroceryListAction;
  list_id: string;
  items: GroceryListItem[];
  total_items: number;
}
