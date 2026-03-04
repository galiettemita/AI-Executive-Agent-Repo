export type BuyAnythingAction = 'search_product' | 'prepare_checkout' | 'place_order' | 'order_status';

export interface BuyAnythingLineItem {
  sku: string;
  title: string;
  quantity: number;
  unit_price_cents: number;
}

export interface BuyAnythingInput {
  action: BuyAnythingAction;
  query?: string;
  amazon_url?: string;
  quantity?: number;
  max_total_cents?: number;
  shipping_address_id?: string;
  line_items?: BuyAnythingLineItem[];
  order_id?: string;
  confirmed?: boolean;
}

export interface BuyAnythingOutput {
  provider: 'buy-anything';
  action: BuyAnythingAction;
  product_options?: Array<{
    sku: string;
    title: string;
    price_cents: number;
    rating: number;
    prime_eligible: boolean;
    url: string;
  }>;
  checkout_preview?: {
    subtotal_cents: number;
    shipping_cents: number;
    tax_cents: number;
    total_cents: number;
  };
  order_id?: string;
  order_status?: 'pending' | 'confirmed' | 'shipped' | 'delivered' | 'cancelled';
}
