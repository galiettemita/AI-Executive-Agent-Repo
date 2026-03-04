import type {
  FoodDeliveryOrderingInput,
  FoodDeliveryOrderingOutput,
  FoodDeliveryRestaurant
} from './types.js';

const RESTAURANTS: FoodDeliveryRestaurant[] = [
  {
    restaurant_id: 'fd_rest_001',
    name: 'Green Bowl Kitchen',
    cuisine: 'Healthy',
    eta_minutes: 28
  },
  {
    restaurant_id: 'fd_rest_002',
    name: 'Spice Route Express',
    cuisine: 'Indian',
    eta_minutes: 34
  }
];

export async function runClient(
  input: FoodDeliveryOrderingInput
): Promise<FoodDeliveryOrderingOutput> {
  if (input.action === 'search_restaurants') {
    return {
      provider: 'food-delivery-ordering',
      action: 'search_restaurants',
      restaurants: RESTAURANTS,
      partnership_status: 'awaiting_api_partnership'
    };
  }

  if (input.action === 'build_cart') {
    const estimatedTotalCents = (input.items ?? []).reduce(
      (sum, item) => sum + item.quantity * 1450,
      0
    );
    return {
      provider: 'food-delivery-ordering',
      action: 'build_cart',
      cart_id: 'cart_fd_001',
      estimated_total_cents: estimatedTotalCents,
      status: 'pending',
      partnership_status: 'awaiting_api_partnership'
    };
  }

  if (input.action === 'checkout') {
    return {
      provider: 'food-delivery-ordering',
      action: 'checkout',
      order_id: 'order_fd_001',
      status: 'confirmed',
      estimated_total_cents: 4350,
      partnership_status: 'awaiting_api_partnership'
    };
  }

  return {
    provider: 'food-delivery-ordering',
    action: 'order_status',
    order_id: input.order_id,
    status: 'confirmed',
    partnership_status: 'awaiting_api_partnership'
  };
}
