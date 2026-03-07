package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

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

// renderOperationJobs writes operations in the requested format.
func renderOperationJobs(w io.Writer, format string, jobs []dto.OperationJob) error {
	switch format {
	case "json":
		return renderJSON(w, jobs)
	case "yaml":
		return renderYAML(w, jobs)
	default:
		return renderOperationJobsTable(w, jobs)
	}
}

func renderOperationJobsTable(w io.Writer, jobs []dto.OperationJob) error {
	if len(jobs) == 0 {
		fmt.Fprintln(w, "No operations found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tKIND\tSTATE\tCREATED\tSUMMARY")
	for _, job := range jobs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			job.ID,
			job.Kind,
			job.State,
			formatTimestamp(job.CreatedAt),
			job.Summary,
		)
	}
	return tw.Flush()
}

// renderOperationJob writes a single operation in the requested format.
func renderOperationJob(w io.Writer, format string, job *dto.OperationJob) error {
	switch format {
	case "json":
		return renderJSON(w, job)
	case "yaml":
		return renderYAML(w, job)
	default:
		return renderOperationJobTable(w, job)
	}
}

func renderOperationJobTable(w io.Writer, job *dto.OperationJob) error {
	if job == nil {
		fmt.Fprintln(w, "No operation found.")
		return nil
	}

	fmt.Fprintf(w, "ID:            %s\n", job.ID)
	fmt.Fprintf(w, "Kind:          %s\n", job.Kind)
	fmt.Fprintf(w, "State:         %s\n", job.State)
	fmt.Fprintf(w, "Created:       %s\n", formatTimestamp(job.CreatedAt))
	if !job.StartedAt.IsZero() {
		fmt.Fprintf(w, "Started:       %s\n", formatTimestamp(job.StartedAt))
	}
	if !job.FinishedAt.IsZero() {
		fmt.Fprintf(w, "Finished:      %s\n", formatTimestamp(job.FinishedAt))
	}
	fmt.Fprintf(w, "Cancellable:   %t\n", job.Cancellable)
	if job.Summary != "" {
		fmt.Fprintf(w, "Summary:       %s\n", job.Summary)
	}
	if job.Error != "" {
		fmt.Fprintf(w, "Error:         %s\n", job.Error)
	}
	if job.Progress != nil {
		fmt.Fprintf(w, "Progress:      %s\n", formatOperationProgress(job.Progress))
	}
	if len(job.Attributes) > 0 {
		fmt.Fprintln(w, "Attributes:")
		for key, value := range job.Attributes {
			fmt.Fprintf(w, "  %s=%s\n", key, value)
		}
	}
	return nil
}

// renderOperationEvents writes operation events in the requested format.
func renderOperationEvents(w io.Writer, format string, events []dto.OperationEvent) error {
	switch format {
	case "json":
		return renderJSON(w, events)
	case "yaml":
		return renderYAML(w, events)
	default:
		return renderOperationEventsTable(w, events)
	}
}

func renderOperationEventsTable(w io.Writer, events []dto.OperationEvent) error {
	if len(events) == 0 {
		fmt.Fprintln(w, "No operation events found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "TIME\tTYPE\tMESSAGE\tERROR\tPROGRESS")
	for _, event := range events {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			formatTimestamp(event.Time),
			event.Type,
			event.Message,
			event.Error,
			formatOperationProgress(event.Progress),
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

// renderReleaseList writes release list rows in the requested format.
func renderReleaseList(w io.Writer, format string, releases []dto.ReleaseListEntry) error {
	switch format {
	case "json":
		return renderJSON(w, releases)
	case "yaml":
		return renderYAML(w, releases)
	default:
		if len(releases) == 0 {
			fmt.Fprintln(w, "No releases found.")
			return nil
		}
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "PROJECT\tTYPE\tNAME\tTRACK\tRISK\tBRANCH\tTARGETS\tRESOURCES\tUPDATED")
		for _, release := range releases {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				release.Project,
				release.ArtifactType.String(),
				release.Name,
				release.Track,
				release.Risk,
				emptyDash(release.Branch),
				formatReleaseTargets(release.Targets),
				formatReleaseResources(release.Resources),
				formatTimestamp(release.UpdatedAt),
			)
		}
		return tw.Flush()
	}
}

// renderReleaseShow writes one release matrix in the requested format.
func renderReleaseShow(w io.Writer, format string, release *dto.ReleaseShowResult) error {
	switch format {
	case "json":
		return renderJSON(w, release)
	case "yaml":
		return renderYAML(w, release)
	default:
		fmt.Fprintf(w, "Project:       %s\n", release.Project)
		fmt.Fprintf(w, "Type:          %s\n", release.ArtifactType.String())
		fmt.Fprintf(w, "Name:          %s\n", release.Name)
		fmt.Fprintf(w, "Updated:       %s\n", formatTimestamp(release.UpdatedAt))
		if len(release.Tracks) > 0 {
			fmt.Fprintf(w, "Tracks:        %s\n", strings.Join(release.Tracks, ", "))
		}
		fmt.Fprintln(w)
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "TRACK\tRISK\tBRANCH\tTARGETS\tRESOURCES")
		for _, channel := range release.Channels {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
				channel.Track,
				channel.Risk,
				emptyDash(channel.Branch),
				formatReleaseTargets(channel.Targets),
				formatReleaseResources(channel.Resources),
			)
		}
		return tw.Flush()
	}
}

