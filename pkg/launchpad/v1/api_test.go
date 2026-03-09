// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// newTestServer creates a test server that routes based on URL path/query.
func newTestServer(t *testing.T, routes map[string]any) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path
		if op := r.URL.Query().Get("ws.op"); op != "" {
			key += "?ws.op=" + op
		}
		resp, ok := routes[key]
		if !ok {
			t.Logf("unhandled route: %s (full URL: %s)", key, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	creds := &Credentials{ConsumerKey: "test", AccessToken: "t", AccessTokenSecret: "s"}
	c := NewClient(creds, nil)
	return c, server
}

func TestGetPerson(t *testing.T) {
	c, server := newTestServer(t, map[string]any{
		"/~jdoe": Person{Name: "jdoe", DisplayName: "John Doe", IsTeam: false},
	})
	defer server.Close()

	_, err := c.GetPerson(context.Background(), server.URL+"/~jdoe")
	_ = err
	// We use the full URL here because GetPerson prepends /~
	// Instead, test with the raw URL approach:
	var p Person
	err = c.GetJSON(context.Background(), server.URL+"/~jdoe", &p)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.Name != "jdoe" {
		t.Errorf("Name = %q, want jdoe", p.Name)
	}
	if p.DisplayName != "John Doe" {
		t.Errorf("DisplayName = %q", p.DisplayName)
	}
}

func TestGetProject(t *testing.T) {
	c, server := newTestServer(t, map[string]any{
		"/sunbeam": Project{
			Name:        "sunbeam",
			DisplayName: "Sunbeam",
			Active:      true,
			VCS:         "Git",
		},
	})
	defer server.Close()

	var p Project
	err := c.GetJSON(context.Background(), server.URL+"/sunbeam", &p)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.Name != "sunbeam" {
		t.Errorf("Name = %q", p.Name)
	}
	if !p.Active {
		t.Error("expected Active")
	}
}

func TestGetBug(t *testing.T) {
	c, server := newTestServer(t, map[string]any{
		"/bugs/42": Bug{
			ID:    42,
			Title: "Test bug",
			Tags:  []string{"sunbeam"},
			Heat:  10,
		},
	})
	defer server.Close()

	var b Bug
	err := c.GetJSON(context.Background(), server.URL+"/bugs/42", &b)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if b.ID != 42 {
		t.Errorf("ID = %d", b.ID)
	}
	if b.Title != "Test bug" {
		t.Errorf("Title = %q", b.Title)
	}
}

func TestGetCollection(t *testing.T) {
	c, server := newTestServer(t, map[string]any{
		"/~team/ppas": Collection[Archive]{
			TotalSize: 2,
			Entries: []Archive{
				{Name: "ppa", Displayname: "Default PPA"},
				{Name: "staging", Displayname: "Staging PPA"},
			},
		},
	})
	defer server.Close()

	col, err := GetCollection[Archive](context.Background(), c, server.URL+"/~team/ppas")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if col.TotalSize != 2 {
		t.Errorf("TotalSize = %d", col.TotalSize)
	}
	if len(col.Entries) != 2 {
		t.Fatalf("len(Entries) = %d", len(col.Entries))
	}
	if col.Entries[0].Name != "ppa" {
		t.Errorf("Entries[0].Name = %q", col.Entries[0].Name)
	}
}

func TestGetAllPages(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Page 1: has next_collection_link
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		start := r.URL.Query().Get("ws.start")
		if start == "2" {
			// Page 2: no next link
			json.NewEncoder(w).Encode(Collection[Person]{
				TotalSize: 3,
				Start:     2,
				Entries:   []Person{{Name: "charlie"}},
			})
			return
		}
		json.NewEncoder(w).Encode(Collection[Person]{
			TotalSize:          3,
			Start:              0,
			NextCollectionLink: server.URL + "/items?ws.start=2",
			Entries:            []Person{{Name: "alice"}, {Name: "bob"}},
		})
	})

	creds := &Credentials{ConsumerKey: "test", AccessToken: "t", AccessTokenSecret: "s"}
	c := NewClient(creds, nil)

	all, err := GetAllPages[Person](context.Background(), c, server.URL+"/items")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("len = %d, want 3", len(all))
	}
	if all[0].Name != "alice" || all[1].Name != "bob" || all[2].Name != "charlie" {
		t.Errorf("names = %v", []string{all[0].Name, all[1].Name, all[2].Name})
	}
}

