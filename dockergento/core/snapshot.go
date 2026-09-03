package core

import "fmt"

//
// A named copy of a project's database, as a dump in a file.
//
// The other half of `db` freezes the data directory instead: that one is available again in
// seconds and is not portable at all. This one is the copy that outlives the environment — it
// survives `down -v`, it can be read by a database two major versions later, and it is small
// enough to keep several of.
//
// The copies live outside the project, beside the cache, for two reasons that matter more than
// tidiness: `config/docker` is committed, so a copy there would end up in somebody's commit, and a
// copy stored inside the environment would not survive the one moment it is needed.
//

// Snapshot is one of them.
type Snapshot struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Size  string `json:"size"`
	Bytes int64  `json:"-"`

	// TakenAt is when the file was written, to the minute, which is the resolution somebody
	// choosing between two copies actually reads.
	TakenAt string `json:"taken_at"`
}

// ValidSnapshotName reports whether a name can be used. It becomes a file name, which is the whole
// rule: what cannot be one is refused rather than quietly turned into something else.
func ValidSnapshotName(name string) bool { return ValidTemplateName(name) }

// SnapshotSize is the size of a file as `du` reports it.
//
// Disk usage rather than the length of the file, because that is what the shell implementation
// showed and it is the more useful of the two: a dump takes the blocks it takes.
func SnapshotSize(bytes int64) string {
	units := []string{"B", "K", "M", "G", "T"}

	value := float64(bytes)
	at := 0

	for value >= 1024 && at < len(units)-1 {
		value /= 1024
		at++
	}

	// Rounded up to the precision shown, which is what `du -h` does: a copy is never reported as
	// smaller than it is
	if at == 0 {
		return fmt.Sprintf("%d%s", int64(value), units[at])
	}

	if value < 10 {
		return fmt.Sprintf("%.1f%s", ceil(value, 10), units[at])
	}

	return fmt.Sprintf("%d%s", int64(ceil(value, 1)), units[at])
}

// ceil rounds up to a given number of steps per unit: ten for one decimal, one for none.
func ceil(value float64, steps float64) float64 {
	scaled := value * steps
	rounded := float64(int64(scaled))

	if scaled > rounded {
		rounded++
	}

	return rounded / steps
}
