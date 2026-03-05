# krea-api

Hands-plane adapter for Krea image generation and upscaling.

## Supported actions

- `generate_image`
- `upscale_image`
- `list_models`

## Notes

- Enforces prompt/image requirements based on action type.
- Returns deterministic model and quality-score metadata.
