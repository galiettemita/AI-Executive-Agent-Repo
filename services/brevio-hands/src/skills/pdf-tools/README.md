# pdf-tools

Document utility adapter for text extraction, merge, and split operations.

## Auth
- No external auth required for local/document pipeline tools.

## Input
- `action`: `extract_text`, `merge`, `split`
- `files` required
- `page_range` required for split
- `output_name` optional output filename

## Output
- `provider`: `pdf-tools`
- action echo with `output_path`, `pages_processed`, optional text preview
