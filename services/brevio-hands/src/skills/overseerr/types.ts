export type OverseerrAction = 'search_media' | 'request_media' | 'list_requests';

export interface OverseerrInput {
  action: OverseerrAction;
  query?: string;
  media_type?: 'movie' | 'tv';
  media_id?: string;
}

export interface OverseerrRequest {
  request_id: string;
  media_id: string;
  title: string;
  media_type: 'movie' | 'tv';
  status: 'pending' | 'approved';
}

export interface OverseerrOutput {
  provider: 'overseerr';
  action: OverseerrAction;
  requests: OverseerrRequest[];
  summary: string;
}
