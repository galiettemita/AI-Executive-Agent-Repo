import type { ClawringhouseInput, ClawringhouseOutput, HouseholdItem } from './types.js';

function toUrgency(item: HouseholdItem): 'low' | 'medium' | 'high' {
  const ratio = item.typical_cycle_days === 0 ? 0 : item.days_since_last_order / item.typical_cycle_days;
  if (item.estimated_units_left <= 1 || ratio >= 1.1) {
    return 'high';
  }
  if (ratio >= 0.8) {
    return 'medium';
  }
  return 'low';
}

function toRecommendations(items: HouseholdItem[]): ClawringhouseOutput['recommendations'] {
  return items.map((item) => {
    const urgency = toUrgency(item);
    return {
      item: item.name,
      urgency,
      reason:
        urgency === 'high'
          ? 'Likely to run out before next planned restock window.'
          : urgency === 'medium'
            ? 'Approaching typical reorder cycle; queue recommendation.'
            : 'Sufficient inventory remains for now.'
    };
  });
}

export async function runClient(input: ClawringhouseInput): Promise<ClawringhouseOutput> {
  const recommendations = toRecommendations(input.household_items ?? []);

  if (input.action === 'schedule_reorder_reminder') {
    return {
      provider: 'clawringhouse',
      action: input.action,
      recommendations,
      next_reminder_local: input.reminder_time_local,
      summary: `Scheduled proactive reorder reminder at ${input.reminder_time_local}.`
    };
  }

  return {
    provider: 'clawringhouse',
    action: input.action,
    recommendations,
    summary: `Generated ${recommendations.length} household reorder recommendation(s).`
  };
}
