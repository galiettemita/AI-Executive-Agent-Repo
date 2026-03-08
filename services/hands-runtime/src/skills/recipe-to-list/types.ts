export type RecipeToListAction = 'parse_recipe' | 'sync_todoist';

export interface RecipeIngredient {
  item: string;
  quantity?: string;
  section: 'produce' | 'dairy' | 'protein' | 'pantry' | 'frozen' | 'household' | 'other';
}

export interface RecipeToListInput {
  action: RecipeToListAction;
  recipe_title?: string;
  recipe_text?: string;
  recipe_items?: Array<{ item: string; quantity?: string }>;
  project_id?: string;
}

export interface RecipeToListOutput {
  provider: 'recipe-to-list';
  action: RecipeToListAction;
  recipe_title: string;
  normalized_items: RecipeIngredient[];
  task_ids?: string[];
  project_name?: string;
}
