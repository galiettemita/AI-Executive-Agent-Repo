export interface TraktInput {
  action: 'history' | 'trending' | 'mark_watched';
  media_id?: string;
  media_type?: 'movie' | 'show';
}

export interface TraktItem {
  id: string;
  title: string;
  media_type: 'movie' | 'show';
  year?: number;
}

export interface TraktOutput {
  provider: 'trakt';
  action: TraktInput['action'];
  items?: TraktItem[];
  marked_watched?: boolean;
}
