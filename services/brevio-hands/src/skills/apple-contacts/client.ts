import type { AppleContact, AppleContactsInput, AppleContactsOutput } from './types.js';

const CONTACTS: AppleContact[] = [
  {
    name: 'Sarah Lee',
    phone: '+14155550111',
    email: 'sarah.lee@example.com'
  },
  {
    name: 'Michael Torres',
    phone: '+14155550112',
    email: 'michael.torres@example.com'
  },
  {
    name: 'Board Assistant',
    phone: '+14155550113',
    email: 'assistant@example.com'
  }
];

export async function runClient(input: AppleContactsInput): Promise<AppleContactsOutput> {
  const query = input.query.toLowerCase();
  const contacts = CONTACTS.filter((contact) => {
    const haystack = `${contact.name} ${contact.phone ?? ''} ${contact.email ?? ''}`.toLowerCase();
    return haystack.includes(query);
  });

  return {
    provider: 'apple-contacts-local',
    contacts
  };
}
