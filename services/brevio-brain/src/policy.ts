import type {
  ActionPolicyMetadata,
  ConsentRequirement,
  ExternalModelEgress,
  HumanReviewLevel,
  NormalizedReasoningRequest,
  PlannedAction,
  PlanPolicySummary,
  PolicyDataClass,
  PolicySensitivity,
  PrivacyMode
} from './types.js';

const emailPattern = /[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}/i;
const phonePattern = /\+?\d[\d\-\s()]{7,}\d/;
const ssnPattern = /\b\d{3}-\d{2}-\d{4}\b/;
const cardPattern = /\b(?:\d[ -]*?){13,19}\b/;
const credentialPattern = /\b(api[_ -]?key|secret|password|token|private key|refresh token)\b/i;
const financialPattern = /\b(bank|routing number|account number|wire|credit card|transaction|budget|invoice|payee|plaid|stripe)\b/i;
const healthPattern = /\b(health|medical|diagnosis|prescription|blood pressure|heart rate|dexcom|withings)\b/i;
const communicationPattern = /\b(email|inbox|mail|reply|recipient|subject|message)\b/i;

export interface PolicySignals {
  dataClass: PolicyDataClass;
  sensitivity: PolicySensitivity;
  containsPII: boolean;
  highRiskDomain: boolean;
}

function includesPII(text: string): boolean {
  return (
    emailPattern.test(text) ||
    phonePattern.test(text) ||
    ssnPattern.test(text) ||
    cardPattern.test(text)
  );
}

export function resolvePrivacyMode(request: Pick<NormalizedReasoningRequest, 'user_preferences'>): PrivacyMode {
  return request.user_preferences.privacy_mode ?? 'strict';
}

export function redactSensitiveText(text: string): string {
  return text
    .replace(emailPattern, '[REDACTED_EMAIL]')
    .replace(phonePattern, '[REDACTED_PHONE]')
    .replace(ssnPattern, '[REDACTED_SSN]')
    .replace(cardPattern, '[REDACTED_ACCOUNT]')
    .replace(credentialPattern, '[REDACTED_SECRET]');
}

export function detectPolicySignals(text: string, intent?: string): PolicySignals {
  const normalized = text.trim();
  const containsPII = includesPII(normalized);
  const financial = intent?.startsWith('finance.') || financialPattern.test(normalized);
  const health = intent?.startsWith('health.') || healthPattern.test(normalized);
  const credentials = credentialPattern.test(normalized);
  const communications = intent?.startsWith('email.') || communicationPattern.test(normalized);

  if (credentials) {
    return {
      dataClass: 'credentials',
      sensitivity: 'critical',
      containsPII: true,
      highRiskDomain: true
    };
  }
  if (health) {
    return {
      dataClass: 'health',
      sensitivity: 'critical',
      containsPII: true,
      highRiskDomain: true
    };
  }
  if (financial) {
    return {
      dataClass: 'financial',
      sensitivity: containsPII ? 'critical' : 'high',
      containsPII: true,
      highRiskDomain: true
    };
  }
  if (communications) {
    return {
      dataClass: 'communications',
      sensitivity: containsPII ? 'high' : 'moderate',
      containsPII,
      highRiskDomain: containsPII
    };
  }
  if (containsPII) {
    return {
      dataClass: 'personal',
      sensitivity: 'high',
      containsPII: true,
      highRiskDomain: false
    };
  }
  return {
    dataClass: 'general',
    sensitivity: 'low',
    containsPII: false,
    highRiskDomain: false
  };
}

function retentionClassFor(dataClass: PolicyDataClass): ActionPolicyMetadata['retention_class'] {
  switch (dataClass) {
    case 'financial':
    case 'health':
    case 'credentials':
      return 'regulated';
    case 'communications':
    case 'personal':
      return 'standard';
    default:
      return 'ephemeral';
  }
}

function consentRequirementFor(intent: string, operation: string, dataClass: PolicyDataClass): ConsentRequirement {
  if (dataClass === 'financial' || dataClass === 'health' || dataClass === 'credentials') {
    return 'required';
  }
  if (intent === 'email.send' || operation === 'send' || operation === 'reply') {
    return 'required';
  }
  if (intent === 'email.search' || intent === 'calendar.schedule') {
    return 'required';
  }
  return 'none';
}

function humanReviewFor(intent: string, operation: string, sensitivity: PolicySensitivity): HumanReviewLevel {
  if (sensitivity === 'critical') {
    return 'required';
  }
  if (intent === 'email.send' || operation === 'send' || operation === 'reply') {
    return 'required';
  }
  if (intent === 'calendar.schedule' || sensitivity === 'high') {
    return 'recommended';
  }
  return 'none';
}

function externalModelEgressFor(dataClass: PolicyDataClass, privacyMode: PrivacyMode): ExternalModelEgress {
  if (privacyMode === 'strict') {
    return dataClass === 'general' ? 'redacted_only' : 'deny';
  }
  if (dataClass === 'financial' || dataClass === 'health' || dataClass === 'credentials') {
    return 'deny';
  }
  if (dataClass === 'communications' || dataClass === 'personal') {
    return 'redacted_only';
  }
  return 'allow';
}

function allowedProcessorsFor(dataClass: PolicyDataClass): string[] {
  const base = ['brain', 'policy'];
  if (dataClass === 'financial' || dataClass === 'health' || dataClass === 'credentials') {
    return [...base, 'local_executor'];
  }
  return [...base, 'approved_connector'];
}

