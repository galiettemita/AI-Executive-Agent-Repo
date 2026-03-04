export interface ShoppingExpertInput {
  query: string;
  max_price?: number;
  category?: string;
  limit?: number;
}

export interface ShoppingResultItem {
  title: string;
  price: number;
  url: string;
  rating: number;
  store: string;
}

export interface ShoppingExpertOutput {
  provider: 'mock_catalog';
  results: ShoppingResultItem[];
}
