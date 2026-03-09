// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andygrunwald/go-gerrit"
)

// GerritForge implements Forge for Gerrit instances.
type GerritForge struct {
	client  *gerrit.Client
	baseURL string // e.g. "https://review.opendev.org"
}

// NewGerritForge creates a new Gerrit forge client.
// baseURL should be the Gerrit instance URL (e.g. "https://review.opendev.org").
func NewGerritForge(client *gerrit.Client, baseURL string) *GerritForge {
	return &GerritForge{client: client, baseURL: baseURL}
}

func (g *GerritForge) Type() ForgeType {
	return ForgeGerrit
}

func (g *GerritForge) ListMergeRequests(ctx context.Context, repo string, opts ListMergeRequestsOpts) ([]MergeRequest, error) {
	query := fmt.Sprintf("project:%s", repo)
	query += " " + gerritStatusQuery(opts.State)
	if opts.Author != "" {
		query += fmt.Sprintf(" owner:%s", opts.Author)
	}

	changes, _, err := g.client.Changes.QueryChanges(ctx, &gerrit.QueryChangeOptions{
		QueryOptions: gerrit.QueryOptions{
			Query: []string{query},
			Limit: 100,
		},
		ChangeOptions: gerrit.ChangeOptions{
			AdditionalFields: []string{"LABELS", "CURRENT_REVISION"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("querying Gerrit changes for %s: %w", repo, err)
	}

	if changes == nil {
		return nil, nil
	}

	var result []MergeRequest
	for _, c := range *changes {
		result = append(result, g.changeToMergeRequest(repo, &c))
	}

	return result, nil
}

func (g *GerritForge) GetMergeRequest(ctx context.Context, repo string, id string) (*MergeRequest, error) {
	change, _, err := g.client.Changes.GetChangeDetail(ctx, id, &gerrit.ChangeOptions{
		AdditionalFields: []string{"LABELS", "CURRENT_REVISION", "DETAILED_LABELS"},
	})
	if err != nil {
		return nil, fmt.Errorf("getting Gerrit change %s: %w", id, err)
	}

	mr := g.changeToMergeRequest(repo, change)
	revisionID := change.CurrentRevision
	if revisionID == "" && len(change.Revisions) == 1 {
		for sha := range change.Revisions {
			revisionID = sha
		}
	}
	if comments, err := g.listChangeComments(ctx, id, change); err == nil {
		mr.Comments = comments
	}
	if revisionID != "" {
		if files, err := g.listRevisionFiles(ctx, id, revisionID); err == nil {
			mr.Files = files
		}
		if diffText, err := g.getRevisionPatch(ctx, id, revisionID); err == nil {
			mr.DiffText = diffText
		}
	}
	return &mr, nil
}

func (g *GerritForge) ListCommits(ctx context.Context, repo string, opts ListCommitsOpts) ([]Commit, error) {
	// Gerrit doesn't have a direct commit listing API.
	// Commits are accessed through changes/patchsets.
	// We can list merged changes and extract commit info.
	query := fmt.Sprintf("project:%s status:merged", repo)
	if opts.Since != nil {
		query += fmt.Sprintf(" after:%s", opts.Since.Format("2006-01-02"))
	}
	if opts.Author != "" {
		query += fmt.Sprintf(" owner:%s", opts.Author)
	}

	changes, _, err := g.client.Changes.QueryChanges(ctx, &gerrit.QueryChangeOptions{
		QueryOptions: gerrit.QueryOptions{
			Query: []string{query},
			Limit: 100,
		},
		ChangeOptions: gerrit.ChangeOptions{
			AdditionalFields: []string{"CURRENT_REVISION"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("listing Gerrit commits for %s: %w", repo, err)
	}

	if changes == nil {
		return nil, nil
	}

	var result []Commit
	for _, c := range *changes {
		result = append(result, g.changeToCommit(repo, &c))
	}

	return result, nil
}

func (g *GerritForge) changeToMergeRequest(repo string, c *gerrit.ChangeInfo) MergeRequest {
	mr := MergeRequest{
		Forge:        ForgeGerrit,
		Repo:         repo,
		ID:           strconv.Itoa(c.Number),
		Title:        c.Subject,
		TargetBranch: c.Branch,
		URL:          g.changeURL(c),
		CreatedAt:    c.Created.Time,
		UpdatedAt:    c.Updated.Time,
	}

	mr.Author = gerritOwnerName(&c.Owner)
	mr.State = gerritMergeState(c)
	mr.ReviewState = gerritReviewState(c)

	return mr
}

func (g *GerritForge) changeToCommit(repo string, c *gerrit.ChangeInfo) Commit {
	commit := Commit{
		Forge:   ForgeGerrit,
		Repo:    repo,
		SHA:     c.CurrentRevision,
		Message: c.Subject,
		URL:     g.changeURL(c),
		Date:    c.Updated.Time,
	}
	commit.Author = gerritOwnerName(&c.Owner)
	commit.BugRefs = ExtractBugRefs(c.Subject)
	return commit
}

func (g *GerritForge) changeURL(c *gerrit.ChangeInfo) string {
	if c.URL != "" {
		return c.URL
	}
	return fmt.Sprintf("%s/c/%s/+/%d", g.baseURL, c.Project, c.Number)
}

// gerritOwnerName extracts a display name from a Gerrit AccountInfo.
func gerritOwnerName(owner *gerrit.AccountInfo) string {
	if owner == nil {
		return ""
	}
	if owner.Name != "" {
		return owner.Name
	}
	if owner.Username != "" {
		return owner.Username
	}
	if owner.Email != "" {
		return owner.Email
	}
	return ""
}

// gerritMergeState converts a Gerrit change status to our MergeState.
func gerritMergeState(c *gerrit.ChangeInfo) MergeState {
	if c.WorkInProgress {
		return MergeStateWIP
	}
	switch c.Status {
	case "NEW":
		return MergeStateOpen
	case "MERGED":
		return MergeStateMerged
	case "ABANDONED":
		return MergeStateAbandoned
	default:
		return MergeStateOpen
	}
}

// gerritReviewState determines review state from Gerrit labels.
func gerritReviewState(c *gerrit.ChangeInfo) ReviewState {
	if c.Labels == nil {
		return ReviewStatePending
	}

	// Check Code-Review label.
	cr, ok := c.Labels["Code-Review"]
	if !ok {
		return ReviewStatePending
	}

	// If there's a rejected review (Code-Review -2).
	if cr.Rejected.AccountID != 0 {
		return ReviewStateRejected
	}

	// If there's a dislike (Code-Review -1).
	if cr.Disliked.AccountID != 0 {
		return ReviewStateChangesRequested
	}

	// If approved (Code-Review +2).
	if cr.Approved.AccountID != 0 {
		return ReviewStateApproved
	}

	// If recommended (Code-Review +1).
	if cr.Recommended.AccountID != 0 {
		return ReviewStateApproved
	}

	return ReviewStatePending
}

func (g *GerritForge) listChangeComments(ctx context.Context, changeID string, change *gerrit.ChangeInfo) ([]ReviewComment, error) {
	var out []ReviewComment
	for _, message := range change.Messages {
		body := strings.TrimSpace(message.Message)
		if body == "" {
			continue
		}
		out = append(out, ReviewComment{
			Kind:      ReviewCommentSystem,
			Author:    gerritOwnerName(&message.Author),
			Body:      body,
			CreatedAt: message.Date.Time,
			UpdatedAt: message.Date.Time,
		})
	}
	commentMap, _, err := g.client.Changes.ListChangeComments(ctx, changeID)
	if err != nil {
		sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
		return out, err
	}
	for path, comments := range *commentMap {
		for _, comment := range comments {
			out = append(out, ReviewComment{
				Kind:      ReviewCommentInline,
				Author:    gerritOwnerName(&comment.Author),
				Body:      strings.TrimSpace(comment.Message),
				File:      firstNonEmpty(comment.Path, path),
				Line:      comment.Line,
				CreatedAt: gerritTimestamp(comment.Updated),
				UpdatedAt: gerritTimestamp(comment.Updated),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].Body < out[j].Body
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (g *GerritForge) listRevisionFiles(ctx context.Context, changeID, revisionID string) ([]ReviewFile, error) {
	files, _, err := g.client.Changes.ListFiles(ctx, changeID, revisionID, &gerrit.FilesOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]ReviewFile, 0, len(files))
	for path, file := range files {
		if path == "/COMMIT_MSG" {
			continue
		}
		out = append(out, ReviewFile{
			Path:         path,
			PreviousPath: file.OldPath,
			Status:       file.Status,
			Additions:    file.LinesInserted,
			Deletions:    file.LinesDeleted,
			Binary:       file.Binary,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func (g *GerritForge) getRevisionPatch(ctx context.Context, changeID, revisionID string) (string, error) {
	encoded, _, err := g.client.Changes.GetPatch(ctx, changeID, revisionID, &gerrit.PatchOptions{})
	if err != nil || encoded == nil || *encoded == "" {
		return "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(*encoded)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func gerritTimestamp(ts *gerrit.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.Time
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// gerritStatusQuery converts our MergeState to a Gerrit query status filter.
func gerritStatusQuery(state MergeState) string {
	switch state {
	case MergeStateOpen, MergeStateWIP:
		return "status:open"
	case MergeStateMerged:
		return "status:merged"
	case MergeStateClosed, MergeStateAbandoned:
		return "status:abandoned"
	default:
		return "(status:open OR status:merged)"
	}
}
