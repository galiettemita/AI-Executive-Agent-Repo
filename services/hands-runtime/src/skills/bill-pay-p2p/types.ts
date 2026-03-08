export interface BillPayP2PInput {
  action: 'list_payees' | 'create_payment' | 'payment_status' | 'cancel_payment';
  payee_id?: string;
  amount_cents?: number;
  memo?: string;
  payment_id?: string;
  confirmed?: boolean;
}

export interface Payee {
  payee_id: string;
  name: string;
  last4?: string;
}

export interface BillPayP2POutput {
  provider: 'bill-pay-p2p';
  action: BillPayP2PInput['action'];
  payees?: Payee[];
  payment_id?: string;
  status?: 'pending' | 'scheduled' | 'completed' | 'cancelled';
  partnership_status: 'awaiting_api_partnership';
}
