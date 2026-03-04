import type {
  FamilyEvent,
  KidsFamilyManagementInput,
  KidsFamilyManagementOutput
} from './types.js';

const EVENTS: FamilyEvent[] = [
  {
    event_id: 'family_001',
    title: 'Soccer Practice',
    date: '2026-03-06',
    time: '17:30',
    location: 'West Field Complex'
  },
  {
    event_id: 'family_002',
    title: 'Piano Lesson',
    date: '2026-03-07',
    time: '10:00',
    location: 'North Studio'
  }
];

export async function runClient(
  input: KidsFamilyManagementInput
): Promise<KidsFamilyManagementOutput> {
  if (input.action === 'family_schedule') {
    return {
      provider: 'kids-family-management',
      action: 'family_schedule',
      events: EVENTS,
      partnership_status: 'awaiting_api_partnership'
    };
  }

  if (input.action === 'pickup_plan') {
    return {
      provider: 'kids-family-management',
      action: 'pickup_plan',
      events: EVENTS,
      checkin_status: 'on_time',
      partnership_status: 'awaiting_api_partnership'
    };
  }

  return {
    provider: 'kids-family-management',
    action: 'location_checkin',
    checkin_status: 'arrived',
    partnership_status: 'awaiting_api_partnership'
  };
}
