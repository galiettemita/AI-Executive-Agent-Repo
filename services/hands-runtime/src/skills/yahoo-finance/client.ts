// Plan §6 step 4 — Real Yahoo Finance spark API (no API key required)

interface SkillContext { token?: string; user_id?: string; }

const YAHOO_BASE = 'https://query1.finance.yahoo.com/v8/finance/spark';

export async function runClient(
  input: Record<string, any>,
  ctx?: SkillContext
): Promise<any> {
  const symbolList: string[] = Array.isArray(input.symbols)
    ? input.symbols
    : [input.symbol ?? 'AAPL'];
  const symbols = symbolList.join(',');

  const url = `${YAHOO_BASE}?symbols=${encodeURIComponent(symbols)}&range=1d&interval=1d`;

  const res = await fetch(url, {
    headers: { 'user-agent': 'Mozilla/5.0 (compatible; BrevioAssistant/1.0)' },
  });
  if (!res.ok) throw new Error(`Yahoo Finance request failed: ${res.status}`);

  const body = await res.json();
  const results = body?.spark?.result;
  if (!results || results.length === 0) {
    throw new Error(`Yahoo Finance returned no data for symbols: ${symbols}`);
  }

  const quotes = results.map((item: any) => {
    const meta = item?.response?.[0]?.meta;
    if (!meta) throw new Error(`Malformed Yahoo Finance response for symbol`);
    const price          = meta.regularMarketPrice as number;
    const previousClose  = meta.previousClose as number;
    const change_pct     = previousClose
      ? Number(((price - previousClose) / previousClose * 100).toFixed(4))
      : null;
    return {
      symbol:         meta.symbol,
      price,
      previous_close: previousClose,
      change_pct,
      volume:         meta.regularMarketVolume,
      currency:       meta.currency ?? 'USD',
    };
  });

  return { quotes };
}
