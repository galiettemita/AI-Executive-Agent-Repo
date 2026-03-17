// Plan §6 step 10 — Real SerpAPI search

interface SkillContext { token?: string; user_id?: string; }

export async function runClient(
  input: Record<string, any>,
  ctx?: SkillContext
): Promise<any> {
  const key = process.env.SERPAPI_KEY;
  if (!key)          throw new Error('SERPAPI_KEY env var is required');
  if (!input.query)  throw new Error('query is required');

  const engine = input.engine      ?? 'google';
  const num    = input.max_results ?? 10;

  const url = `https://serpapi.com/search`
    + `?api_key=${key}`
    + `&engine=${encodeURIComponent(engine)}`
    + `&q=${encodeURIComponent(input.query)}`
    + `&num=${num}`;

  const res = await fetch(url);
  if (!res.ok) throw new Error(`SerpAPI error: ${res.status}`);
  const body = await res.json();

  const results = (body.organic_results ?? []).map((r: any) => ({
    title:    r.title,
    link:     r.link,
    source:   r.source   ?? null,
    snippet:  r.snippet  ?? null,
    position: r.position ?? null,
  }));

  return {
    query:         input.query,
    engine,
    results,
    total_results: body.search_information?.total_results ?? null,
  };
}
