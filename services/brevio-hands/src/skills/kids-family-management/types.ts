export interface KidsFamilyManagementInput {
  action: 'family_schedule' | 'pickup_plan' | 'location_checkin';
  child_name?: string;
  date?: string;
  location?: string;
}

export interface FamilyEvent {
  event_id: string;
  title: string;
  date: string;
  time: string;
  location: string;
}

export interface KidsFamilyManagementOutput {
  provider: 'kids-family-management';
  action: KidsFamilyManagementInput['action'];
  events?: FamilyEvent[];
  checkin_status?: 'on_time' | 'delayed' | 'arrived';
  partnership_status: 'awaiting_api_partnership';
}
