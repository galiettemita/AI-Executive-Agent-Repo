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

export interface ApprovalPolicyDescriptor {
  write_operations: string[];
  recipient_sensitive_operations?: string[];
  requires_confirmation?: boolean;
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

function strictestRequirement(
  left: ExecutionPolicy['consent_requirement'] | ExecutionPolicy['human_review'],
  right: ExecutionPolicy['consent_requirement'] | ExecutionPolicy['human_review']
): ExecutionPolicy['consent_requirement'] {
  if (left === 'required' || right === 'required') {
    return 'required';
  }
  if (left === 'recommended' || right === 'recommended') {
    return 'recommended';
  }
  return 'none';
}

export function deriveRequiredPolicy(
  descriptor: ApprovalPolicyDescriptor | undefined,
  operation: string
): ExecutionPolicy | undefined {
  if (!descriptor) {
    return undefined;
  }
  const required: ExecutionPolicy = {};
  if (descriptor.requires_confirmation || descriptor.write_operations.includes(operation)) {
    required.consent_requirement = 'required';
  }
  if (descriptor.requires_confirmation) {
    required.human_review = 'required';
  }
  if (descriptor.recipient_sensitive_operations?.includes(operation)) {
    required.recipient_verification = 'required';
  }
  return Object.keys(required).length > 0 ? required : undefined;
}

export function mergeExecutionPolicy(
  required: ExecutionPolicy | undefined,
  provided: ExecutionPolicy | undefined
): ExecutionPolicy | undefined {
  if (!required && !provided) {
    return undefined;
  }
  return {
    consent_requirement: strictestRequirement(required?.consent_requirement, provided?.consent_requirement),
    consent_record: provided?.consent_record ?? required?.consent_record,
    human_review: strictestRequirement(required?.human_review, provided?.human_review),
    human_review_record: provided?.human_review_record ?? required?.human_review_record,
    recipient_verification:
      provided?.recipient_verification === 'verified'
        ? 'verified'
        : required?.recipient_verification === 'required' || provided?.recipient_verification === 'required'
          ? 'required'
          : 'not_applicable'
  };
}
