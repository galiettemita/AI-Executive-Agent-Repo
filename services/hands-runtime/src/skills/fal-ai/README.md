# fal-ai

Image generation adapter for Fal.ai model execution.

- Plane: `hands`
- External API target: Fal.ai model endpoints (production), deterministic image URL simulation (current)
- Auth: API key (server-side)

## Input

- `prompt` (required)
- `model` (optional)
- `size` (`square`, `portrait`, `landscape`)

## Output

- `provider`: `fal-ai`
- `image_url`
- `model_used`
- `size`

## Safety

- Content policy term filter rejects blocked prompts before image generation.

## Brevio use case

"Generate a social post hero image for tomorrow's launch" -> validated image generation request.
