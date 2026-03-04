export type ApplePhotosAction = 'list_albums' | 'search_photos' | 'recent_photos';

export interface ApplePhotosInput {
  action: ApplePhotosAction;
  album_name?: string;
  query?: string;
  date_from?: string;
  date_to?: string;
  limit?: number;
}

export interface ApplePhotoItem {
  photo_id: string;
  filename: string;
  captured_at: string;
  album: string;
  tags: string[];
}

export interface ApplePhotosOutput {
  provider: 'apple-photos';
  action: ApplePhotosAction;
  albums: string[];
  photos: ApplePhotoItem[];
  summary: string;
}
