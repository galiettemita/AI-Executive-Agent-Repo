import type { AggregationRequest, AggregationResponse } from './types.js';

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

function summarizeResultLine(result: AggregationRequest['skill_results'][number]): string {
  if (result.status === 'SUCCESS') {
    return `- ${result.skill_id}: completed successfully.`;
  }
  if (result.status === 'PARTIAL') {
    return `- ${result.skill_id}: partially completed.`;
  }
  if (result.status === 'TIMEOUT') {
    return `- ${result.skill_id}: timed out.`;
  }
  const errorCode = result.error?.code ?? 'UNKNOWN_ERROR';
  return `- ${result.skill_id}: failed (${errorCode}).`;
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
  const failureCount = request.skill_results.filter((result) => result.status === 'FAILED' || result.status === 'TIMEOUT').length;

  const responseText = [
    stylePrefix(request.user_profile?.communication_style),
    ...lines,
    `Completed ${successCount}/${request.skill_results.length} skill executions.`
  ].join('\n');

  const suggestedActions: string[] = [];
  if (failureCount > 0) {
    suggestedActions.push('Retry failed actions');
  }
  if (successCount > 0) {
    suggestedActions.push('Confirm next step');
  }

  return {
    response_text: formatByChannel(request.channel, responseText),
    suggested_actions: suggestedActions,
    follow_up_scheduled: false
  };
}
