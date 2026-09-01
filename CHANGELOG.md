# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Generated `Tag` as the concrete model defined by YouTrack instead of an abstract interface. Callers now construct and inspect `Tag` values directly rather than using the deprecated `IssueTag` implementation.

### Fixed

- Current `$type: "Tag"` responses now decode, and tag request payloads use the current discriminator.

## [0.1.1] - 2026-08-26

### Changed

- Moved the code generator and JSON codec behind internal package boundaries, reducing the public API surface; maintainers now regenerate sources with `go generate ./internal/codegen`.

### Fixed

- Preserved a pager's offset after a failed page request so retrying no longer skips results.
- Improved client JSON handling for exponent-form numbers, Unicode surrogate pairs, trailing whitespace, malformed values, invalid marshaler state, and writer errors.

[unreleased]: https://github.com/arjenjb/go-youtrackapi/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/arjenjb/go-youtrackapi/compare/v0.1.0...v0.1.1
