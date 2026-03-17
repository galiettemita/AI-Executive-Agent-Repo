// Plan §6 step 1 — Real Home Assistant REST API
// Resolves: P0-HOME-ASSISTANT-NO-HTTP

interface SkillContext { token?: string; user_id?: string; }

const RESTRICTED_ACTIONS = new Set(['unlock', 'disable_alarm', 'alarm_disarm']);

export async function runClient(
  input: Record<string, any>,
  ctx?: SkillContext
): Promise<any> {
  const haUrl   = process.env.HOME_ASSISTANT_URL;
  const haToken = process.env.HOME_ASSISTANT_TOKEN;
  if (!haUrl || !haToken) {
    throw new Error('HOME_ASSISTANT_URL and HOME_ASSISTANT_TOKEN env vars are required');
  }

  if (!input.entity_id) throw new Error('entity_id is required');
  if (!input.action)    throw new Error('action is required');

  if (RESTRICTED_ACTIONS.has(input.action) && !input.two_factor_code) {
    throw new Error('SAFETY_2FA_REQUIRED');
  }

  const domain = input.entity_id.split('.')[0];

  const serviceRes = await fetch(
    `${haUrl}/api/services/${domain}/${input.action}`,
    {
      method: 'POST',
      headers: {
        authorization:   `Bearer ${haToken}`,
        'content-type':  'application/json',
      },
      body: JSON.stringify({ entity_id: input.entity_id }),
    }
  );
  if (!serviceRes.ok) {
    throw new Error(`HA service call failed: ${serviceRes.status}`);
  }

  const stateRes = await fetch(
    `${haUrl}/api/states/${input.entity_id}`,
    {
      headers: { authorization: `Bearer ${haToken}` },
    }
  );
  if (!stateRes.ok) {
    throw new Error(`HA state fetch failed: ${stateRes.status}`);
  }
  const data = await stateRes.json();

  return {
    state: data.state,
    attributes: {
      ...data.attributes,
      entity_id:   input.entity_id,
      action:      input.action,
      acknowledged: true,
    },
  };
}
