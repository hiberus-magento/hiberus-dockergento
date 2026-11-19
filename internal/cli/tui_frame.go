package cli

import (
	"fmt"
	"strings"
)

//
// From composed lines to a frame, and from a frame to bytes; ported from console/tasks/tui_frame.sh.
//
// A frame is exactly as many lines as the terminal has rows, each written at its own position and
// clearing what was to its right, so nothing is ever erased first and a shorter screen cannot
// leave anything behind.
//

// compose turns the JSON into lines cut to the terminal width. It runs when data arrives and when
// the width changes, never on a keystroke.
func (d *dashboard) compose() {
	d.composedCols = d.cols

	d.fleetHeader = dashFleetHeader(d.cols - 4)
	d.fleetRows = dashRowsOf(d.environments, d.cols-4)
	d.doctorRows = dashDoctorLines(d.doctorJSON, d.cols-2)

	if d.detailJSON != "" {
		d.detailRows = dashDetailLines(d.detailJSON, d.cols-4)
	} else {
		d.detailRows = nil
	}
}

// frame is every line of the screen, in order.
func (d *dashboard) frame() []string {
	bold, red, green, yellow, white, off := code("\033[1m"), code("\033[0;31m"), code("\033[0;32m"),
		code("\033[0;33m"), code("\033[1;37m"), code(reset)

	lines := []string{}

	if d.view == "fleet" {
		lines = d.fleetFrame(lines, bold, red, green, yellow, white, off)
	} else {
		lines = d.detailFrame(lines, bold, white, off)
	}

	// Whatever the view did, the two bottom rows belong to the status and the keys
	if len(lines) > d.rows-2 && d.rows >= 2 {
		lines = lines[:d.rows-2]
	}

	for len(lines) < d.rows-2 {
		lines = append(lines, "")
	}

	state := d.message
	if state == "" && d.loadedAt != "" {
		state = "data from " + d.loadedAt
	}

	return append(lines, dashCut(state, d.cols-2), bold+dashCut(d.footerKeys(), d.cols-2)+off)
}

func (d *dashboard) fleetFrame(lines []string, bold, red, green, yellow, white, off string) []string {
	lines = append(lines, bold+"Dockergento — environments on this machine"+off, "")

	for _, row := range d.doctorRows {
		if strings.HasPrefix(row, "ERROR") {
			lines = append(lines, "  "+red+row+off)
		} else {
			lines = append(lines, "  "+yellow+row+off)
		}
	}

	if len(d.doctorRows) > 0 {
		lines = append(lines, "")
	}

	lines = append(lines, "  "+bold+d.fleetHeader+off)

	// An empty fleet before the first read is not an empty fleet
	if len(d.fleetRows) == 0 {
		lines = append(lines, "")

		if d.loadedAt == "" {
			return append(lines, "  Reading the environments on this machine…")
		}

		return append(lines, "  No environments found on this machine.",
			"  Create one with "+green+"hm setup"+off+" inside a Magento project.")
	}

	// Two rows at the bottom for the status and the keys, and one for the line saying which part
	// of the list is on screen when it does not all fit
	room := d.rows - len(lines) - 2
	visible := room

	if len(d.fleetRows) > room {
		visible = room - 1
	}

	if visible < 1 {
		visible = 1
	}

	if d.selected < d.firstVisible {
		d.firstVisible = d.selected
	} else if d.selected >= d.firstVisible+visible {
		d.firstVisible = d.selected - visible + 1
	}

	if d.firstVisible < 0 {
		d.firstVisible = 0
	}

	last := min(d.firstVisible+visible, len(d.fleetRows))

	for at := d.firstVisible; at < last; at++ {
		if at == d.selected {
			lines = append(lines, green+"> "+d.fleetRows[at]+off)
		} else {
			lines = append(lines, "  "+d.fleetRows[at])
		}
	}

	if last < len(d.fleetRows) || d.firstVisible > 0 {
		lines = append(lines, fmt.Sprintf("  %sshowing %d-%d of %d%s", white, d.firstVisible+1, last,
			len(d.fleetRows), off))
	}

	return lines
}

func (d *dashboard) detailFrame(lines []string, bold, white, off string) []string {
	lines = append(lines, bold+d.detailName+off, "")

	if len(d.detailRows) == 0 {
		return append(lines, "  Nothing to show.")
	}

	// What does not fit scrolls, and says so, rather than being cut off in silence
	room := d.rows - len(lines) - 2
	visible := room

	if len(d.detailRows) > room {
		visible = room - 1
	}

	if visible < 1 {
		visible = 1
	}

	most := max(len(d.detailRows)-visible, 0)
	d.detailOffset = max(min(d.detailOffset, most), 0)

	last := min(d.detailOffset+visible, len(d.detailRows))

	for at := d.detailOffset; at < last; at++ {
		lines = append(lines, "  "+d.detailRows[at])
	}

	if len(d.detailRows) > room {
		lines = append(lines, fmt.Sprintf("  %sshowing %d-%d of %d   j/k to scroll%s", white,
			d.detailOffset+1, last, len(d.detailRows), off))
	}

	return lines
}

// footerKeys are the keys available right now, in whichever form fits. ASCII only: it is the one
// line that must always be readable.
func (d *dashboard) footerKeys() string {
	long := "esc back   s start   x stop   r restart   l logs   o browser   ? keys   q quit"
	short := "esc back   s/x/r start/stop/restart   ? keys   q quit"

	if d.view == "fleet" {
		long = "j/k move   enter open   s start   x stop   r restart   l logs   o browser   g refresh   ? keys   q quit"
		short = "j/k move   enter open   s/x/r start/stop/restart   ? keys   q quit"
	}

	if len(long) <= d.cols-2 {
		return long
	}

	return short
}

// frameBytes is the whole frame as one write: each line at its row, clearing to its right.
func frameBytes(lines []string) string {
	built := strings.Builder{}

	for at, line := range lines {
		fmt.Fprintf(&built, "\033[%d;1H%s\033[K", at+1, line)
	}

	return built.String()
}

// paint draws one frame, held by the terminal while it is written and presented in one go.
func (d *dashboard) paint() {
	if d.cols != d.composedCols {
		d.compose()
	}

	fmt.Fprint(d.out, "\033[?2026h"+frameBytes(d.frame())+"\033[J"+"\033[?2026l")
}
