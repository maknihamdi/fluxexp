---
name: openspec-sync-specs
description: Fold a change's delta specs into the main specs under openspec/specs/. Use when archiving a change, or whenever ADDED/MODIFIED/REMOVED/RENAMED requirements need to be applied to a capability's spec.
license: MIT
compatibility: Requires openspec CLI.
metadata:
  author: fluxexp
  version: "1.0"
---

Apply a change's delta specs to the main specs, so `openspec/specs/<capability>/spec.md`
states the behavior the change introduced.

`openspec-archive-change` (and `/opsx:archive`) delegates its sync step to this skill and
then moves the change directory itself. **This skill never moves, archives or commits
anything** — it only edits `openspec/specs/`.

> There is also `openspec archive <change>`, which syncs *and* moves in one CLI step. Do
> not use it here: it would move the directory out from under the caller. Use it only when
> the user asks for the non-experimental archive flow directly.

**Input**: the change name. The caller may pass its own delta analysis — treat it as a
hint, and re-read the files yourself before editing.

**Steps**

1. **Locate the delta specs**

   ```bash
   ls openspec/changes/<name>/specs/*/spec.md
   ```

   No delta specs → report "nothing to sync" and stop. This is a normal outcome for
   tooling- or docs-only changes.

2. **For each capability, read both sides**

   - delta: `openspec/changes/<name>/specs/<capability>/spec.md`
   - main: `openspec/specs/<capability>/spec.md`

   If the main spec does not exist (a new capability), create it with the scaffold the
   CLI uses, then apply the delta's ADDED requirements into it:

   ```markdown
   # <capability> Specification

   ## Purpose
   TBD - created by archiving change <name>. Update Purpose after archive.
   ## Requirements
   ```

3. **Apply the delta operations, in this order**

   The delta groups requirements under `## ADDED Requirements`,
   `## MODIFIED Requirements`, `## REMOVED Requirements`, `## RENAMED Requirements`.
   Apply renames first, so the headers the other operations match against are the
   current ones.

   | Operation | What to do in the main spec |
   |---|---|
   | RENAMED | Change the `### Requirement:` header from FROM: to TO:, keeping the body untouched |
   | REMOVED | Delete the whole requirement block. **Never** copy the delta's `**Reason**` / `**Migration**` lines into the main spec — they document the change, not the behavior |
   | MODIFIED | Replace the whole requirement block with the delta's version, verbatim |
   | ADDED | Append the requirement block at the end of the `## Requirements` section |

   **Matching rule**: a requirement is identified by its `### Requirement: <name>` header,
   compared case-sensitively but whitespace-insensitively. Its block runs from that header
   to the next `### ` header or end of file.

   **Never copy the `## ADDED/MODIFIED/REMOVED/RENAMED Requirements` headers themselves**
   into the main spec — they exist only in delta files.

4. **Stop and report instead of guessing**

   - A MODIFIED or REMOVED header matches no requirement in the main spec → do not add it
     silently; report the mismatch and let the caller decide (usually a typo or a rename
     that was not declared).
   - An ADDED requirement already exists in the main spec → that capability is already
     synced. Leave it as it is; do not append a duplicate.
   - A MODIFIED block carries **fewer scenarios** than the requirement it replaces → this is
     the documented pitfall of a partial MODIFIED, which loses detail at archive time. Flag
     it before replacing, quoting what would be dropped.

5. **Leave everything else byte-for-byte**

   Do not reorder requirements, reflow prose, or fix unrelated formatting in the main spec.
   A sync diff should show only the requirements the delta names.

6. **Validate every capability touched**

   ```bash
   openspec validate <capability> --strict
   ```

   Scenarios must use exactly four hashtags (`#### Scenario:`) — three fails silently — and
   every requirement needs at least one scenario and a SHALL/MUST in its prose. If
   validation fails, fix the spec (or report why the delta cannot be applied as written)
   rather than leaving the main spec invalid.

**Output**

Report, per capability:
- requirements added, replaced, removed, renamed — by name
- the validation result
- anything skipped, and why

State explicitly that the change directory was not moved and nothing was committed.

**Guardrails**
- Never move, delete or archive `openspec/changes/<name>/` — the caller does that.
- Never run `openspec archive`.
- Never commit; leave the edits in the working tree.
- Re-running the skill on an already-synced change must be a no-op, not a duplication.
- The delta is the source of truth for the requirements it names, and only those.