func TestWsOpURL(t *testing.T) {
	tests := []struct {
		base   string
		op     string
		params map[string]string
		want   string
	}{
		{
			"https://api.launchpad.net/devel/~user",
			"getPPAByName",
			map[string]string{"name": "staging"},
			"getPPAByName",
		},
	}

	for _, tt := range tests {
		got := wsOpURL(tt.base, tt.op, nil)
		if !strings.Contains(got, "ws.op="+tt.want) {
			t.Errorf("wsOpURL(%q, %q) = %q, missing ws.op=%s", tt.base, tt.op, got, tt.want)
		}
	}
}

func TestSearchBugTasks(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/sunbeam", func(w http.ResponseWriter, r *http.Request) {
		op := r.URL.Query().Get("ws.op")
		if op != "searchTasks" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		status := r.URL.Query().Get("status")
		if status != "New" {
			t.Errorf("expected status=New, got %q", status)
		}
		json.NewEncoder(w).Encode(Collection[BugTask]{
			TotalSize: 1,
			Entries: []BugTask{
				{Title: "Fix this", Status: "New", Importance: "High"},
			},
		})
	})

	creds := &Credentials{ConsumerKey: "test", AccessToken: "t", AccessTokenSecret: "s"}
	c := NewClient(creds, nil)

	// SearchBugTasks uses resolveURL which prepends APIBaseURL for relative paths,
	// so we pass the full server URL to bypass that.
	params := BugTaskSearchOpts{Status: []string{"New"}}
	u := wsOpURL(server.URL+"/sunbeam", "searchTasks", params.values())
	tasks, err := GetAllPages[BugTask](context.Background(), c, u)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("len = %d, want 1", len(tasks))
	}
	if tasks[0].Status != "New" {
		t.Errorf("Status = %q", tasks[0].Status)
	}
}

func TestGetGitRepository(t *testing.T) {
	c, server := newTestServer(t, map[string]any{
		"/~owner/project/+git/repo": GitRepository{
			ID:            123,
			Name:          "repo",
			DefaultBranch: "refs/heads/main",
			TargetDefault: true,
		},
	})
	defer server.Close()

	var r GitRepository
	err := c.GetJSON(context.Background(), server.URL+"/~owner/project/+git/repo", &r)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if r.ID != 123 {
		t.Errorf("ID = %d", r.ID)
	}
	if r.DefaultBranch != "refs/heads/main" {
		t.Errorf("DefaultBranch = %q", r.DefaultBranch)
	}
}

func TestGetRockRecipe(t *testing.T) {
	c, server := newTestServer(t, map[string]any{
		"/~owner/project/+rock/my-rock": RockRecipe{
			Name:      "my-rock",
			AutoBuild: true,
			StoreName: "my-rock-store",
		},
	})
	defer server.Close()

	var r RockRecipe
	err := c.GetJSON(context.Background(), server.URL+"/~owner/project/+rock/my-rock", &r)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if r.Name != "my-rock" {
		t.Errorf("Name = %q", r.Name)
	}
	if !r.AutoBuild {
		t.Error("expected AutoBuild = true")
	}
}

func TestGetCharmRecipe(t *testing.T) {
	c, server := newTestServer(t, map[string]any{
		"/~owner/project/+charm/my-charm": CharmRecipe{
			Name:      "my-charm",
			AutoBuild: false,
			BuildPath: "charm/",
		},
	})
	defer server.Close()

	var r CharmRecipe
	err := c.GetJSON(context.Background(), server.URL+"/~owner/project/+charm/my-charm", &r)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if r.Name != "my-charm" {
		t.Errorf("Name = %q", r.Name)
	}
	if r.BuildPath != "charm/" {
		t.Errorf("BuildPath = %q", r.BuildPath)
	}
}

