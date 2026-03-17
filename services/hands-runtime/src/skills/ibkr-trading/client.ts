// Plan §6 step 5 — IBKR Client Portal API
// IBKR Client Portal uses a self-signed certificate.
// Operator must set NODE_TLS_REJECT_UNAUTHORIZED=0 in the shell environment.

interface SkillContext { token?: string; user_id?: string; }

export async function runClient(
  input: Record<string, any>,
  ctx?: SkillContext
): Promise<any> {
  const portalUrl = process.env.IBKR_CLIENT_PORTAL_URL ?? 'https://localhost:5000';

  switch (input.action) {

    case 'quote_symbol': {
      if (!input.conid) throw new Error('conid is required for quote_symbol');
      const url = `${portalUrl}/v1/api/iserver/marketdata/snapshot?conids=${input.conid}&fields=31,55,7221`;
      const res  = await fetch(url);
      if (!res.ok) throw new Error(`IBKR snapshot failed: ${res.status}`);
      const data = await res.json();
      return {
        conid:      input.conid,
        last_price: data[0]?.['31'] ?? null,
        symbol:     data[0]?.['55'] ?? null,
        exchange:   data[0]?.['7221'] ?? null,
      };
    }

    case 'place_order': {
      if (input.confirmed !== true) {
        throw new Error('ORDER_CONFIRMATION_REQUIRED: set input.confirmed=true to execute live order');
      }
      if (!process.env.IBKR_CLIENT_PORTAL_URL) {
        throw new Error('IBKR_CLIENT_PORTAL_URL env var is required for live order placement');
      }
      if (!input.conid)    throw new Error('conid is required');
      if (!input.side)     throw new Error('side (BUY|SELL) is required');
      if (!input.quantity) throw new Error('quantity is required');

      const accountId = process.env.IBKR_ACCOUNT_ID;
      if (!accountId) throw new Error('IBKR_ACCOUNT_ID env var is required');

      const order = {
        conid:     input.conid,
        orderType: input.order_type ?? 'MKT',
        side:      input.side,
        quantity:  input.quantity,
        tif:       input.tif ?? 'DAY',
      };

      const res = await fetch(`${portalUrl}/v1/api/iserver/account/${accountId}/orders`, {
        method:  'POST',
        headers: { 'content-type': 'application/json' },
        body:    JSON.stringify({ orders: [order] }),
      });
      if (!res.ok) throw new Error(`IBKR order placement failed: ${res.status}`);
      const data = await res.json();

      const receiptId = data[0]?.order_id ?? data[0]?.id ?? 'unknown';
      console.log('[IBKR ORDER]', JSON.stringify({
        receipt_id: receiptId,
        ...order,
        timestamp:  new Date().toISOString(),
      }));

      return {
        receipt_id: receiptId,
        status:     data[0]?.order_status ?? 'submitted',
        conid:      input.conid,
      };
    }

    default:
      throw new Error(`Unknown IBKR action: ${input.action}. Valid: quote_symbol, place_order`);
  }
}
