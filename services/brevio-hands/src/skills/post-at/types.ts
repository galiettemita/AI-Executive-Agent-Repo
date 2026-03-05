export type PostAtAction = 'track_parcel';

export interface PostAtInput {
  action: PostAtAction;
  tracking_number?: string;
}

export interface PostAtCheckpoint {
  timestamp: string;
  location: string;
  status: string;
}

export interface PostAtOutput {
  provider: 'post-at';
  action: PostAtAction;
  tracking_number: string;
  latest_status: string;
  checkpoints: PostAtCheckpoint[];
  summary: string;
}
