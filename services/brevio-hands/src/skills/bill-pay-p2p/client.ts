import type { BillPayP2PInput, BillPayP2POutput, Payee } from './types.js';

const PAYEES: Payee[] = [
  { payee_id: 'payee_001', name: 'Austin Utilities', last4: '4481' },
  { payee_id: 'payee_002', name: 'Jordan Smith', last4: '1150' }
];

export async function runClient(input: BillPayP2PInput): Promise<BillPayP2POutput> {
  if (input.action === 'list_payees') {
    return {
      provider: 'bill-pay-p2p',
      action: 'list_payees',
      payees: PAYEES,
      partnership_status: 'awaiting_api_partnership'
    };
  }

  if (input.action === 'create_payment') {
    return {
      provider: 'bill-pay-p2p',
      action: 'create_payment',
      payment_id: 'payment_001',
      status: 'scheduled',
      partnership_status: 'awaiting_api_partnership'
    };
  }

  if (input.action === 'cancel_payment') {
    return {
      provider: 'bill-pay-p2p',
      action: 'cancel_payment',
      payment_id: input.payment_id,
      status: 'cancelled',
      partnership_status: 'awaiting_api_partnership'
    };
  }

  return {
    provider: 'bill-pay-p2p',
    action: 'payment_status',
    payment_id: input.payment_id,
    status: 'scheduled',
    partnership_status: 'awaiting_api_partnership'
  };
}
