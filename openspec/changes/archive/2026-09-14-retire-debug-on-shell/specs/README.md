# Spec phase: retire-debug-on-shell

What was checked, what was found, and why one delta was written and a second, tempting one was
not.

## What was checked

- `openspec/changes/retire-debug-on-shell/proposal.md` and `exploration.md`, read in full.
- The direct precedent, `openspec/changes/archive/2026-09-11-retire-read-only-shell-commands/`
  (`proposal.md`, `specs/go-entrypoint/spec.md`, `archive-report.md`), to see how the last
  retirement cycle handled its own spec phase.
- The live, merged `openspec/specs/go-entrypoint/spec.md`, not just the proposal's citations of
  it — specifically the "A command proved in Go has no shell twin" requirement (`:1011-1063`) and
  the "A ported command's engine interaction is provable without Docker" requirement (`:253-859`),
  whose scenario "The xdebug host environment variable is not consulted" (`:604-615`) contains the
  premise the proposal flags at `:606`.
- `tests/unit/command_safety_test.sh:55-79`, the live `RETIRED` array and its drift check, against
  what the merged spec's enumeration currently names.

## Finding 1: the proposal's "spec.md — Unchanged" is not correct

The proposal's Affected Areas table lists `openspec/specs/go-entrypoint/spec.md` as **Unchanged**,
reasoning only about the `:606` premise. That reasoning is sound as far as it goes, but it answers
a narrower question than the one that matters here.

The direct precedent's own delta (`archive/.../specs/go-entrypoint/spec.md:1-14`) shows that when
`list`, `describe`, `logs` were added to the shell-retirement mechanism, the cycle wrote a
`MODIFIED Requirements` delta for exactly one requirement — "A command proved in Go has no shell
twin" — growing its enumerated name list from twelve to fifteen, and updating the "walks the
fifteen by name (Previously: twelve)" cross-reference in the same edit. This requirement's
normative text is itself a live enumeration of every command the mechanism currently covers; it is
not narrative commentary.

This cycle grows the same array by the same mechanism — `debug-on` added to `RETIRED`
(`command_safety_test.sh:69-71`, 15 names to 16), gated on the same deletion, verified by the same
mutation check. By direct, unambiguous analogy to the one precedent this task named, that
requirement owes the same kind of `MODIFIED` delta this cycle. It has been written:
`specs/go-entrypoint/spec.md` in this change, growing the enumeration to sixteen and renaming the
per-name scenario to its singular case.

**Correction carried forward**: the proposal's Affected Areas table should read `openspec/specs/
go-entrypoint/spec.md` as **Modified** (one small, mechanical `MODIFIED Requirements` delta), not
Unchanged. The proposal's broader point — that no *behavioral* spec changes, because nothing a
user types changes — still holds; the delta only extends an enumeration, it adds no new scenario
about behavior.

## Finding 2: the `:606` premise — recorded, not corrected, and this is deliberate

The proposal is right that `:606`'s GIVEN goes vacuous, and right that it is the weakest case for
leaving. Verified independently: that scenario belongs to "A ported command's engine interaction
is provable without Docker" (`:253-859`), a single requirement spanning roughly 600 lines and
scenarios for `debug-off`, `debug-on`, `magento`, `logs`, and others unrelated to this deletion.
The `sdd-spec` MODIFIED workflow requires copying the **entire** requirement block, not just the
touched scenario, because the archive step replaces the whole block. Doing that here to fix one
premise clause — whose `WHEN`/`THEN`/test are unaffected, and whose conclusion the deletion
strengthens rather than weakens — would multiply this artifact by roughly an order of magnitude
for content that does not change in truth-relevant ways.

Judged disproportionate, for the same reason the previous cycle gave when it declined to amend its
own stale "no shell implementation any more" comment written two commits before it became true:
content that is true when written is left, and the fact that it later goes stale (here: vacuous)
is recorded rather than silently fixed. `:606`'s GIVEN is accepted as a harmless vacuous premise.
No delta touches it. This is a decision, not an oversight — recorded here so a future reader who
notices the GIVEN no longer describes reality finds this note rather than a silent gap.

## Finding 3: `:466` and `:570`, checked and left alone

Both cited by the proposal as spec-file mentions of the deleted files. Verified against the live
spec: `:466` ("`restart.sh` ... the faithful port of `debug-on.sh:26`") and `:570` ("`restart.sh`,
called from inside `debug-on.sh`, never re-entered `bin/run`'s own gate") are provenance about the
shell script's past behavior, not premises about present repository state — the same class as the
nine source-comment citations the proposal already decided to leave. Both stay true after
deletion. No delta needed for either; not touched.

## Finding 4: the unchanged "gains one such case for describe" note

The `MODIFIED` delta written for this cycle carries forward one line unedited on purpose: "Pinned
by the `command_moved_to_binary` cases in `tests/integration/cli_contract_test.sh`, which gains
one such case for `describe`." This sentence was already non-exhaustive before this cycle — it
named only `describe` although `list` and `logs` were added in the same historical change, and
`set-host`/`version` before that — so it was never a live inventory to keep synchronized, only an
illustrative note from whichever cycle last touched it. Left as written, on the same "true when
written" basis as Finding 2 and the proposal's own dangling-citations decision. Not a gap: the
requirement's enumeration (the part that is a live inventory) was updated; this narrative aside was
not, deliberately.

## Net result

One `MODIFIED Requirements` delta at `specs/go-entrypoint/spec.md`, sized to the one requirement
whose enumeration this cycle's `RETIRED` addition extends. No delta for `magento-commands` or any
other domain — the proposal's scope table names none, and nothing in this cycle adds, modifies, or
removes behavior in a domain other than the shell-retirement mechanism itself.
