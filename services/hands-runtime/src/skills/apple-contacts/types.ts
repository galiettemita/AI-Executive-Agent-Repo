export interface AppleContactsInput {
  query: string;
}

export interface AppleContact {
  name: string;
  phone?: string;
  email?: string;
}

export interface AppleContactsOutput {
  provider: 'apple-contacts-local';
  contacts: AppleContact[];
}
