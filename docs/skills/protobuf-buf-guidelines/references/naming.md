# Naming Conventions

This reference defines consistent naming for Protobuf APIs.

## File names
- Use `lower_snake_case.proto`
- Use nouns for resource-like definitions (e.g., `book.proto`) and `service.proto` for service definitions if you prefer a single entry file per version.

## Packages
- Lowercase segments
- Version as the last segment: `.v1`, `.v2`

## Messages
- Use UpperCamelCase: `CreateBookRequest`
- Prefer suffixes/prefixes that signal intent:
  - Requests: `*Request`
  - Responses: `*Response`

## Fields
- Use lower_snake_case: `page_size`, `book_id`
- Reserve `name` for canonical resource identifier where applicable.

## Enums
- Enum type: UpperCamelCase (e.g., `BookStatus`)
- Enum values: UPPER_SNAKE_CASE
- Zero value: `*_UNSPECIFIED` (or `UNSPECIFIED`) and treat it as invalid/unknown.

## Services and RPCs
- Service name: UpperCamelCase noun (`LibraryService`)
- RPC name: UpperCamelCase verb phrase:
  - Standard methods: `GetBook`, `ListBooks`, `CreateBook`, `UpdateBook`, `DeleteBook`
  - Custom actions: `PublishBook`, `MoveBook`

## Comments
- Add a short comment for every service, RPC, message, and non-obvious field.
- Put constraints either in validation annotations (preferred) or as explicit comments.
