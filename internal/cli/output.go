package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
	"gopkg.in/yaml.v3"
)

// renderMergeRequests writes merge requests in the requested format.
func renderMergeRequests(w io.Writer, format string, mrs []forge.MergeRequest) error {
	switch format {
	case "json":
		return renderJSON(w, mrs)
	case "yaml":
		return renderYAML(w, mrs)
	default:
		return renderTable(w, mrs)
	}
}

func renderJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func renderYAML(w io.Writer, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// renderBugTasks writes bug tasks in the requested format.
func renderBugTasks(w io.Writer, format string, tasks []forge.BugTask) error {
	switch format {
	case "json":
		return renderJSON(w, tasks)
	case "yaml":
		return renderYAML(w, tasks)
	default:
		return renderBugTable(w, tasks)
	}
}

func renderBugTable(w io.Writer, tasks []forge.BugTask) error {
	if len(tasks) == 0 {
		fmt.Fprintln(w, "No bug tasks found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PROJECT\tID\tSTATUS\tIMPORTANCE\tASSIGNEE\tTITLE\tURL")
	for _, t := range tasks {
		title := t.Title
		if len(title) > 60 {
			title = title[:57] + "..."
		}
		title = strings.ReplaceAll(title, "\t", " ")

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			t.Project,
			t.BugID,
			t.Status,
			t.Importance,
			t.Assignee,
			title,
			t.URL,
		)
	}
	return tw.Flush()
}

func renderTable(w io.Writer, mrs []forge.MergeRequest) error {
	if len(mrs) == 0 {
		fmt.Fprintln(w, "No merge requests found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PROJECT\tFORGE\tID\tSTATE\tAUTHOR\tTITLE\tURL")
	for _, mr := range mrs {
		title := mr.Title
		if len(title) > 60 {
			title = title[:57] + "..."
		}
		title = strings.ReplaceAll(title, "\t", " ")

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			mr.Repo,
			mr.Forge,
			mr.ID,
			mr.State,
			mr.Author,
			title,
			mr.URL,
		)
	}
	return tw.Flush()
}
