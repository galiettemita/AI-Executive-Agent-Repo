export interface RedditInput {
  action: 'search' | 'list_hot' | 'post';
  subreddit?: string;
  query?: string;
  title?: string;
  text?: string;
  confirmed?: boolean;
}

export interface RedditPostSummary {
  id: string;
  subreddit: string;
  title: string;
  score: number;
  url: string;
}

export interface RedditOutput {
  provider: 'reddit';
  action: RedditInput['action'];
  posts?: RedditPostSummary[];
  submitted?: boolean;
  post_id?: string;
}
