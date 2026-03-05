import type { ApplePhotoItem, ApplePhotosInput, ApplePhotosOutput } from './types.js';

const ALBUMS = ['Family', 'Italy Trip', 'Work Events', 'Screenshots'];

const PHOTOS: ApplePhotoItem[] = [
  {
    photo_id: 'photo_001',
    filename: 'italy-colosseum.jpg',
    captured_at: '2025-06-14T10:45:00.000Z',
    album: 'Italy Trip',
    tags: ['travel', 'rome', 'landmark']
  },
  {
    photo_id: 'photo_002',
    filename: 'family-dinner.jpg',
    captured_at: '2026-02-20T01:10:00.000Z',
    album: 'Family',
    tags: ['family', 'dinner']
  },
  {
    photo_id: 'photo_003',
    filename: 'launch-event-stage.jpg',
    captured_at: '2026-01-11T19:25:00.000Z',
    album: 'Work Events',
    tags: ['work', 'event', 'stage']
  }
];

export async function runClient(input: ApplePhotosInput): Promise<ApplePhotosOutput> {
  if (input.action === 'list_albums') {
    return {
      provider: 'apple-photos',
      action: input.action,
      albums: ALBUMS,
      photos: [],
      summary: `Found ${ALBUMS.length} album(s) in Apple Photos.`
    };
  }

  const limit = input.limit ?? 10;

  if (input.action === 'search_photos') {
    const query = (input.query ?? '').toLowerCase();
    const photos = PHOTOS.filter(
      (photo) =>
        photo.filename.toLowerCase().includes(query) ||
        photo.tags.some((tag) => tag.toLowerCase().includes(query))
    ).slice(0, limit);

    return {
      provider: 'apple-photos',
      action: input.action,
      albums: ALBUMS,
      photos,
      summary: `Search returned ${photos.length} photo result(s) for query "${input.query}".`
    };
  }

  return {
    provider: 'apple-photos',
    action: input.action,
    albums: ALBUMS,
    photos: PHOTOS.slice(0, limit),
    summary: `Returned ${Math.min(limit, PHOTOS.length)} recent photo(s).`
  };
}
