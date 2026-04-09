import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { evaluateApprovalGate } from './approval-policy.js';

describe('approval policy', () => {
  it('blocks execution when consent is required but missing', () => {
    const result = evaluateApprovalGate({
      consent_requirement: 'required'
    });

    assert.equal(result?.code, 'CONSENT_REQUIRED');
    assert.equal(result?.httpStatus, 412);
  });

  it('blocks execution when human review is required but missing', () => {
    const result = evaluateApprovalGate({
      consent_requirement: 'none',
      human_review: 'required'
    });

    assert.equal(result?.code, 'HUMAN_REVIEW_REQUIRED');
  });

  it('allows execution when approval evidence is present', () => {
    const result = evaluateApprovalGate({
      consent_requirement: 'required',
      consent_record: 'consent-1',
      human_review: 'required',
      human_review_record: 'review-1',
      recipient_verification: 'verified'
    });

    assert.equal(result, null);
  });
});