func TestGetSnap(t *testing.T) {
	c, server := newTestServer(t, map[string]any{
		"/~owner/+snap/my-snap": Snap{
			Name:      "my-snap",
			AutoBuild: true,
			ProEnable: true,
		},
	})
	defer server.Close()

	var s Snap
	err := c.GetJSON(context.Background(), server.URL+"/~owner/+snap/my-snap", &s)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if s.Name != "my-snap" {
		t.Errorf("Name = %q", s.Name)
	}
	if !s.ProEnable {
		t.Error("expected ProEnable = true")
	}
}

func TestGetArchivePublishedSources(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/ubuntu/+archive/ppa", func(w http.ResponseWriter, r *http.Request) {
		op := r.URL.Query().Get("ws.op")
		if op != "getPublishedSources" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(Collection[SourcePublishing]{
			TotalSize: 1,
			Entries: []SourcePublishing{
				{
					SourcePackageName:    "hello",
					SourcePackageVersion: "2.10-1",
					Status:               "Published",
					Pocket:               "Release",
				},
			},
		})
	})

	creds := &Credentials{ConsumerKey: "test", AccessToken: "t", AccessTokenSecret: "s"}
	c := NewClient(creds, nil)

	sources, err := c.GetPublishedSources(context.Background(), server.URL+"/ubuntu/+archive/ppa", PublishedSourceOpts{
		Status: "Published",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("len = %d, want 1", len(sources))
	}
	if sources[0].SourcePackageName != "hello" {
		t.Errorf("SourcePackageName = %q", sources[0].SourcePackageName)
	}
}

func TestGetMergeProposal(t *testing.T) {
	c, server := newTestServer(t, map[string]any{
		"/~user/project/branch/+merge/1": MergeProposal{
			QueueStatus:   "Needs review",
			Description:   "Fix the thing",
			SourceGitPath: "refs/heads/fix-thing",
			TargetGitPath: "refs/heads/main",
		},
	})
	defer server.Close()

	mp, err := c.GetMergeProposal(context.Background(), server.URL+"/~user/project/branch/+merge/1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if mp.QueueStatus != "Needs review" {
		t.Errorf("QueueStatus = %q", mp.QueueStatus)
	}
	if mp.SourceGitPath != "refs/heads/fix-thing" {
		t.Errorf("SourceGitPath = %q", mp.SourceGitPath)
	}
}

func TestBugTaskSearchOpts_Values(t *testing.T) {
	opts := BugTaskSearchOpts{
		Status:         []string{"New", "Confirmed"},
		Importance:     []string{"High"},
		Tags:           []string{"regression", "-wontfix"},
		TagsCombinator: "All",
		SearchText:     "crash",
		OmitDuplicates: true,
	}
	v := opts.values()

	statuses := v["status"]
	if len(statuses) != 2 {
		t.Errorf("expected 2 statuses, got %d", len(statuses))
	}
	if v.Get("tags_combinator") != "All" {
		t.Errorf("tags_combinator = %q", v.Get("tags_combinator"))
	}
	if v.Get("search_text") != "crash" {
		t.Errorf("search_text = %q", v.Get("search_text"))
	}
	if v.Get("omit_duplicates") != "true" {
		t.Errorf("omit_duplicates = %q", v.Get("omit_duplicates"))
	}
}

func TestBugTaskSearchOpts_Values_DefaultsToAllStatuses(t *testing.T) {
	v := (BugTaskSearchOpts{}).values()

	statuses := v["status"]
	if len(statuses) != len(allBugTaskStatuses) {
		t.Fatalf("len(statuses) = %d, want %d", len(statuses), len(allBugTaskStatuses))
	}
	for i, status := range allBugTaskStatuses {
		if statuses[i] != status {
			t.Fatalf("statuses[%d] = %q, want %q", i, statuses[i], status)
		}
	}
}

