package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/port"
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

// renderMergeRequestDetail writes a single merge request in the requested format.
func renderMergeRequestDetail(w io.Writer, format string, mr *forge.MergeRequest) error {
	switch format {
	case "json":
		return renderJSON(w, mr)
	case "yaml":
		return renderYAML(w, mr)
	default:
		return renderMergeRequestDetailTable(w, mr)
	}
}

func renderMergeRequestDetailTable(w io.Writer, mr *forge.MergeRequest) error {
	fmt.Fprintf(w, "Project:       %s\n", mr.Repo)
	fmt.Fprintf(w, "Forge:         %s\n", mr.Forge)
	fmt.Fprintf(w, "ID:            %s\n", mr.ID)
	fmt.Fprintf(w, "Title:         %s\n", mr.Title)
	fmt.Fprintf(w, "Author:        %s\n", mr.Author)
	fmt.Fprintf(w, "State:         %s\n", mr.State)
	fmt.Fprintf(w, "Review:        %s\n", mr.ReviewState)
	fmt.Fprintf(w, "Source:        %s\n", mr.SourceBranch)
	fmt.Fprintf(w, "Target:        %s\n", mr.TargetBranch)
	fmt.Fprintf(w, "URL:           %s\n", mr.URL)
	if !mr.CreatedAt.IsZero() {
		fmt.Fprintf(w, "Created:       %s\n", mr.CreatedAt.Format("2006-01-02 15:04"))
	}
	if !mr.UpdatedAt.IsZero() {
		fmt.Fprintf(w, "Updated:       %s\n", mr.UpdatedAt.Format("2006-01-02 15:04"))
	}
	if len(mr.Checks) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Checks:")
		for _, c := range mr.Checks {
			fmt.Fprintf(w, "  %s  %s  %s\n", c.State, c.Name, c.URL)
		}
	}
	if mr.Description != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Description:")
		fmt.Fprintln(w, mr.Description)
	}
	return nil
}

// renderBugDetail writes a single bug in the requested format.
func renderBugDetail(w io.Writer, format string, b *forge.Bug) error {
	switch format {
	case "json":
		return renderJSON(w, b)
	case "yaml":
		return renderYAML(w, b)
	default:
		return renderBugDetailTable(w, b)
	}
}

func renderBugDetailTable(w io.Writer, b *forge.Bug) error {
	fmt.Fprintf(w, "Bug:           #%s\n", b.ID)
	fmt.Fprintf(w, "Title:         %s\n", b.Title)
	fmt.Fprintf(w, "Owner:         %s\n", b.Owner)
	fmt.Fprintf(w, "URL:           %s\n", b.URL)
	if len(b.Tags) > 0 {
		fmt.Fprintf(w, "Tags:          %s\n", strings.Join(b.Tags, ", "))
	}
	if !b.CreatedAt.IsZero() {
		fmt.Fprintf(w, "Created:       %s\n", b.CreatedAt.Format("2006-01-02 15:04"))
	}
	if !b.UpdatedAt.IsZero() {
		fmt.Fprintf(w, "Updated:       %s\n", b.UpdatedAt.Format("2006-01-02 15:04"))
	}
	if b.Description != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Description:")
		fmt.Fprintln(w, b.Description)
	}
	if len(b.Tasks) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Tasks:")
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "  TARGET\tSTATUS\tIMPORTANCE\tASSIGNEE\tURL")
		for _, t := range b.Tasks {
			target := t.Title
			// LP bug task titles are like "Bug #12345 in projectname: title"
			// Use the full title as the target identifier
			if len(target) > 50 {
				target = target[:47] + "..."
			}
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\n",
				target,
				t.Status,
				t.Importance,
				t.Assignee,
				t.URL,
			)
		}
		return tw.Flush()
	}
	return nil
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

// renderBuilds writes builds in the requested format.
func renderBuilds(w io.Writer, format string, builds []port.Build) error {
	switch format {
	case "json":
		return renderJSON(w, builds)
	case "yaml":
		return renderYAML(w, builds)
	default:
		return renderBuildsTable(w, builds)
	}
}

func renderBuildsTable(w io.Writer, builds []port.Build) error {
	if len(builds) == 0 {
		fmt.Fprintln(w, "No builds found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PROJECT\tRECIPE\tARCH\tSTATE\tCREATED\tURL")
	for _, b := range builds {
		created := ""
		if !b.CreatedAt.IsZero() {
			created = b.CreatedAt.Format("2006-01-02 15:04")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			b.Project,
			b.Recipe,
			b.Arch,
			b.State,
			created,
			b.WebLink,
		)
	}
	return tw.Flush()
}

// renderBuildRequests writes build request results in the requested format.
func renderBuildRequests(w io.Writer, format string, results []port.BuildRequest) error {
	switch format {
	case "json":
		return renderJSON(w, results)
	case "yaml":
		return renderYAML(w, results)
	default:
		return renderBuildRequestsTable(w, results)
	}
}

func renderBuildRequestsTable(w io.Writer, results []port.BuildRequest) error {
	if len(results) == 0 {
		fmt.Fprintln(w, "No build requests found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "STATUS\tERROR\tURL")
	for _, r := range results {
		fmt.Fprintf(tw, "%s\t%s\t%s\n",
			r.Status,
			r.ErrorMessage,
			r.WebLink,
		)
	}
	return tw.Flush()
}

// renderRecipes writes recipes in the requested format.
func renderRecipes(w io.Writer, format string, recipes []port.Recipe) error {
	switch format {
	case "json":
		return renderJSON(w, recipes)
	case "yaml":
		return renderYAML(w, recipes)
	default:
		return renderRecipesTable(w, recipes)
	}
}

func renderRecipesTable(w io.Writer, recipes []port.Recipe) error {
	if len(recipes) == 0 {
		fmt.Fprintln(w, "No recipes found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tOWNER\tPROJECT\tAUTO_BUILD\tCREATED")
	for _, r := range recipes {
		created := ""
		if !r.CreatedAt.IsZero() {
			created = r.CreatedAt.Format("2006-01-02 15:04")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%v\t%s\n",
			r.Name,
			r.ArtifactType,
			r.Owner,
			r.Project,
			r.AutoBuild,
			created,
		)
	}
	return tw.Flush()
}
