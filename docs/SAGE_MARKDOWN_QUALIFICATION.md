# Sage deterministic Markdown writer qualification

Validated on 2026-09-30 on `features/TASK-157-sage-parity`, following queue commit `5c64271`.
The reference is `ai_sessions_skills` commit `b85a1ab`, specifically `tool/bin/kb.py`
and its entry parsing/mutation tests in `tool/tests/test_portable.py`.

## Implemented library behavior

`internal/sage/knowledge.Writer` accepts a validated draft, a caller-owned hexadecimal
signature, and a topic. It creates a portable topic filename, deterministic Markdown with
`##` sections and `###` entries, and a sorted topic index in `README.md`. New entries append
within a section in document order, as in the reference; replay is byte-stable. The writer
accepts reference signatures of 6–40 hex characters and longer Go digests up to 64 characters.

The parser reads heading, section, signatures, and the first command fence. Code-fenced
headings/signatures never become structural metadata. Rendering escapes HTML in prose,
flattens headings, and chooses a fence longer than any backtick sequence in commands.
Filename normalization also guards Windows devices, drive/stream syntax, reserved index names,
and traversal. Existing symlink files and symlink directory components are rejected.

An existing signature anywhere in the topic files wins over the proposed topic. Its prose and
file placement remain untouched; a replay repairs the index when necessary. `AddSignature`
attaches a caller-identified variant to an existing entry without rewriting its content.
Determining semantic action equivalence belongs to the upcoming routing/signature work.

Writes use a synced private temporary file followed by same-directory replacement, with
Windows `MoveFileEx` replacement/write-through flags and POSIX rename. No truncate-in-place
step exists. Each file is replaced separately: topic first, index second. An index failure
returns an error; retry discovers the published signature and repairs the index without a
duplicate entry. The index has an owned marker block, preserving user text outside it.
Malformed marker blocks or unclosed topic fences cause an error rather than a destructive repair.

## Evidence

- Seven named `TestReference...` tests directly port filename, entry parsing, multiple-signature,
  idempotent signature addition, file creation, section reuse, and earlier-entry preservation
  scenarios. Their parity ledger rows now reference executable tests.
- Golden rendering and structural-injection tests cover model text inside headings and fences.
- Fault injection at topic and index replacement proves original-file preservation and retry
  recovery. A corrected-location replay test proves cross-file signature reuse and index repair.
- Concurrent writer tests prove no lost updates between instances in one process.
- Malformed-file and symlink tests cover refusal without modifying unrelated content.
- Native Windows: full uncached `go test ./... -count=1 -timeout 60s` and `go vet ./...` passed.
- Fedora WSL2: writer tests under `-race`, writer vet, and a ten-second two-worker fuzz run passed
  (164,435 executions). The fuzz target checks that arbitrary command text cannot create extra
  entries or signatures. WSL is Linux coverage, not native-machine qualification.

The first test run exposed an incorrect test expectation of seven command lines instead of six
in the structural-injection fixture. The parser output was correct; the assertion was corrected.

## Remaining integration and limits

This is a qualified writer library. The daemon still persists drafts as `distilled`; it does
not yet call this writer or claim `documented` completion. Next connect durable publication
claims/recovery, choose the configured knowledge root and stable signature mapping, and prove
spool-to-Markdown restart behavior before acknowledging a documented job.

The daemon must be the single publishing process. A package mutex serializes writer instances
inside that process; there is no cross-process writer lock. Concurrent external editing or
hostile filesystem replacement is not transactionally coordinated. Replacing a file does not
promise recovery from every power-loss/filesystem failure. Files are bounded to 4 MiB each;
oversized topics return an error without truncation.

Reference fuzzy section matching, action-derived variant matching, route correction commands,
merge/refile, skipped-action files, full-entry search indexing, local commits, and operational
diagnostics remain open. Full Sage parity and the end-to-end acceptance gate remain open.
