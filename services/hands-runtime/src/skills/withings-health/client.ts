// Plan §6 step 7 — Real Withings Health API
// POST https://wbsapi.withings.net/measure?action=getmeas
// action=getmeas is a QUERY PARAM; body is application/x-www-form-urlencoded

interface SkillContext { token?: string; user_id?: string; }

const MEAS_TYPE: Record<string, number> = {
  weight:          1,
  body_fat_pct:    6,
  muscle_mass_kg:  76,
  heart_rate_bpm:  11,
};

const UNIT_LABEL: Record<string, string> = {
  weight: 'kg', body_fat_pct: '%', muscle_mass_kg: 'kg', heart_rate_bpm: 'bpm',
};

export async function runClient(
  input: Record<string, any>,
  ctx?: SkillContext
): Promise<any> {
  const token = ctx?.token ?? process.env.WITHINGS_TOKEN;
  if (!token) throw new Error('Withings token required: provide ctx.token or set WITHINGS_TOKEN');

  const measureType = input.measure_type ?? 'weight';
  const meastype    = MEAS_TYPE[measureType];
  if (meastype === undefined) {
    throw new Error(`Unknown measure_type: "${measureType}". Valid: ${Object.keys(MEAS_TYPE).join(', ')}`);
  }

  const res = await fetch('https://wbsapi.withings.net/measure?action=getmeas', {
    method:  'POST',
    headers: {
      authorization:  `Bearer ${token}`,
      'content-type': 'application/x-www-form-urlencoded',
    },
    body: `meastype=${meastype}&category=1`,
  });
  if (!res.ok) throw new Error(`Withings HTTP error: ${res.status}`);
  const body = await res.json();

  if (body.status !== 0) {
    throw new Error(`Withings API error status ${body.status}: ${body.error ?? 'unknown'}`);
  }

  const measurements = (body.body?.measuregrps ?? []).map((grp: any) => ({
    date:         new Date(grp.date * 1000).toISOString(),
    value:        grp.measures[0].value * Math.pow(10, grp.measures[0].unit),
    measure_type: measureType,
    unit:         UNIT_LABEL[measureType] ?? '',
  }));

  return {
    measurements,
    measure_type: measureType,
    count:        measurements.length,
  };
}
