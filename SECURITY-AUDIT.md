# Security audit of upstream gc-cli

Fork of [Timmy6942025/gc-cli](https://github.com/Timmy6942025/gc-cli) at upstream
commit `36e7c90`. Audited 2026-09-25 before any account was connected to it.

## Verdict

**Nothing nefarious found.** No exfiltration, no telemetry, no install-time code
execution, no credential mishandling, no obfuscation. The OAuth implementation is
textbook-correct. Five weaknesses were found and fixed on this fork; four are
excess privilege or correctness, one is a privacy concern about routing consent
through a third party.

## What was checked, and what it showed

| Check | Result |
| --- | --- |
| Every URL in the tree | Only `googleapis.com`, `accounts.google.com`, `classroom.google.com`, `calendar.google.com`. No author-controlled endpoint anywhere. |
| Shell/process execution | One `spawnSync` in `bin/gc-cli.js`, which execs the bundled binary with a fixed path derived from `process.platform`/`process.arch`. No shell, no interpolation. |
| npm install hooks | None. No `preinstall`, `install` or `postinstall`, so `npm install` executes no code. |
| Token storage | OS keyring via `zalando/go-keyring` (Keychain on macOS). Never written to a file. |
| Config file permissions | `0600`. |
| Cache contents (`internal/store/sqlite.go`) | Course/coursework metadata only. No tokens. |
| Telemetry | `telemetry_opt_in` exists in the config struct but **no code reads it** and there is no reporting endpoint. Dead field, not a backdoor. |
| Secrets in git history | None. `git log -p --all` shows no client secret was ever committed; it has always been an empty ldflags injection point. |
| Dependencies | All first-party Google, `spf13/cobra`, `zalando/go-keyring`, `modernc.org/sqlite`. `go mod verify` passes against the Go checksum database. |
| CI workflow | Checkout, setup-go, `gofmt -l`, `go test`. No secrets, no publish step, pinned major action versions. |
| OAuth flow (`internal/auth/manager.go`) | PKCE S256, `state` generated from `crypto/rand` and verified on callback, loopback redirect on `127.0.0.1` with an ephemeral port, `access_type=offline`. Correct. |
| Build and tests | Builds clean, `go vet` clean, `go test ./...` passes. |

## Findings, and the fix applied here

### 1. Consent was routed through the author's Google Cloud project (privacy)

`internal/auth/default_client.go` shipped a hardcoded OAuth client ID belonging to
upstream (`597878429548-…`). A default install therefore authorized against a
third party's Cloud project. The author cannot intercept tokens — those go
straight from this machine to Google — but they do control the consent screen,
the app's verification state and its publishing status, none of which a user can
audit. An app left in "Testing" also issues refresh tokens that expire after
7 days.

**Fixed:** this fork ships no default client. `GC_OAUTH_CLIENT_ID` /
`GC_OAUTH_CLIENT_SECRET` (or `--client-id` / `--client-secret`) are required, and
the error explains how to create one. The client ID is live upstream and was left
untouched there; it is simply no longer used by default.

### 2. Writes and deletes were one typo away (excess privilege)

The binary can delete a course, delete coursework, remove a student, turn in a
submission and assign grades. Under automation that blast radius is unacceptable.

**Fixed:** `internal/cli/guard.go` refuses any command that can change Classroom
state unless `GC_ALLOW_WRITES=1` is set. The gate is an **allowlist**, so a
command added later is refused until someone classifies it. Covered by tests,
including one that walks the whole command tree.

### 3. `--scopes` was broken and its documented example could not work

Short scope names were passed to Google verbatim. Google rejects the entire
authorization request with
`invalid_scope: Some requested scopes were invalid. {invalid=[classroom.courses.readonly]}`
— verified directly against `accounts.google.com`. Upstream's own `--scopes`
example in `gc auth login --help` fails for this reason.

**Fixed:** `auth.ExpandScope` expands short names to full scope URLs; full URLs
and `openid` pass through unchanged.

### 4. Default login requested other people's data (excess privilege)

`DefaultReadScopes` included `classroom.rosters.readonly` and
`classroom.coursework.students.readonly` — classmates' names, email addresses and
submitted work. Nothing in the read path needs them.

**Fixed:** both removed from the default set. Still available via `--scopes` for
anyone who needs them.

### 5. `classwork list` requested a teacher-only scope (correctness)

It asked for `classroom.coursework.students.readonly`, which a student account is
never granted, so the command 403'd for exactly the users most likely to run it.

**Fixed:** now requests `classroom.coursework.me.readonly`.

## Also fixed

Errors were printed twice — once by `app.Printer`, once by cobra — which put
non-JSON text on the `--json` stream. `SilenceErrors` is now set.

## Upstream limitation, closed on this fork

**Upstream could not download an attached worksheet.** `internal/drive/client.go`
had no download or export call at all; `GetFile` returned metadata only
(`id,name,mimeType,webViewLink`). The only Drive scope requested was `drive.file`,
which by design reaches only files the app itself created. So upstream could tell
you an assignment exists, when it is due and where its attachment lives — it
could not fetch the attachment's bytes.

This fork adds that, because fetching the attached homework PDF is the entire
point of adopting the tool:

- `internal/drive/download.go` — `DownloadFile` streams a binary attachment
  verbatim and exports a Google-native Doc/Slides/Sheet/Drawing to PDF. Folders,
  shortcuts and Google Forms have no byte stream, so they return
  `ErrNotDownloadable` and are reported and skipped rather than failing the run.
- `internal/cli/materials.go` — `gc materials list` enumerates every attachment
  per assignment with its kind, and `gc materials fetch --out DIR` writes the
  Drive-backed ones to disk, one subdirectory per assignment.
- Attachment names are teacher-authored free text that routinely contains
  slashes, so `SafeFileName` reduces every name to a single path element. A
  property test asserts that traversal inputs (`../../etc/passwd`, `..`,
  absolute paths, NUL bytes) cannot escape the destination directory.
- Both new commands are classified read-only in the `GC_ALLOW_WRITES` gate:
  `fetch` writes only to the local directory the operator named and changes no
  Classroom state.

### The scope cost of this, stated plainly

Downloading a teacher's attachment requires `drive.readonly`, which grants read
access to **the entire Drive of the authorizing account**, not just coursework
attachments. Google offers no narrower scope for "files other people shared with
me": `drive.file` is app-created-only, and there is no coursework-attachment
scope. That is a real privilege increase over the rest of this fork's read path.

It is therefore **not** added to `DefaultReadScopes`. Only `gc materials fetch`
requests it, so `auth login` and every other read command continue to authorize
without it, and the broad grant only happens if and when someone downloads.
