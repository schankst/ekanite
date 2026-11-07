# Changelog

Alle nennenswerten Änderungen an diesem Projekt werden hier dokumentiert.
Das Format orientiert sich an [Keep a Changelog](https://keepachangelog.com/de/1.1.0/).

**Historic:** v1.3.0 (2017) war das letzte Upstream-Release. Dieser Fork (`schankst/ekanite`) dokumentiert ab 2026 Änderungen im Abschnitt „Unreleased“, bis ein neues Release geschnitten wird.

## [Unreleased]

### Added
- Fork-Roadmap und Agent-/Wellen-Dokumentation (`docs/ROADMAP.md`, `docs/AGENTS.md`).

### Changed
- Go-Toolchain-/Modul-Modernisierung im Fork-Kontext (u. a. `go mod`/Dependency-Pflege gemäß Projektstand).

### Fixed
- (Noch keine veröffentlichten Fix-Einträge in diesem Abschnitt.)

### Docs
- Suchverhalten, Limits und Query-Syntax dokumentiert (README/docs).
- README/Flags-Beschreibung an tatsächliche Eingabeformate, UDP und TLS-Verhalten angeglichen; fehlende GoDoc-Kommentare für exportierte Parser-/Input-Symbole ergänzt.
- Hinweise zu Secure-Development-Themen und Modell-/Betriebs-Policy werden im Rahmen der Fork-Dokumentation gepflegt (siehe Issues/Roadmap).

## 1.4.0 (unreleased, historisch)
- [PR #95](https://github.com/ekanite/ekanite/pull/95): Refactor for multiple parsers. Thanks @jweisscrypto
- [PR #97](https://github.com/ekanite/ekanite/pull/97): Enable 'go mod' support via Go 1.13.

## 1.3.0 (November 26th 2017)
With this release, Ekanite is moving to Go 1.9.1.

- [PR #81](https://github.com/ekanite/ekanite/pull/81): Close engine on exit.
- [PR #82](https://github.com/ekanite/ekanite/pull/82): Name shards so that they are always loaded correctly on restart. 

## 1.2.3 (July 6th 2017)
- [PR #78](https://github.com/ekanite/ekanite/pull/78): Type "time" cannot be used as an expvar value.

## 1.2.2 (June 25th 2017)
- [PR #77](https://github.com/ekanite/ekanite/pull/77): Don't panic if interface already in use. Thanks @mysinmyc

## 1.2.1 (June 17th 2017)
With this release Ekanite is moving to Go 1.8.
- [PR #76](https://github.com/ekanite/ekanite/pull/76): Don't share parsers. Fixes [issue #75](https://github.com/ekanite/ekanite/issues/75). Thanks @mysinmyc 

## 1.2.0 (January 3rd 2017)
- [PR #63](https://github.com/ekanite/ekanite/pull/63): Add search duration to HTTP query service output.
- Cyclomatic complexity improvements.
- With this release Ekanite is moving to Go 1.7.

## 1.1.1 (October 6th 2016)
- [PR #60](https://github.com/ekanite/ekanite/pull/60): Integrate latest bleve API changes.
- Go cyclo changes.

## 1.1.0 (September 3rd 2016)
- [PR #57](https://github.com/ekanite/ekanite/pull/57): Correctly handle closed TCP connections.
- [PR #37](https://github.com/ekanite/ekanite/pull/37): Move debug var support to dedicated HTTP server.
- [PR #36](https://github.com/ekanite/ekanite/pull/36): Remove noisy log message.
- [PR #52](https://github.com/ekanite/ekanite/pull/52): Close file descriptor after listing shards in directory.
- [PR #53](https://github.com/ekanite/ekanite/pull/53): Very minor refactor of UDP start control.
- Go lint fixes.

## 1.0 (April 10th 2016)
Check out [this tag](https://github.com/ekanite/ekanite/releases/tag/v1.0.0) for full details.
