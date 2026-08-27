# Tasks

## Template storage

- [x] Add `console/tasks/db_template.sh`: naming, labels, lookup, size and listing
- [x] Resolve `<project>/<name>` addressing, defaulting the project to the current one
- [x] Record `hm.project`, `hm.root`, `hm.template`, `hm.db_image` and `hm.created` on the volume

## Freezing

- [x] `hm db freeze [--name=<name>] [--force]`
- [x] Stop the database service only while copying, and start it again afterwards
- [x] Refuse an existing name without `--force`, and refuse an empty data directory

## Cloning

- [x] `hm db clone [<address>] [--force]`
- [x] Refuse while any container of the project runs, with the blocked exit code
- [x] Confirm by typing the project name when existing data would be replaced
- [x] Compare the database image and refuse a mismatch without `--force`

## Listing and deleting

- [x] `hm db templates [--json]`
- [x] `hm db drop <address>` with confirmation and reported size
- [x] Refuse to delete a template attached to a running container

## Verification

- [x] Unit tests for addressing, naming and the image comparison
- [x] Integration test: freeze, clone into an empty project, verify the data is there
- [x] Integration test: refusals (running environment, image mismatch, missing template)
- [x] `docs/db.md` updated with templates and when to use them instead of snapshots
- [x] Changelog entry and backlog note
