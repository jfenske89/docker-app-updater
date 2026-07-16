// Package report turns a set of per-app update results into the
// notification message text, built directly from what actually happened
// rather than a second, independent scrape of Docker state.
package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jfenske89/docker-app-updater/internal/update"
)

type group struct {
	status update.Status
	title  string
}

// groups defines the sections that render, in display order. Unchanged is
// deliberately absent: nothing to say about it.
var groups = []group{
	{update.StatusUpdated, "Docker apps updated"},
	{update.StatusRecreated, "Recreated, image unchanged"},
	{update.StatusRestarted, "Restarted"},
	{update.StatusSkipped, "Skipped, no containers found"},
	{update.StatusError, "⚠ Errors"},
}

// Build renders results into notification message text. It returns "" if there
// is nothing worth reporting (every app was unchanged), even when label is
// set: a label with no content isn't worth sending on its own.
//
// label, if non-empty, is prepended as a bold header line so messages from
// multiple hosts can be told apart.
func Build(label string, results []update.Result) string {
	byStatus := make(map[update.Status][]update.Result)
	for _, r := range results {
		byStatus[r.Status] = append(byStatus[r.Status], r)
	}

	var sections []string
	for _, g := range groups {
		apps := byStatus[g.status]
		if len(apps) == 0 {
			continue
		}

		sort.Slice(apps, func(i, j int) bool { return apps[i].App.Name < apps[j].App.Name })

		var lines []string
		lines = append(lines, fmt.Sprintf("**%s (%d)**", g.title, len(apps)))
		for _, r := range apps {
			lines = append(lines, fmt.Sprintf("- %s", r.App.Name))
			if r.Err != nil {
				lines = append(lines, fmt.Sprintf("  `%s`", truncate(r.Err.Error(), 300)))
			}
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	if len(sections) == 0 {
		return ""
	}

	if label != "" {
		sections = append([]string{fmt.Sprintf("**%s**", label)}, sections...)
	}

	return strings.Join(sections, "\n\n")
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
