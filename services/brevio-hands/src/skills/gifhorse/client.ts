import type { GifhorseInput, GifhorseOutput } from './types.js';

export async function runClient(input: GifhorseInput): Promise<GifhorseOutput> {
  return {
    provider: 'gifhorse',
    action: 'search_gif',
    gifs: [
      {
        caption: `Reaction for ${input.query}`,
        gif_url: 'https://media.example.com/reaction.gif'
      }
    ],
    summary: `Found 1 GIF candidate for ${input.query}.`
  };
}
