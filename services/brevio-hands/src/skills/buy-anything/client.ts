import type { BuyAnythingInput, BuyAnythingOutput } from './types.js';

const SEARCH_OPTIONS: BuyAnythingOutput['product_options'] = [
  {
    sku: 'AMZ-RUN-001',
    title: 'Carbon Running Shoe',
    price_cents: 12999,
    rating: 4.7,
    prime_eligible: true,
    url: 'https://www.amazon.com/dp/AMZ-RUN-001'
  },
  {
    sku: 'AMZ-RUN-002',
    title: 'Stability Trainer Pro',
    price_cents: 10499,
    rating: 4.5,
    prime_eligible: true,
    url: 'https://www.amazon.com/dp/AMZ-RUN-002'
  }
];

function buildCheckoutPreview(input: BuyAnythingInput): BuyAnythingOutput['checkout_preview'] {
  const subtotal = (input.line_items ?? []).reduce(
    (sum, item) => sum + item.unit_price_cents * item.quantity,
    0
  );
  const derivedSubtotal = subtotal > 0 ? subtotal : 10999 * (input.quantity ?? 1);
  const shipping = derivedSubtotal >= 3500 ? 0 : 699;
  const tax = Math.round(derivedSubtotal * 0.0825);
  return {
    subtotal_cents: derivedSubtotal,
    shipping_cents: shipping,
    tax_cents: tax,
    total_cents: derivedSubtotal + shipping + tax
  };
}

export async function runClient(input: BuyAnythingInput): Promise<BuyAnythingOutput> {
  if (input.action === 'search_product') {
    return {
      provider: 'buy-anything',
      action: input.action,
      product_options: SEARCH_OPTIONS
    };
  }

  if (input.action === 'prepare_checkout') {
    return {
      provider: 'buy-anything',
      action: input.action,
      checkout_preview: buildCheckoutPreview(input)
    };
  }

  if (input.action === 'place_order') {
    return {
      provider: 'buy-anything',
      action: input.action,
      order_id: 'ord_buy_anything_001',
      order_status: 'confirmed',
      checkout_preview: buildCheckoutPreview(input)
    };
  }

  return {
    provider: 'buy-anything',
    action: input.action,
    order_id: input.order_id,
    order_status: 'shipped'
  };
}
