export interface BirdInput {
  action: 'timeline' | 'search' | 'post';
  query?: string;
  text?: string;
  confirmed?: boolean;
}

export interface BirdPost {
  id: string;
  author: string;
  text: string;
  likes: number;
  reposts: number;
}

export interface BirdOutput {
  provider: 'bird';
  action: BirdInput['action'];
  posts?: BirdPost[];
  posted?: boolean;
  post_id?: string;
}