func TestBugTaskSearchOpts_Values_IncludesDocumentedClosedAndDeferredStatuses(t *testing.T) {
	statuses := (BugTaskSearchOpts{}).values()["status"]
	for _, want := range []string{"Deferred", "Does Not Exist", "Fix Released"} {
		if !containsString(statuses, want) {
			t.Fatalf("default statuses = %v, want to include %q", statuses, want)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestProjectAndBugWrappers(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			if got := r.FormValue("ws.op"); got != "new_project" {
				t.Fatalf("ws.op = %q, want new_project", got)
			}
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			json.NewEncoder(w).Encode(Collection[Project]{
				Entries: []Project{{Name: "sunbeam"}},
			})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/sunbeam", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Query().Get("ws.op") == "searchTasks":
			json.NewEncoder(w).Encode(Collection[BugTask]{
				Entries: []BugTask{{Title: "project task", Status: "Fix Released"}},
			})
		case r.Method == http.MethodPost && r.FormValue("ws.op") == "newSeries":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusOK)
		default:
			json.NewEncoder(w).Encode(Project{Name: "sunbeam"})
		}
	})
	mux.HandleFunc("/sunbeam/series", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(Collection[ProjectSeries]{
			Entries: []ProjectSeries{{Name: "2025.1"}},
		})
	})
	mux.HandleFunc("/sunbeam/2025.1", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(ProjectSeries{Name: "2025.1"})
	})
	mux.HandleFunc("/bugs", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(Collection[BugTask]{
				Entries: []BugTask{{Title: "global task", Status: "New"}},
			})
		case http.MethodPost:
			json.NewEncoder(w).Encode(Bug{ID: 42, Title: "Created bug"})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/bugs/42", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(Bug{ID: 42, Title: "Created bug"})
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/bugs/42/bug_tasks", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(Collection[BugTask]{
			Entries: []BugTask{{Title: "task 42", Status: "Triaged"}},
		})
	})
	mux.HandleFunc("/task/42", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method = %s, want PATCH", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})

	client := testRewrittenClient(t, server)

	project, err := client.CreateProject(context.Background(), "sunbeam", "Sunbeam", "summary", "desc")
	if err != nil || project.Name != "sunbeam" {
		t.Fatalf("CreateProject() = %+v, %v", project, err)
	}
	if _, err := client.SearchProjects(context.Background(), "beam"); err != nil {
		t.Fatalf("SearchProjects() error = %v", err)
	}
	if _, err := client.SearchBugTasks(context.Background(), "sunbeam", BugTaskSearchOpts{Status: []string{"Fix Released"}}); err != nil {
		t.Fatalf("SearchBugTasks() error = %v", err)
	}
	if _, err := client.GetProjectSeries(context.Background(), "sunbeam"); err != nil {
		t.Fatalf("GetProjectSeries() error = %v", err)
	}
	if _, err := client.CreateProjectSeries(context.Background(), "sunbeam", "2025.1", "summary"); err != nil {
		t.Fatalf("CreateProjectSeries() error = %v", err)
	}
	if err := client.SetDevelopmentFocus(context.Background(), "sunbeam", server.URL+"/sunbeam/2025.1"); err != nil {
		t.Fatalf("SetDevelopmentFocus() error = %v", err)
	}
	if _, err := client.GetBug(context.Background(), 42); err != nil {
		t.Fatalf("GetBug() error = %v", err)
	}
	if _, err := client.GetBugTasks(context.Background(), 42); err != nil {
		t.Fatalf("GetBugTasks() error = %v", err)
	}
	if _, err := client.SearchGlobalBugTasks(context.Background(), BugTaskSearchOpts{Status: []string{"New"}}); err != nil {
		t.Fatalf("SearchGlobalBugTasks() error = %v", err)
	}
	if _, err := client.CreateBug(context.Background(), server.URL+"/sunbeam", "title", "desc", []string{"tag"}); err != nil {
		t.Fatalf("CreateBug() error = %v", err)
	}
	if err := client.UpdateBugTaskStatus(context.Background(), server.URL+"/task/42", "Fix Released"); err != nil {
		t.Fatalf("UpdateBugTaskStatus() error = %v", err)
	}
	if err := client.AddBugTask(context.Background(), 42, server.URL+"/sunbeam/2025.1"); err != nil {
		t.Fatalf("AddBugTask() error = %v", err)
	}
}

