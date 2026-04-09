export interface ExecutionPolicy {
  consent_requirement?: 'none' | 'recommended' | 'required';
  consent_record?: string;
  human_review?: 'none' | 'recommended' | 'required';
  human_review_record?: string;
  recipient_verification?: 'not_applicable' | 'required' | 'verified';
}

export interface ApprovalGateFailure {
  code: 'CONSENT_REQUIRED' | 'HUMAN_REVIEW_REQUIRED' | 'RECIPIENT_VERIFICATION_REQUIRED';
  message: string;
  httpStatus: number;
}

function hasRecord(value: string | undefined): boolean {
  return typeof value === 'string' && value.trim().length > 0;
}

export function evaluateApprovalGate(policy: ExecutionPolicy | undefined): ApprovalGateFailure | null {
  if (!policy) {
    return null;
  }

  if (policy.consent_requirement === 'required' && !hasRecord(policy.consent_record)) {
    return {
      code: 'CONSENT_REQUIRED',
      message: 'execution requires a recorded consent approval before this skill can run',
      httpStatus: 412
    };
  }

  if (policy.human_review === 'required' && !hasRecord(policy.human_review_record)) {
    return {
      code: 'HUMAN_REVIEW_REQUIRED',
      message: 'execution requires recorded human review before this skill can run',
      httpStatus: 412
    };
  }

  if (policy.recipient_verification === 'required') {
    return {
      code: 'RECIPIENT_VERIFICATION_REQUIRED',
      message: 'execution requires verified recipient state before this skill can run',
      httpStatus: 412
    };
  }

  return null;
}
