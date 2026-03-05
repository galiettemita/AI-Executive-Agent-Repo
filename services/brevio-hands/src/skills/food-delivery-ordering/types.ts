export interface FoodDeliveryOrderingInput {
  action: 'search_restaurants' | 'build_cart' | 'checkout' | 'order_status';
  address?: string;
  cuisine?: string;
  restaurant_id?: string;
  items?: Array<{ item_id: string; quantity: number }>;
  cart_id?: string;
  order_id?: string;
  confirmed?: boolean;
}

export interface FoodDeliveryRestaurant {
  restaurant_id: string;
  name: string;
  cuisine: string;
  eta_minutes: number;
}

export interface FoodDeliveryOrderingOutput {
  provider: 'food-delivery-ordering';
  action: FoodDeliveryOrderingInput['action'];
  restaurants?: FoodDeliveryRestaurant[];
  cart_id?: string;
  order_id?: string;
  status?: 'pending' | 'confirmed' | 'delivered' | 'cancelled';
  estimated_total_cents?: number;
  partnership_status: 'awaiting_api_partnership';
}
