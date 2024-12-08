Every time you add an example, please make a single, simple test that proves the example runs as expected
NEVER run tests automatically unless explicitly asked
when asked to write a test, write just the test without trying to fix it
avoid writing useless comments: if you need to write a comment, explain WHY the code does something instead of WHAT it does
When creating a PR, describe what you did, but don't include the "test plan" section.

## Code Style
- Use `goimports` for formatting (run via `make`)
- Follow standard Go formatting conventions
- Group imports: standard library first, then third-party
- Use PascalCase for exported types/methods, camelCase for variables
- Add comments for public API and complex logic
- Place related functionality in logically named files
- dont leave trivial comments like '// Verify results are correct'

## Error Handling
- Use custom `Error` type with detailed context
- Include error wrapping with `Unwrap()` method
- Return errors with proper context information (line, position)

## Testing
- Write table-driven tests with clear input/output expectations
- Use package `tpl_test` for external testing perspective
- Include detailed error messages (expected vs. actual)
- Test every exported function and error case

## Dependencies
- Minimum Go version: 1.23.0
- External dependencies managed through go modules

## Modernization Notes
- Use `errors.Is()` and `errors.As()` for error checking
- Replace `interface{}` with `any` type alias
- Replace type assertions with type switches where appropriate
- Use generics for type-safe operations
- Implement context cancellation handling for long operations
- Add proper docstring comments for exported functions and types
- Use log/slog for structured logging
- Add linting and static analysis tools