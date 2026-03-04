import type { FigmaInput, FigmaOutput } from './types.js';

export async function runClient(input: FigmaInput): Promise<FigmaOutput> {
  const fileKey = input.file_key ?? 'figma-unknown';
  const findings =
    input.action === 'audit_accessibility'
      ? ['Contrast issue in button labels', 'Missing alt text on hero image']
      : ['Consistent spacing grid detected', 'Design tokens aligned with style guide'];

  return {
    provider: 'figma',
    action: input.action,
    file_key: fileKey,
    findings,
    export_url:
      input.action === 'export_asset'
        ? `https://assets.brevio.local/figma/${fileKey}/${input.node_id ?? 'node'}.${input.format ?? 'png'}`
        : undefined,
    summary: `Figma action ${input.action} completed for file ${fileKey}.`
  };
}
