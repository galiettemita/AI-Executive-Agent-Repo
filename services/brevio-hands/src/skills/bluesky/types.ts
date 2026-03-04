export interface BlueskyInput {
  action: 'timeline' | 'search' | 'post';
  query?: string;
  text?: string;
  confirmed?: boolean;
}

export interface BlueskyPost {
  uri: string;
  author_handle: string;
  text: string;
  like_count: number;
}

export interface BlueskyOutput {
  provider: 'bluesky';
  action: BlueskyInput['action'];
  posts?: BlueskyPost[];
  posted?: boolean;
  uri?: string;
}
