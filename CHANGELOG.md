# Changelog

Please document all notable changes to this project in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## [v0.4.0]

### Added

- Allow passing the bearer token via URL query parameter (`/webhook?token=<token>`) in addition to the `Authorization` header. Header takes precedence.

## [v0.3.0]

### Added

- Added ChangeLog (this file)

### Changed

- Moved code and CI pipeline to GitHub
- Changed version tags to have a `v` prefix

## [0.2.0]

### Added

- Support for passing through custom variables on Icinga checks

## [0.1.1]

### Changed

- Improved README
- Improved log messages in webhook handler

## [0.1.0]

Initial implementation

[0.1.0]: https://github.com/vshn/signalilo/releases/tag/0.1.0
[0.1.1]: https://github.com/vshn/signalilo/releases/tag/0.1.1
[0.2.0]: https://github.com/vshn/signalilo/releases/tag/0.2.0
[v0.3.0]: https://github.com/vshn/signalilo/releases/tag/v0.3.0
