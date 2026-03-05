export type TMDBType = 'movie' | 'tv';

export interface TMDBInput {
  query?: string;
  genre?: string;
  type?: TMDBType;
}

export interface TMDBResult {
  title: string;
  year: number;
  rating: number;
  overview: string;
  streaming: string;
}

export interface TMDBOutput {
  provider: 'tmdb';
  results: TMDBResult[];
}
