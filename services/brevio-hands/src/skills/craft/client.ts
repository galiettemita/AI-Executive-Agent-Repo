import type { CraftInput, CraftOutput } from './types.js';

export async function runClient(input: CraftInput): Promise<CraftOutput> {
  const docs = [
    {
      doc_id: input.doc_id ?? 'craft-doc-001',
      title: input.doc_title ?? 'Craft Document',
      updated_at: '2026-03-04T18:05:00.000Z'
    }
  ];

  return {
    provider: 'craft',
    action: input.action,
    docs,
    summary: `Craft action ${input.action} completed for ${docs[0]?.doc_id}.`
  };
}