func formatReleaseTargets(targets []dto.ReleaseTargetSnapshot) string {
	if len(targets) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(targets))
	for _, target := range targets {
		label := target.Architecture
		if label == "" {
			label = target.Base.Architecture
		}
		if label == "" {
			label = "default"
		}
		revision := ""
		if target.Revision > 0 {
			revision = fmt.Sprintf("r%d", target.Revision)
		}
		version := target.Version
		if version != "" && revision != "" {
			parts = append(parts, fmt.Sprintf("%s:%s/%s", label, revision, version))
		} else if revision != "" {
			parts = append(parts, fmt.Sprintf("%s:%s", label, revision))
		} else if version != "" {
			parts = append(parts, fmt.Sprintf("%s:%s", label, version))
		} else {
			parts = append(parts, label)
		}
	}
	return strings.Join(parts, ", ")
}

func formatReleaseResources(resources []dto.ReleaseResourceSnapshot) string {
	if len(resources) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(resources))
	for _, resource := range resources {
		if resource.Revision > 0 {
			parts = append(parts, fmt.Sprintf("%s:r%d", resource.Name, resource.Revision))
		} else {
			parts = append(parts, resource.Name)
		}
	}
	return strings.Join(parts, ", ")
}

func emptyDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
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
	Excuses struct {
		Directory string                   `json:"directory" yaml:"directory"`
		Entries   []dto.ExcusesCacheStatus `json:"entries" yaml:"entries"`
		Error     string                   `json:"error,omitempty" yaml:"error,omitempty"`
	} `json:"excuses" yaml:"excuses"`
	Releases struct {
		Directory string                   `json:"directory" yaml:"directory"`
		Entries   []dto.ReleaseCacheStatus `json:"entries" yaml:"entries"`
		Error     string                   `json:"error,omitempty" yaml:"error,omitempty"`
	} `json:"releases" yaml:"releases"`
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

	fmt.Fprintln(w, "\n=== Excuses ===")
	if status.Excuses.Error != "" {
		fmt.Fprintf(w, "  (unavailable: %s)\n", status.Excuses.Error)
	} else if len(status.Excuses.Entries) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		fmt.Fprintf(w, "directory: %s\n", status.Excuses.Directory)
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "  TRACKER\tENTRIES\tLAST UPDATED\tSIZE")
		for _, entry := range status.Excuses.Entries {
			lastUpdated := "never"
			if !entry.LastUpdated.IsZero() {
				lastUpdated = entry.LastUpdated.Format("2006-01-02 15:04:05")
			}
			fmt.Fprintf(tw, "  %s\t%d\t%s\t%s\n", entry.Tracker, entry.EntryCount, lastUpdated, formatSize(entry.DiskSize))
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	fmt.Fprintln(w, "\n=== Releases ===")
	if status.Releases.Error != "" {
		fmt.Fprintf(w, "  (unavailable: %s)\n", status.Releases.Error)
	} else if len(status.Releases.Entries) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		fmt.Fprintf(w, "directory: %s\n", status.Releases.Directory)
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "  PROJECT\tTYPE\tNAME\tTRACKS\tCHANNELS\tLAST UPDATED")
		for _, entry := range status.Releases.Entries {
			lastUpdated := "never"
			if !entry.LastUpdated.IsZero() {
				lastUpdated = entry.LastUpdated.Format("2006-01-02 15:04:05")
			}
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%d\t%d\t%s\n",
				entry.Project,
				entry.ArtifactType.String(),
				entry.Name,
				entry.TrackCount,
				entry.ChannelCount,
				lastUpdated,
			)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	return nil
}

func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func formatOperationProgress(progress *dto.OperationProgress) string {
	if progress == nil {
		return ""
	}
	parts := []string{}
	if progress.Phase != "" {
		parts = append(parts, progress.Phase)
	}
	if progress.Message != "" {
		parts = append(parts, progress.Message)
	}
	if progress.Indeterminate {
		parts = append(parts, "indeterminate")
	} else if progress.Total > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d", progress.Current, progress.Total))
	}
	return strings.Join(parts, " | ")
}
