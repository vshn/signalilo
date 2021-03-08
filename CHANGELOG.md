# Changelog

Please document all notable changes to this project in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased]

### Added

- Push image to DockerHub and Quay ([#47])
- Option to configure custom severity to service level mappings ([#52])

### Fixed

- Fix service and downtime listing in garbage collector ([#64])

## [v0.8.0]

### Added

- Option to allow changing the annotation name used for `plugin_output` ([#44])

### Fixed

- Update alertmanager mapping section in README ([#45])

## [v0.7.0]

### Added

- Add ability to disable http keep-alives when connecting to Icinga2 API ([#30])
- Support for injecting variables with static value on all Icinga2 services ([#39])
- Introduce option to continue using CN to verify TLS certificates ([#41])

### Notes

- Going forward, building Signalilo requires Go 1.15+, due to the changes
  introduced in [#41], which make use of the `tls.Config` field
  `VerifyConnection` which was introduced in Go 1.15.
  Users of the Docker image don't need to make any changes.
- By default, the Icinga2 API server name is verified against the
  certificate's CN field.
  If your Icinga2 API certificate is only valid when checking the
  certificate's SAN field, please run Signalilo with
  `--icinga_x509_verify_cn=false` which turns on the new Go default behavior
  which ignores the CN field and checks against the SAN field.

## [v0.6.0]

### Fixed

- Resolved "heartbeat" alerts are handled correctly, i.e. not at all ([#24])
- The go module dependency definition doesn't use `replace` to pull in our fork of `go-icinga2-client` anymore. ([#21])
- `go get` is now able to fetch and install Signalilo ([#19])

## [v0.5.0]

### Added

- Support for "heartbeat" services, i.e. services which alert when they don't receive a regular update.

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

[Unreleased]: https://github.com/vshn/signalilo/compare/v0.8.0...HEAD
[0.1.0]: https://github.com/vshn/signalilo/releases/tag/0.1.0
[0.1.1]: https://github.com/vshn/signalilo/releases/tag/0.1.1
[0.2.0]: https://github.com/vshn/signalilo/releases/tag/0.2.0
[v0.3.0]: https://github.com/vshn/signalilo/releases/tag/v0.3.0
[v0.4.0]: https://github.com/vshn/signalilo/releases/tag/v0.4.0
[v0.5.0]: https://github.com/vshn/signalilo/releases/tag/v0.5.0
[v0.6.0]: https://github.com/vshn/signalilo/releases/tag/v0.6.0
[v0.7.0]: https://github.com/vshn/signalilo/releases/tag/v0.7.0
[v0.8.0]: https://github.com/vshn/signalilo/releases/tag/v0.8.0
[#19]: https://github.com/vshn/signalilo/pull/19
[#21]: https://github.com/vshn/signalilo/pull/21
[#24]: https://github.com/vshn/signalilo/pull/24
[#30]: https://github.com/vshn/signalilo/pull/30
[#39]: https://github.com/vshn/signalilo/pull/39
[#41]: https://github.com/vshn/signalilo/pull/41
[#44]: https://github.com/vshn/signalilo/pull/44
[#45]: https://github.com/vshn/signalilo/pull/45
[#47]: https://github.com/vshn/signalilo/pull/47
[#52]: https://github.com/vshn/signalilo/pull/52
[#64]: https://github.com/vshn/signalilo/pull/64
