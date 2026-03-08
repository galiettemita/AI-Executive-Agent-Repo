# parcel-package-tracking

Tracks global shipments with carrier inference and event timeline output.

## Auth
- API key in production (mocked in deterministic local adapter).

## Input
- `tracking_number` required (8-40 chars)
- `carrier` optional (`auto`, `ups`, `usps`, `fedex`, `dhl`)
- `locale` optional response locale

## Output
- `provider`: `parcel`
- `tracking_number`, `carrier`, `status`
- `eta` optional estimated delivery timestamp
- `history[]` event timeline

## Notes
- Supports package-tracking disambiguation for international carriers.