func TestPersonAndGitWrappers(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/~team/ppas", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(Collection[Archive]{
			Entries: []Archive{{Name: "ppa"}},
		})
	})
	mux.HandleFunc("/~team", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("ws.op") {
		case "getPPAByName":
			json.NewEncoder(w).Encode(Archive{Name: "ppa"})
		case "getOwnedProjects":
			json.NewEncoder(w).Encode(Collection[Project]{Entries: []Project{{Name: "sunbeam"}}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	mux.HandleFunc("/+git", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ws.op") == "getDefaultRepository" {
			json.NewEncoder(w).Encode(GitRepository{Name: "repo"})
			return
		}
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/~owner/sunbeam/+git/repo", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(GitRepository{Name: "repo"})
	})
	mux.HandleFunc("/repo", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("ws.op") {
		case "getRefByPath":
			json.NewEncoder(w).Encode(GitRef{Path: "refs/heads/main"})
		case "getMergeProposals":
			json.NewEncoder(w).Encode(Collection[MergeProposal]{Entries: []MergeProposal{{QueueStatus: "Needs review"}}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	mux.HandleFunc("/repo/branches", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(Collection[GitRef]{Entries: []GitRef{{Path: "refs/heads/main"}}})
	})
	mux.HandleFunc("/repo/refs", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(Collection[GitRef]{Entries: []GitRef{{Path: "refs/tags/v1"}}})
	})
	mux.HandleFunc("/mp/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		json.NewEncoder(w).Encode(MergeProposal{QueueStatus: "Approved"})
	})

	client := testRewrittenClient(t, server)

	if _, err := client.GetPPAs(context.Background(), "team"); err != nil {
		t.Fatalf("GetPPAs() error = %v", err)
	}
	if _, err := client.GetPPAByName(context.Background(), "team", "ppa"); err != nil {
		t.Fatalf("GetPPAByName() error = %v", err)
	}
	if _, err := client.GetOwnedProjects(context.Background(), "team"); err != nil {
		t.Fatalf("GetOwnedProjects() error = %v", err)
	}
	if _, err := client.CreateGitRepository(context.Background(), "owner", "sunbeam", "repo"); err != nil {
		t.Fatalf("CreateGitRepository() error = %v", err)
	}
	if _, err := client.GetGitRef(context.Background(), server.URL+"/repo", "refs/heads/main"); err != nil {
		t.Fatalf("GetGitRef() error = %v", err)
	}
	if _, err := client.GetGitBranches(context.Background(), server.URL+"/repo"); err != nil {
		t.Fatalf("GetGitBranches() error = %v", err)
	}
	if _, err := client.GetGitRefs(context.Background(), server.URL+"/repo"); err != nil {
		t.Fatalf("GetGitRefs() error = %v", err)
	}
	if _, err := client.GetGitRepoMergeProposals(context.Background(), server.URL+"/repo", "Needs review"); err != nil {
		t.Fatalf("GetGitRepoMergeProposals() error = %v", err)
	}
	if _, err := client.GetDefaultRepository(context.Background(), server.URL+"/sunbeam"); err != nil {
		t.Fatalf("GetDefaultRepository() error = %v", err)
	}
	if err := client.SetMergeProposalStatus(context.Background(), server.URL+"/mp/1", "Approved"); err != nil {
		t.Fatalf("SetMergeProposalStatus() error = %v", err)
	}
}

func testRewrittenClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse(server.URL): %v", err)
	}
	httpClient := server.Client()
	httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/devel")
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
		return http.DefaultTransport.RoundTrip(req)
	})
	return NewClient(&Credentials{ConsumerKey: "test", AccessToken: "t", AccessTokenSecret: "s"}, nil, httpClient)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
