export type GifhorseAction = 'search_gif';

export interface GifhorseInput {
  action: GifhorseAction;
  query?: string;
}

export interface GifhorseItem {
  caption: string;
  gif_url: string;
}

export interface GifhorseOutput {
  provider: 'gifhorse';
  action: GifhorseAction;
  gifs: GifhorseItem[];
  summary: string;
}
