// Plan §6 step 6 — Real Dexcom CGM API

interface SkillContext { token?: string; user_id?: string; }

export async function runClient(
  input: Record<string, any>,
  ctx?: SkillContext
): Promise<any> {
  const token = ctx?.token ?? process.env.DEXCOM_TOKEN;
  if (!token) throw new Error('Dexcom token required: provide ctx.token or set DEXCOM_TOKEN');

  const endDate   = new Date();
  const startDate = new Date(endDate.getTime() - 3 * 60 * 60 * 1000);

  const url = `https://api.dexcom.com/v3/users/self/egvs`
    + `?startDate=${startDate.toISOString()}`
    + `&endDate=${endDate.toISOString()}`;

  const res = await fetch(url, {
    headers: { authorization: `Bearer ${token}` },
  });
  if (!res.ok) throw new Error(`Dexcom API error: ${res.status}`);
  const body = await res.json();

  const readings = (body.records ?? []).map((r: any) => ({
    time:              r.displayTime,
    value_mg_dl:       r.value,
    trend:             r.trend,
    trend_description: r.trendDescription ?? null,
    status:            r.value < 70 ? 'low' : r.value > 180 ? 'high' : 'in_range',
  }));

  const alerts = readings
    .filter((r: any) => r.status !== 'in_range')
    .map((r: any) => ({ time: r.time, value_mg_dl: r.value_mg_dl, status: r.status, trend: r.trend }));

  return {
    readings,
    latest:  readings.length > 0 ? readings[readings.length - 1] : null,
    alerts,
    summary: {
      total:          readings.length,
      low_count:      readings.filter((r: any) => r.status === 'low').length,
      high_count:     readings.filter((r: any) => r.status === 'high').length,
      in_range_count: readings.filter((r: any) => r.status === 'in_range').length,
    },
  };
}
