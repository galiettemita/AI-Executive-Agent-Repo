# icloud-findmy

Find My device location adapter.

- Plane: `hands`
- External API target: iCloud Find My bridge (pyicloud)
- Auth: Apple ID session with 2FA (production)

## Input

- `device_name` (optional filter)

## Output

- `provider`: `icloud-findmy`
- `devices[]` with name, coordinates, battery

## Brevio use case

"Where are my AirPods?" -> returns latest available location and battery status.
