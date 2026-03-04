# grocery-list

Hands-plane grocery planning adapter for list CRUD and section-based organization.

## Supported actions

- `add_items`
- `remove_items`
- `list_items`
- `organize_by_section`
- `clear_list` (confirmation required)

## Notes

- Clear-list mutation requires `confirmed=true`.
- Deterministic fixture state is used for local contract validation.
