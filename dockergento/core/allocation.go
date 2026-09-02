package core

import (
	"fmt"
	"strings"
)

//
// What a worktree is given in the orchestrated topology.
//
// One daemon per stateful service, and the isolation is logical: Magento supports all four
// natively, so nothing has to be invented. A schema in MariaDB, an index prefix in OpenSearch,
// three numbered databases in Redis and a vhost in RabbitMQ.
//

// databasesPerWorktree is three because Magento keeps the cache, the page cache and the sessions
// apart, and mixing them across branches is how a flush in one clears another's sessions.
const databasesPerWorktree = 3

// RedisDatabases is what the shared Redis has to be configured with. The default is 16, which is
// five worktrees; 128 is what makes the model worth having.
const RedisDatabases = 128

// MaxSlots is how many worktrees a project can have at once in the orchestrated topology, and it
// is Redis that decides it — every other service isolates by name, which does not run out.
const MaxSlots = RedisDatabases / databasesPerWorktree

// Allocation is what one worktree was given.
type Allocation struct {
	// Slot is the number everything else is derived from, and the only thing that has to be
	// handed out atomically. It is reused once a worktree is gone.
	Slot int `json:"slot"`

	Schema      string `json:"schema"`
	IndexPrefix string `json:"index_prefix"`

	CacheDB     int `json:"cache_db"`
	PageCacheDB int `json:"page_cache_db"`
	SessionDB   int `json:"session_db"`

	VHost string `json:"vhost"`
}

// AllocationFor derives everything a worktree needs from its name and its slot.
//
// Pure, so that the naming can be checked without a database, and derived rather than stored
// per-field so that two worktrees cannot end up with the same schema through a bad write.
func AllocationFor(worktree string, slot int) Allocation {
	name := strings.ReplaceAll(worktree, "-", "_")

	return Allocation{
		Slot:        slot,
		Schema:      "m2_" + name,
		IndexPrefix: worktree,
		CacheDB:     slot * databasesPerWorktree,
		PageCacheDB: slot*databasesPerWorktree + 1,
		SessionDB:   slot*databasesPerWorktree + 2,
		VHost:       worktree,
	}
}

// ErrNoSlots is returned when a project has run out of them.
type ErrNoSlots struct{ Project string }

func (e ErrNoSlots) Error() string {
	return fmt.Sprintf("'%s' has no free slot left: %d worktrees is what %d Redis databases allow",
		e.Project, MaxSlots, RedisDatabases)
}
