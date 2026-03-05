import type { GammaInput, GammaOutput } from './types.js';

export async function runClient(input: GammaInput): Promise<GammaOutput> {
  const deckID = input.deck_id ?? 'gamma-deck-001';
  const title = input.topic ?? 'Updated Gamma Deck';
  const slides = input.slide_count ?? 8;

  return {
    provider: 'gamma',
    action: input.action,
    deck_id: deckID,
    title,
    slide_count: slides,
    export_url: input.action === 'export_deck' ? `https://assets.brevio.local/gamma/${deckID}.${input.format ?? 'pdf'}` : undefined,
    summary: `Gamma action ${input.action} completed for ${deckID} (${slides} slides).`
  };
}
