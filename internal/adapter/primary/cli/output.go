package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
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
func renderBuilds(w io.Writer, format string, builds []dto.Build) error {
	switch format {
	case "json":
		return renderJSON(w, builds)
	case "yaml":
		return renderYAML(w, builds)
	default:
		return renderBuildsTable(w, builds)
	}
}

func renderBuildsTable(w io.Writer, builds []dto.Build) error {
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
func renderBuildRequests(w io.Writer, format string, results []dto.BuildRequest) error {
	switch format {
	case "json":
		return renderJSON(w, results)
	case "yaml":
		return renderYAML(w, results)
	default:
		return renderBuildRequestsTable(w, results)
	}
}

func renderBuildRequestsTable(w io.Writer, results []dto.BuildRequest) error {
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

// renderCommits writes commits in the requested format.
func renderCommits(w io.Writer, format string, commits []forge.Commit) error {
	switch format {
	case "json":
		return renderJSON(w, commits)
	case "yaml":
		return renderYAML(w, commits)
	default:
		return renderCommitTable(w, commits)
	}
}

func renderCommitTable(w io.Writer, commits []forge.Commit) error {
	if len(commits) == 0 {
		fmt.Fprintln(w, "No commits found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PROJECT\tFORGE\tSHA\tAUTHOR\tDATE\tSTATUS\tLINK\tMESSAGE")
	for _, c := range commits {
		msg := c.Message
		if idx := strings.Index(msg, "\n"); idx != -1 {
			msg = msg[:idx]
		}
		if len(msg) > 60 {
			msg = msg[:57] + "..."
		}
		msg = strings.ReplaceAll(msg, "\t", " ")

		date := ""
		if !c.Date.IsZero() {
			date = c.Date.Format("2006-01-02 15:04")
		}
		sha := c.SHA
		if len(sha) > 10 {
			sha = sha[:10]
		}

		status := ""
		link := ""
		if c.MergeRequest != nil {
			status = c.MergeRequest.State.String()
			link = c.MergeRequest.URL
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			c.Repo,
			c.Forge,
			sha,
			c.Author,
			date,
			status,
			link,
			msg,
		)
	}
	return tw.Flush()
}

// renderBugSyncResult writes bug sync results in the requested format.
func renderBugSyncResult(w io.Writer, format string, result *dto.BugSyncResult, dryRun bool) error {
	switch format {
	case "json":
		return renderJSON(w, result)
	case "yaml":
		return renderYAML(w, result)
	default:
		return renderBugSyncTable(w, result, dryRun)
	}
}

func renderBugSyncTable(w io.Writer, result *dto.BugSyncResult, dryRun bool) error {
	if len(result.Actions) == 0 {
		fmt.Fprintln(w, "No bugs to sync.")
		return nil
	}
	prefix := ""
	if dryRun {
		prefix = "would "
	}
	for _, a := range result.Actions {
		switch a.ActionType {
		case dto.BugSyncActionStatusUpdate:
			fmt.Fprintf(w, "%supdate: Bug #%s task %q %s → %s\n", prefix, a.BugID, a.TaskTitle, a.OldStatus, a.NewStatus)
		case dto.BugSyncActionSeriesAssignment:
			fmt.Fprintf(w, "%sassign: Bug #%s to series %q on project %q\n", prefix, a.BugID, a.Series, a.Project)
		case dto.BugSyncActionAddProjectTask:
			fmt.Fprintf(w, "%sadd: Bug #%s task on project %q\n", prefix, a.BugID, a.Project)
		}
	}
	return nil
}

// renderProjectSyncResult writes project sync results in the requested format.
func renderProjectSyncResult(w io.Writer, format string, result *dto.ProjectSyncResult, dryRun bool) error {
	switch format {
	case "json":
		return renderJSON(w, result)
	case "yaml":
		return renderYAML(w, result)
	default:
		return renderProjectSyncTable(w, result, dryRun)
	}
}

func renderProjectSyncTable(w io.Writer, result *dto.ProjectSyncResult, dryRun bool) error {
	if len(result.Actions) == 0 {
		fmt.Fprintln(w, "No changes needed.")
		return nil
	}
	prefix := ""
	if dryRun {
		prefix = "would "
	}
	for _, a := range result.Actions {
		switch a.ActionType {
		case dto.ProjectSyncActionCreateSeries:
			fmt.Fprintf(w, "%screate: series %q on project %q\n", prefix, a.Series, a.Project)
		case dto.ProjectSyncActionSetDevFocus:
			fmt.Fprintf(w, "%sset: development focus to %q on project %q\n", prefix, a.Series, a.Project)
		case dto.ProjectSyncActionDevFocusUnchanged:
			fmt.Fprintf(w, "unchanged: development focus already %q on project %q\n", a.Series, a.Project)
		}
	}
	return nil
}

// renderStringList writes a list of strings in the requested format.
func renderStringList(w io.Writer, format string, items []string) error {
	switch format {
	case "json":
		return renderJSON(w, items)
	case "yaml":
		return renderYAML(w, items)
	default:
		for _, item := range items {
			fmt.Fprintln(w, item)
		}
		return nil
	}
}

type cacheEntry struct {
	Name string `json:"name" yaml:"name"`
	Size string `json:"size" yaml:"size"`
}

type cacheFullStatus struct {
	Git struct {
		Directory string       `json:"directory" yaml:"directory"`
		Repos     []cacheEntry `json:"repos" yaml:"repos"`
	} `json:"git" yaml:"git"`
	Packages struct {
		Directory string            `json:"directory,omitempty" yaml:"directory,omitempty"`
		Error     string            `json:"error,omitempty" yaml:"error,omitempty"`
		Sources   []dto.CacheStatus `json:"sources" yaml:"sources"`
	} `json:"packages" yaml:"packages"`
	Upstream struct {
		Directory string       `json:"directory" yaml:"directory"`
		Repos     []cacheEntry `json:"repos" yaml:"repos"`
	} `json:"upstream" yaml:"upstream"`
	Bugs struct {
		Directory string               `json:"directory" yaml:"directory"`
		Entries   []dto.BugCacheStatus `json:"entries" yaml:"entries"`
		Error     string               `json:"error,omitempty" yaml:"error,omitempty"`
	} `json:"bugs" yaml:"bugs"`
}

func renderCacheFullStatus(w io.Writer, format string, status *cacheFullStatus) error {
	switch format {
	case "json":
		return renderJSON(w, status)
	case "yaml":
		return renderYAML(w, status)
	default:
		return renderCacheFullStatusTable(w, status)
	}
}

func renderCacheFullStatusTable(w io.Writer, status *cacheFullStatus) error {
	fmt.Fprintln(w, "=== Git Repos ===")
	if len(status.Git.Repos) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		fmt.Fprintf(w, "directory: %s\n", status.Git.Directory)
		for _, r := range status.Git.Repos {
			fmt.Fprintf(w, "  %s  (%s)\n", r.Name, r.Size)
		}
	}

	fmt.Fprintln(w, "\n=== Packages Index ===")
	if status.Packages.Error != "" {
		fmt.Fprintf(w, "  (unavailable: %s)\n", status.Packages.Error)
	} else if len(status.Packages.Sources) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		fmt.Fprintf(w, "directory: %s\n", status.Packages.Directory)
		if err := renderCacheStatusTable(w, status.Packages.Sources); err != nil {
			return err
		}
	}

	fmt.Fprintln(w, "\n=== Upstream Repos ===")
	if len(status.Upstream.Repos) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		fmt.Fprintf(w, "directory: %s\n", status.Upstream.Directory)
		for _, r := range status.Upstream.Repos {
			fmt.Fprintf(w, "  %s  (%s)\n", r.Name, r.Size)
		}
	}

	fmt.Fprintln(w, "\n=== Bugs ===")
	if status.Bugs.Error != "" {
		fmt.Fprintf(w, "  (unavailable: %s)\n", status.Bugs.Error)
	} else if len(status.Bugs.Entries) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		fmt.Fprintf(w, "directory: %s\n", status.Bugs.Directory)
		for _, e := range status.Bugs.Entries {
			syncStr := "never"
			if !e.LastSync.IsZero() {
				syncStr = e.LastSync.Format("2006-01-02 15:04:05")
			}
			fmt.Fprintf(w, "  %s/%s: %d bugs, %d tasks (synced: %s)\n", e.ForgeType, e.Project, e.BugCount, e.TaskCount, syncStr)
		}
	}

	return nil
}
