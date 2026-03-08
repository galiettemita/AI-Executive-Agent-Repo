# notion

Notion page/search mutation adapter.

- Plane: `hands`
- External API target: Notion API v1 (production), deterministic simulation (current)
- Auth: OAuth2 scopes `read_content`, `update_content`, `insert_content`

## Input

- `action` (`search`, `create_page`, `append_block`)
- `query` for search
- `title`/`content` for create
- `page_id`/`content` for append

## Output

- `provider`: `notion`
- `action`
- `page_id` (mutations)
- `pages[]` search or updated page metadata

## Brevio use case

"Add this to my project notes in Notion" -> create/append action with validated fields.
