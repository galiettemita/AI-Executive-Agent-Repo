# streaming-recommendations

Custom-build adapter for personalized streaming recommendations and watchlist actions.

## Auth
- Pending partner API integrations (streaming catalogs/watchlist APIs).

## Input
- `action`: `recommend`, `watchlist_add`, `watchlist_list`
- `recommend`: needs `mood` or `genre`
- `watchlist_add`: needs `title` and `confirmed: true`

## Output
- `provider`: `streaming-recommendations`
- action echo with recommendations/watchlist flags
- `partnership_status`: `awaiting_api_partnership`

## Notes
- `// CUSTOM_BUILD_REQUIRED: Awaiting API partnership` enforced in adapter runtime.