function consentScopeFor(intent: string, skillId: string | undefined): string | undefined {
  if (intent.startsWith('email.')) {
    return `connector:${skillId ?? 'email'}`;
  }
  if (intent.startsWith('finance.')) {
    return 'domain:financial';
  }
  if (intent.startsWith('health.')) {
    return 'domain:health';
  }
  if (intent === 'calendar.schedule') {
    return `connector:${skillId ?? 'calendar'}`;
  }
  return undefined;
}

function recipientVerificationFor(intent: string, operation: string): ActionPolicyMetadata['recipient_verification'] {
  if (intent === 'email.send' || operation === 'send' || operation === 'reply') {
    return 'required';
  }
  return 'not_applicable';
}

export function buildActionPolicyMetadata(
  request: NormalizedReasoningRequest,
  goal: string,
  intent: string,
  operation: string,
  skillId: string | undefined
): ActionPolicyMetadata {
  const privacyMode = resolvePrivacyMode(request);
  const signals = detectPolicySignals(goal, intent);
  const consentRequirement = consentRequirementFor(intent, operation, signals.dataClass);
  return {
    data_class: signals.dataClass,
    sensitivity: signals.sensitivity,
    privacy_mode: privacyMode,
    legal_basis: 'user_request',
    consent_requirement: consentRequirement,
    consent_scope: consentScopeFor(intent, skillId),
    consent_record: undefined,
    recipient_verification: recipientVerificationFor(intent, operation),
    provenance: 'user_message',
    human_review: humanReviewFor(intent, operation, signals.sensitivity),
    external_model_egress: externalModelEgressFor(signals.dataClass, privacyMode),
    contains_pii: signals.containsPII,
    retention_class: retentionClassFor(signals.dataClass),
    allowed_processors: allowedProcessorsFor(signals.dataClass)
  };
}

export function buildPlanPolicySummary(
  request: NormalizedReasoningRequest,
  actions: PlannedAction[]
): PlanPolicySummary {
  const privacyMode = resolvePrivacyMode(request);
  const executeActions = actions.filter((action) => action.action_type === 'execute_skill');
  const dataClasses = [...new Set(executeActions.map((action) => action.policy.data_class))].sort();
  const requiresConsent = executeActions.some((action) => action.policy.consent_requirement !== 'none');
  const requiresRecipientVerification = executeActions.some((action) => action.policy.recipient_verification === 'required');
  const humanReviewRequired = executeActions.some((action) => action.policy.human_review === 'required');
  const containsPII = executeActions.some((action) => action.policy.contains_pii);
  const strictestSensitivity = executeActions.reduce<PolicySensitivity>((current, action) => {
    const order: Record<PolicySensitivity, number> = { low: 0, moderate: 1, high: 2, critical: 3 };
    return order[action.policy.sensitivity] > order[current] ? action.policy.sensitivity : current;
  }, 'low');

  const externalModelEgress = executeActions.reduce<ExternalModelEgress>((current, action) => {
    if (current === 'deny' || action.policy.external_model_egress === 'deny') {
      return 'deny';
    }
    if (current === 'redacted_only' || action.policy.external_model_egress === 'redacted_only') {
      return 'redacted_only';
    }
    return 'allow';
  }, privacyMode === 'open' ? 'allow' : 'redacted_only');

  return {
    privacy_mode: privacyMode,
    data_classes: dataClasses,
    contains_pii: containsPII,
    highest_sensitivity: strictestSensitivity,
    external_model_egress: externalModelEgress,
    requires_consent: requiresConsent,
    requires_recipient_verification: requiresRecipientVerification,
    human_review_required: humanReviewRequired
  };
}

export function evaluateExternalPlannerPolicy(
  request: NormalizedReasoningRequest,
  actions: PlannedAction[]
): { allowed: boolean; reason: string } {
  const summary = buildPlanPolicySummary(request, actions);
  if (!request.user_preferences.allow_external_reasoning) {
    return {
      allowed: false,
      reason: 'External planner retained locally because external reasoning is not explicitly enabled.'
    };
  }
  if (summary.external_model_egress === 'deny') {
    return {
      allowed: false,
      reason: 'External planner skipped because request policy forbids external model egress.'
    };
  }
  if (summary.human_review_required) {
    return {
      allowed: false,
      reason: 'External planner skipped because the request requires human review.'
    };
  }
  return {
    allowed: true,
    reason: 'External planner allowed under current privacy policy.'
  };
}

export function buildExternalPlannerInput(
  request: NormalizedReasoningRequest,
  actions: PlannedAction[],
  confidence: number
): Record<string, unknown> {
  const policySummary = buildPlanPolicySummary(request, actions);
  return {
    request_summary: {
      message_text: redactSensitiveText(request.message_text),
      channel: request.channel ?? 'API',
      deployment_mode: request.deployment_mode ?? 'cloud',
      user_tier: request.user_tier ?? 'free',
      communication_style: request.user_profile.communication_style ?? 'balanced'
    },
    action_summaries: actions.map((action) => ({
      step_id: action.step_id,
      action_type: action.action_type,
      intent: action.intent,
      skill_id: action.skill_id,
      operation: action.operation,
      dependencies: action.dependencies,
      policy: action.policy
    })),
    policy_summary: policySummary,
    draft_confidence: confidence
  };
}
