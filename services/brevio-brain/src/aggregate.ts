import type { AggregationRequest, AggregationResponse, SkillResult } from './types.js';

function trimToMax(text: string, maxChars: number): string {
  if (text.length <= maxChars) {
    return text;
  }
  return `${text.slice(0, Math.max(0, maxChars - 1)).trimEnd()}…`;
}

function formatByChannel(channel: AggregationRequest['channel'], text: string): string {
  const normalized = text.replace(/\r\n/g, '\n').trim();
  if (channel === 'WHATSAPP') {
    return trimToMax(normalized.replace(/\n{3,}/g, '\n\n'), 4096);
  }
  return trimToMax(normalized, 4096);
}

function summarizeData(data: SkillResult['data']): string {
  if (!data) {
    return 'completed';
  }
  if (typeof data.summary === 'string' && data.summary.trim() !== '') {
    return data.summary.trim();
  }
  if (typeof data.note === 'string' && data.note.trim() !== '') {
    return data.note.trim();
  }
  if (typeof data.provider === 'string' && typeof data.action === 'string') {
    return `${data.provider} ${data.action}`;
  }
  for (const [key, value] of Object.entries(data)) {
    if (Array.isArray(value)) {
      return `${key}: ${value.length} item${value.length === 1 ? '' : 's'}`;
    }
  }
  return 'completed';
}

function summarizeResultLine(result: SkillResult): string {
  if (result.status === 'SUCCESS') {
    return `- ${result.skill_id}: ${summarizeData(result.data)}.`;
  }
  if (result.status === 'PARTIAL') {
    return `- ${result.skill_id}: partial success (${summarizeData(result.data)}).`;
  }
  if (result.status === 'TIMEOUT') {
    return `- ${result.skill_id}: timed out.`;
  }
  const errorCode = result.error?.code ?? 'UNKNOWN_ERROR';
  return `- ${result.skill_id}: failed (${errorCode}). Review the connector logs or approval requirements before retrying.`;
}

function stylePrefix(style: string | undefined): string {
  switch (style) {
    case 'concise':
      return 'Quick update:';
    case 'detailed':
      return 'Detailed update:';
    default:
      return 'Update:';
  }
}

export function aggregateResults(request: AggregationRequest): AggregationResponse {
  const lines = request.skill_results.map((result) => summarizeResultLine(result));
  const successCount = request.skill_results.filter((result) => result.status === 'SUCCESS').length;
  const partialCount = request.skill_results.filter((result) => result.status === 'PARTIAL').length;
  const failureCount = request.skill_results.filter((result) => result.status === 'FAILED' || result.status === 'TIMEOUT').length;
  const total = Math.max(request.skill_results.length, 1);
  const completionRatio = Number(((successCount + partialCount * 0.5) / total).toFixed(2));

  const warnings: string[] = [];
  if (failureCount > 0) {
    warnings.push(`${failureCount} skill execution${failureCount === 1 ? '' : 's'} failed or timed out.`);
  }
  if (partialCount > 0) {
    warnings.push(`${partialCount} skill execution${partialCount === 1 ? '' : 's'} completed only partially.`);
  }

  const responseText = [
    stylePrefix(request.user_profile?.communication_style),
    ...lines,
    `Completion ratio: ${completionRatio}.`
  ].join('\n');

  const suggestedActions: string[] = [];
  if (failureCount > 0) {
    suggestedActions.push('Retry failed actions');
  }
  if (partialCount > 0) {
    suggestedActions.push('Review partial results');
  }
  if (successCount > 0) {
    suggestedActions.push('Confirm next step');
  }

  return {
    response_text: formatByChannel(request.channel, responseText),
    suggested_actions: suggestedActions,
    follow_up_scheduled: false,
    completion_ratio: completionRatio,
    warnings
  };
}
