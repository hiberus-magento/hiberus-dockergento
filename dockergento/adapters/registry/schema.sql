--
-- What this machine knows about the projects on it.
--
-- Intention and allocation, never observed state: which containers exist is asked of Docker,
-- because a registry can go on claiming an environment exists after somebody removed it by hand.
-- What Docker cannot answer is what somebody decided — that this worktree is on the agent profile,
-- that it reads the main checkout's dependencies, that it was given schema seven.
--
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- A project is keyed by its compose project name and not by its path: the path can move, and the
-- name is what decides which containers, which volumes and which database are used.
CREATE TABLE IF NOT EXISTS projects (
    name       TEXT PRIMARY KEY,
    root       TEXT NOT NULL,
    topology   TEXT NOT NULL DEFAULT 'classic',
    created_at TEXT NOT NULL
);

--
-- A branch environment: a git worktree with an environment of its own.
--
-- It lives here and not in the checkout because config/docker/properties.json is a committed file:
-- a worktree's project name written there would travel in somebody's commit and rename the main
-- environment at the same time. Its absence is also the switch — a worktree with no row keeps the
-- guardrails that stop it from recreating the main environment with its own mounts.
--
CREATE TABLE IF NOT EXISTS worktrees (
    project     TEXT NOT NULL REFERENCES projects(name) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    path        TEXT NOT NULL,
    branch      TEXT NOT NULL DEFAULT '',
    profile     TEXT NOT NULL DEFAULT 'agent',
    domain      TEXT NOT NULL DEFAULT '',
    environment TEXT NOT NULL,
    vendor      TEXT NOT NULL DEFAULT 'own',
    created_at  TEXT NOT NULL,
    PRIMARY KEY (project, name)
);

-- Two environments answering to the same compose project name would share containers, volumes and
-- a database without either of them knowing
CREATE UNIQUE INDEX IF NOT EXISTS worktrees_environment ON worktrees(environment);

--
-- Whether this copy of the data has been anonymised, and when.
--
-- Keyed by environment rather than by project, because a branch environment has its own database
-- and answering for it with the main one's record is exactly the mistake that matters.
--
CREATE TABLE IF NOT EXISTS data_state (
    environment   TEXT PRIMARY KEY,
    anonymised_at TEXT NOT NULL
);

--
-- What a worktree was given in the orchestrated topology: its schema, its index prefix, its Redis
-- databases and its queue vhost, all derived from one slot.
--
-- Empty in the classic topology, and here from the first day on purpose. Adding it later means
-- migrating the state of everyone who is already using the tool, which is the one kind of change
-- that cannot be rolled back quietly.
--
CREATE TABLE IF NOT EXISTS allocations (
    project       TEXT NOT NULL,
    worktree      TEXT NOT NULL,
    slot          INTEGER NOT NULL,
    schema_name   TEXT NOT NULL,
    index_prefix  TEXT NOT NULL,
    cache_db      INTEGER NOT NULL,
    page_cache_db INTEGER NOT NULL,
    session_db    INTEGER NOT NULL,
    vhost         TEXT NOT NULL,
    PRIMARY KEY (project, worktree),
    FOREIGN KEY (project, worktree) REFERENCES worktrees(project, name) ON DELETE CASCADE
);

-- Two worktrees on the same slot would share a schema and three Redis databases
CREATE UNIQUE INDEX IF NOT EXISTS allocations_slot ON allocations(project, slot);
