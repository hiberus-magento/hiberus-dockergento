package core

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

//
// The table of Magento releases the tool has images for, grouped under the image bucket each one
// runs on: one column per bucket of requirements.json, the 2.3 line on the left and the 2.4 line on
// the right, and in each column the releases equivalent_versions.json sends to it.
//
// What compatibility.sh printed, character for character. Its orderings came from two tools and
// are kept as they were: the buckets in `sort -V` order, the releases in the order jq's sort_by
// gave them — a release's base compared as text, then its patch number, a release with no patch
// first.
//

const compatibilityColumn = 10

// CompatibilityTable is the table, starting with a blank line and ending with its last rule.
func CompatibilityTable(requirements map[string]map[string]string, equivalents map[string]string) string {
	var older, newer []string

	for bucket := range requirements {
		switch {
		case strings.HasPrefix(bucket, "2.3."):
			older = append(older, bucket)
		case strings.HasPrefix(bucket, "2.4."):
			newer = append(newer, bucket)
		}
	}

	byVersion := func(buckets []string) {
		sort.Slice(buckets, func(i, j int) bool {
			return !VersionAtLeast(buckets[i], buckets[j])
		})
	}

	byVersion(older)
	byVersion(newer)

	columns := make([][]string, 0, len(older)+len(newer))
	rows := 0

	for _, bucket := range append(append([]string{}, older...), newer...) {
		releases := releasesOf(bucket, equivalents)
		columns = append(columns, releases)
		rows = max(rows, len(releases))
	}

	olderWidth := sectionWidth(len(older))
	newerWidth := sectionWidth(len(newer))
	total := 2 + olderWidth + 4 + newerWidth + 2
	inner := total - 2

	rule := strings.Repeat("-", total)
	double := strings.Repeat("=", total)

	var table strings.Builder

	table.WriteString("\n" + double + "\n")
	table.WriteString("|" + centred("SUPPORTED MAGENTO VERSIONS", inner) + "|\n")
	table.WriteString(double + "\n")
	table.WriteString("| " + centred("2.3.x", olderWidth) + " || " + centred("2.4.x", newerWidth) + " |\n")
	table.WriteString(rule + "\n")

	for row := 0; row < rows; row++ {
		table.WriteString("| " + cells(columns[:len(older)], row) + " || " + cells(columns[len(older):], row) + " |\n")
	}

	table.WriteString(rule + "\n")

	return table.String()
}

// releasesOf are the releases that run on bucket, in jq's sort_by order.
func releasesOf(bucket string, equivalents map[string]string) []string {
	releases := []string{}

	for release, runsOn := range equivalents {
		if runsOn == bucket {
			releases = append(releases, release)
		}
	}

	key := func(release string) (string, int) {
		if base, patch, found := strings.Cut(release, "-p"); found && !strings.Contains(patch, "-p") {
			number, _ := strconv.Atoi(patch)

			return base, number
		}

		return release, -1
	}

	sort.Slice(releases, func(i, j int) bool {
		leftBase, leftPatch := key(releases[i])
		rightBase, rightPatch := key(releases[j])

		if leftBase != rightBase {
			return leftBase < rightBase
		}

		return leftPatch < rightPatch
	})

	return releases
}

// sectionWidth is how wide a line's columns are with the separators between them.
func sectionWidth(columns int) int {
	if columns == 0 {
		return 0
	}

	return columns*compatibilityColumn + (columns-1)*3
}

// centred puts text in the middle of width, the extra space on the right.
func centred(text string, width int) string {
	left := (width - len(text)) / 2

	return strings.Repeat(" ", max(left, 0)) + text + strings.Repeat(" ", max(width-left-len(text), 0))
}

// cells is one row across columns, each padded to the column width.
func cells(columns [][]string, row int) string {
	parts := make([]string, len(columns))

	for index, column := range columns {
		cell := ""
		if row < len(column) {
			cell = column[row]
		}

		parts[index] = fmt.Sprintf("%-*s", compatibilityColumn, cell)
	}

	return strings.Join(parts, " | ")
}
