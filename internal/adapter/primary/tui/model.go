// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	frontend "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/frontend"
	runtimeadapter "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/runtime"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

type viewID int

const (
	viewDashboard viewID = iota
	viewBuilds
	viewReleases
	viewPackages
	viewBugs
	viewReviews
	viewCommits
	viewProjects
)

type overlayKind int

const (
	overlayNone overlayKind = iota
	overlayHelp
	overlayAuth
	overlayOperations
	overlayCache
	overlayLogs
	overlayServer
	overlayPrompt
	overlayBuildFilters
	overlayReleaseFilters
	overlayBuildTrigger
	overlayPackageFilters
	overlayBugFilters
	overlayReviewFilters
	overlayCommitFilters
	overlayProjectFilters
)

type deferredActionKind int

const (
	deferredNone deferredActionKind = iota
	deferredAuthLogin
	deferredAuthLogout
	deferredOperationCancel
	deferredBuildTrigger
	deferredSwitchServer
)

type deferredAction struct {
	kind        deferredActionKind
	operationID string
	buildReq    frontend.BuildTriggerRequest
}

type toastState struct {
	message string
	level   string
}

type dashboardModel struct {
	section int
	auth    *dto.AuthStatus
	ops     []dto.OperationJob
	builds  []dto.Build
	cache   *frontend.CacheStatusResponse
	loaded  bool
	err     string
}

type buildsFilters struct {
	project string
	state   string
	active  bool
	source  string
}

type buildsModel struct {
	filters buildsFilters
	rows    []dto.Build
	index   int
	loaded  bool
	err     string
}

type releasesFilters struct {
	project       string
	artifactType  string
	risk          string
	track         string
	branch        string
	targetProfile string
	allTargets    bool
}

type releasesModel struct {
	filters   releasesFilters
	rows      []dto.ReleaseListEntry
	artifacts []releaseArtifactSummary
	index     int
	detail    *dto.ReleaseShowResult
	loaded    bool
	err       string
}

type releaseArtifactSummary struct {
	Project             string
	Name                string
	ArtifactType        dto.ArtifactType
	ReleasedAt          time.Time
	ChannelCount        int
	ResourceCount       int
	LatestVisibleTarget string
	VisibleTargetCount  int
}

type operationsDrawerModel struct {
	rows   []dto.OperationJob
	events []dto.OperationEvent
	index  int
	loaded bool
	err    string
}

type authModalModel struct {
	status *dto.AuthStatus
	begin  *dto.LaunchpadAuthBeginResult
	loaded bool
	err    string
}

type cacheModalModel struct {
	status *frontend.CacheStatusResponse
	loaded bool
	err    string
}

type serverModalModel struct {
	local  *runtimeadapter.LocalServerStatus
	err    string
	loaded bool
}

type logsModalModel struct {
	sessionLines []string
	daemonLines  []string
	daemonNote   string
	loaded       bool
	err          string
}

type formModalModel struct {
	title    string
	fields   []textinput.Model
	active   int
	errorMsg string
}

type promptModel struct {
	title  string
	body   string
	accept string
	reject string
}

type releaseFilterOptions struct {
	projects       []string
	artifactTypes  []string
	risks          []string
	tracks         []string
	branches       []string
	targetProfiles []string
}

type dashboardLoadedMsg struct {
	auth   *dto.AuthStatus
	ops    []dto.OperationJob
	builds []dto.Build
	cache  *frontend.CacheStatusResponse
	err    error
}

type buildsLoadedMsg struct {
	rows []dto.Build
	err  error
}

type releasesLoadedMsg struct {
	rows []dto.ReleaseListEntry
	err  error
}

type releaseDetailLoadedMsg struct {
	key    string
	detail *dto.ReleaseShowResult
	err    error
}

type opsLoadedMsg struct {
	rows   []dto.OperationJob
	events []dto.OperationEvent
	err    error
}

type authStatusLoadedMsg struct {
	status *dto.AuthStatus
	err    error
}

type authBeginMsg struct {
	begin *dto.LaunchpadAuthBeginResult
	err   error
}

type authFinalizeMsg struct {
	result *dto.LaunchpadAuthFinalizeResult
	err    error
}

type authLogoutMsg struct {
	result *dto.LaunchpadAuthLogoutResult
	err    error
}

type cacheLoadedMsg struct {
	status *frontend.CacheStatusResponse
	err    error
}

type localServerStatusMsg struct {
	status runtimeadapter.LocalServerStatus
	err    error
}

type logsLoadedMsg struct {
	sessionLines []string
	daemonLines  []string
	daemonNote   string
	err          error
}

type buildTriggeredMsg struct {
	job *dto.OperationJob
	err error
}

type operationCancelledMsg struct {
	job *dto.OperationJob
	err error
}

type upgradedMsg struct {
	err error
}

type browserOpenedMsg struct {
	err error
}

type tickDashboardMsg time.Time
type tickOperationsMsg time.Time
type tickLogsMsg time.Time
type clearToastMsg struct{}

type rootModel struct {
	session *runtimeadapter.Session
	logs    *logBuffer
	theme   theme

	width  int
	height int

	activeView viewID
	overlay    overlayKind

	lastRefresh   time.Time
	toast         toastState
	contentScroll int
	overlayScroll int
	pendingG      bool

	dashboard dashboardModel
	builds    buildsModel
	releases  releasesModel
	packages  packagesModel
	bugs      bugsModel
	reviews   reviewsModel
	commits   commitsModel
	projects  projectsModel
	ops       operationsDrawerModel
	auth      authModalModel
	cache     cacheModalModel
	logsModal logsModalModel
	server    serverModalModel

	buildFilterForm   formModalModel
	releaseFilterForm formModalModel
	buildTriggerForm  formModalModel
	packageFilterForm formModalModel
	bugFilterForm     formModalModel
	reviewFilterForm  formModalModel
	commitFilterForm  formModalModel
	projectFilterForm formModalModel
	prompt            promptModel
	deferred          deferredAction
}

func newRootModel(session *runtimeadapter.Session, noColor bool) rootModel {
	return newRootModelWithLogs(session, noColor, nil)
}

func newRootModelWithLogs(session *runtimeadapter.Session, noColor bool, logs *logBuffer) rootModel {
	t := newTheme()
	if noColor {
		lipgloss.SetColorProfile(0)
	}
	m := rootModel{
		session:    session,
		logs:       logs,
		theme:      t,
		activeView: viewDashboard,
		builds: buildsModel{
			filters: buildsFilters{active: true, source: "remote"},
		},
		releases: releasesModel{
			filters: releasesFilters{},
		},
		packages: packagesModel{
			filters: packagesFilters{mode: packageModeInventory, backport: "none"},
		},
		bugs: bugsModel{
			filters: bugsFilters{merge: true},
		},
		commits: commitsModel{
			filters: commitsFilters{mode: commitModeLog},
		},
	}
	return m
}

func (m rootModel) Init() tea.Cmd {
	return tea.Batch(
		loadDashboardCmd(m.session),
		loadBuildsCmd(m.session, m.builds.filters),
		loadReleasesCmd(m.session, m.releases.filters),
		loadPackagesCmd(m.session, m.packages.filters),
		loadBugsCmd(m.session, m.bugs.filters),
		loadReviewsCmd(m.session, m.reviews.filters),
		loadCommitsCmd(m.session, m.commits.filters),
		loadProjectsCmd(m.session, m.projects.filters),
		tickDashboardCmd(),
	)
}

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.overlay != overlayNone {
			return m.updateOverlay(msg)
		}
		return m.updateGlobal(msg)
	case dashboardLoadedMsg:
		m.dashboard.loaded = msg.err == nil
		m.dashboard.err = errString(msg.err)
		if msg.err == nil {
			m.dashboard.auth = msg.auth
			m.dashboard.ops = msg.ops
			m.dashboard.builds = msg.builds
			m.dashboard.cache = msg.cache
			m.lastRefresh = time.Now()
		}
	case buildsLoadedMsg:
		m.builds.loaded = msg.err == nil
		m.builds.err = errString(msg.err)
		if msg.err == nil {
			m.builds.rows = msg.rows
			if m.builds.index >= len(m.builds.rows) && len(m.builds.rows) > 0 {
				m.builds.index = len(m.builds.rows) - 1
			}
			m.lastRefresh = time.Now()
		}
	case releasesLoadedMsg:
		m.releases.loaded = msg.err == nil
		m.releases.err = errString(msg.err)
		if msg.err == nil {
			m.releases.rows = msg.rows
			m.releases.artifacts = summarizeReleaseArtifacts(msg.rows)
			if m.releases.index >= len(m.releases.artifacts) && len(m.releases.artifacts) > 0 {
				m.releases.index = len(m.releases.artifacts) - 1
			}
			m.lastRefresh = time.Now()
			if artifact := m.selectedReleaseArtifact(); artifact != nil {
				return m, loadReleaseDetailCmd(m.session, *artifact, m.releases.filters)
			}
		}
	case releaseDetailLoadedMsg:
		if msg.err == nil && msg.key == m.releaseDetailKey() {
			m.releases.detail = msg.detail
		}
	case packagesLoadedMsg:
		m.packages.loaded = msg.err == nil
		m.packages.err = errString(msg.err)
		if msg.err == nil {
			m.packages.inventoryRows = msg.inventoryRows
			m.packages.diffRows = msg.diffRows
			m.packages.diffSources = msg.diffSources
			m.packages.diffHasUpstream = msg.diffHasUpstream
			m.packages.excuseRows = msg.excuseRows
			m.packages.prompt = msg.prompt
			m.packages.index = clampIndex(m.packages.index, m.packages.rowCount())
			m.packages.clearStaleDetail()
			m.lastRefresh = time.Now()
			return m, loadPackageDetailCmd(m.session, m.packages)
		}
	case packageInventoryDetailLoadedMsg:
		if msg.err == nil && msg.key == m.packageDetailKey() {
			m.packages.inventoryDetail = msg.detail
		}
	case packageDiffDetailLoadedMsg:
		if msg.err == nil && msg.key == m.packageDetailKey() {
			m.packages.diffDetail = msg.detail
		}
	case packageExcuseDetailLoadedMsg:
		if msg.err == nil && msg.key == m.packageDetailKey() {
			m.packages.excuseDetail = msg.detail
		}
	case bugsLoadedMsg:
		m.bugs.loaded = msg.err == nil
		m.bugs.err = errString(msg.err)
		if msg.err == nil {
			m.bugs.rows = msg.rows
			m.bugs.warnings = msg.warnings
			m.bugs.index = clampIndex(m.bugs.index, len(m.bugs.rows))
			if task := selectedBug(m.bugs.rows, m.bugs.index); task != nil {
				m.lastRefresh = time.Now()
				return m, loadBugDetailCmd(m.session, *task)
			}
			m.bugs.detail = nil
			m.lastRefresh = time.Now()
		}
	case bugDetailLoadedMsg:
		if msg.err == nil && msg.key == m.bugDetailKey() {
			m.bugs.detail = msg.detail
		}
	case reviewsLoadedMsg:
		m.reviews.loaded = msg.err == nil
		m.reviews.err = errString(msg.err)
		if msg.err == nil {
			m.reviews.rows = msg.rows
			m.reviews.warnings = msg.warnings
			m.reviews.index = clampIndex(m.reviews.index, len(m.reviews.rows))
			if mr := selectedReview(m.reviews.rows, m.reviews.index); mr != nil {
				m.lastRefresh = time.Now()
				return m, loadReviewDetailCmd(m.session, *mr)
			}
			m.reviews.detail = nil
			m.lastRefresh = time.Now()
		}
	case reviewDetailLoadedMsg:
		if msg.err == nil && msg.key == m.reviewDetailKey() {
			m.reviews.detail = msg.detail
		}
	case commitsLoadedMsg:
		m.commits.loaded = msg.err == nil
		m.commits.err = errString(msg.err)
		if msg.err == nil {
			m.commits.rows = msg.rows
			m.commits.warnings = msg.warnings
			m.commits.prompt = msg.prompt
			m.commits.index = clampIndex(m.commits.index, len(m.commits.rows))
			m.lastRefresh = time.Now()
		}
	case projectsLoadedMsg:
		m.projects.loaded = msg.err == nil
		m.projects.err = errString(msg.err)
		if msg.err == nil {
			m.projects.config = msg.config
			m.projects.rows = msg.rows
			m.projects.index = clampIndex(m.projects.index, len(m.projects.rows))
			m.lastRefresh = time.Now()
		}
	case opsLoadedMsg:
		m.ops.loaded = msg.err == nil
		m.ops.err = errString(msg.err)
		if msg.err == nil {
			m.ops.rows = msg.rows
			if m.ops.index >= len(m.ops.rows) && len(m.ops.rows) > 0 {
				m.ops.index = len(m.ops.rows) - 1
			}
			m.ops.events = msg.events
		}
	case authStatusLoadedMsg:
		m.auth.loaded = msg.err == nil
		m.auth.err = errString(msg.err)
		if msg.err == nil {
			m.auth.status = msg.status
			if m.dashboard.auth == nil {
				m.dashboard.auth = msg.status
			}
		}
	case authBeginMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), "error")
			m.auth.err = msg.err.Error()
			return m, clearToastLater()
		}
		m.auth.begin = msg.begin
		m.setToast("Launchpad auth started", "info")
		return m, clearToastLater()
	case authFinalizeMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), "error")
			m.auth.err = msg.err.Error()
			return m, clearToastLater()
		}
		m.auth.begin = nil
		m.auth.status = &dto.AuthStatus{Launchpad: msg.result.Launchpad}
		m.dashboard.auth = m.auth.status
		m.setToast("Launchpad login completed", "success")
		return m, tea.Batch(clearToastLater(), loadDashboardCmd(m.session))
	case authLogoutMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), "error")
			return m, clearToastLater()
		}
		m.auth.begin = nil
		m.auth.status = &dto.AuthStatus{}
		m.dashboard.auth = m.auth.status
		m.setToast("Launchpad credentials cleared", "success")
		return m, tea.Batch(clearToastLater(), loadDashboardCmd(m.session))
	case cacheLoadedMsg:
		m.cache.loaded = msg.err == nil
		m.cache.err = errString(msg.err)
		if msg.err == nil {
			m.cache.status = msg.status
		}
	case localServerStatusMsg:
		m.server.loaded = msg.err == nil
		m.server.err = errString(msg.err)
		if msg.err == nil {
			status := msg.status
			m.server.local = &status
		}
	case logsLoadedMsg:
		m.logsModal.loaded = msg.err == nil
		m.logsModal.err = errString(msg.err)
		if msg.err == nil {
			m.logsModal.sessionLines = msg.sessionLines
			m.logsModal.daemonLines = msg.daemonLines
			m.logsModal.daemonNote = msg.daemonNote
		}
	case buildTriggeredMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), "error")
			return m, clearToastLater()
		}
		m.overlay = overlayOperations
		m.setToast("Build trigger queued", "success")
		if msg.job != nil {
			m.ops.index = 0
		}
		return m, tea.Batch(clearToastLater(), loadOperationsCmd(m.session, selectedOperationID(m.ops.rows, m.ops.index)))
	case operationCancelledMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), "error")
			return m, clearToastLater()
		}
		m.setToast("Operation cancelled", "success")
		return m, tea.Batch(clearToastLater(), loadDashboardCmd(m.session), loadOperationsCmd(m.session, selectedOperationID(m.ops.rows, m.ops.index)))
	case upgradedMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), "error")
			m.overlay = overlayNone
			m.deferred = deferredAction{}
			return m, clearToastLater()
		}
		m.overlay = overlayNone
		action := m.deferred
		m.deferred = deferredAction{}
		cmd := m.resumeDeferredAction(action)
		m.setToast("Switched to local daemon", "success")
		return m, tea.Batch(clearToastLater(), cmd, loadDashboardCmd(m.session))
	case browserOpenedMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), "error")
		} else {
			m.setToast("Opened browser", "info")
		}
		return m, clearToastLater()
	case actionDeniedMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), "error")
			return m, clearToastLater()
		}
	case tickDashboardMsg:
		if m.activeView == viewDashboard && m.overlay == overlayNone {
			return m, tea.Batch(loadDashboardCmd(m.session), tickDashboardCmd())
		}
		return m, tickDashboardCmd()
	case tickOperationsMsg:
		if m.overlay == overlayOperations || hasRunningOperation(m.ops.rows) {
			return m, tea.Batch(loadOperationsCmd(m.session, selectedOperationID(m.ops.rows, m.ops.index)), tickOperationsCmd())
		}
		return m, nil
	case tickLogsMsg:
		if m.overlay == overlayLogs {
			return m, tea.Batch(loadLogsCmd(m.session, m.logs), tickLogsCmd())
		}
		return m, nil
	case clearToastMsg:
		m.toast = toastState{}
	}
	return m, nil
}

func (m rootModel) updateGlobal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "g":
		if m.pendingG {
			m.pendingG = false
			return m.jumpActiveTop()
		}
		m.pendingG = true
		return m, nil
	case "G":
		m.pendingG = false
		return m.jumpActiveBottom()
	default:
		m.pendingG = false
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "m", "?":
		m.overlay = overlayHelp
		m.overlayScroll = 0
		return m, nil
	case "a":
		m.overlay = overlayAuth
		return m, loadAuthStatusCmd(m.session)
	case "o":
		m.overlay = overlayOperations
		return m, tea.Batch(loadOperationsCmd(m.session, selectedOperationID(m.ops.rows, m.ops.index)), tickOperationsCmd())
	case "c":
		m.overlay = overlayCache
		return m, loadCacheCmd(m.session)
	case "l":
		m.overlay = overlayLogs
		m.overlayScroll = 0
		return m, tea.Batch(loadLogsCmd(m.session, m.logs), tickLogsCmd())
	case "s":
		m.overlay = overlayServer
		return m, loadLocalServerStatusCmd(m.session)
	case "1":
		m.activeView = viewDashboard
		m.contentScroll = 0
		return m, nil
	case "2":
		m.activeView = viewBuilds
		m.contentScroll = 0
		return m, nil
	case "3":
		m.activeView = viewReleases
		m.contentScroll = 0
		return m, nil
	case "4":
		m.activeView = viewPackages
		m.contentScroll = 0
		return m, nil
	case "5":
		m.activeView = viewBugs
		m.contentScroll = 0
		return m, nil
	case "6":
		m.activeView = viewReviews
		m.contentScroll = 0
		return m, nil
	case "7":
		m.activeView = viewCommits
		m.contentScroll = 0
		return m, nil
	case "8":
		m.activeView = viewProjects
		m.contentScroll = 0
		return m, nil
	case "tab":
		m.activeView = (m.activeView + 1) % 8
		m.contentScroll = 0
		return m, nil
	case "shift+tab":
		m.activeView--
		if m.activeView < 0 {
			m.activeView = viewProjects
		}
		m.contentScroll = 0
		return m, nil
	case "r":
		return m, m.refreshActiveView()
	case "pgdown", "ctrl+d":
		m.contentScroll += m.scrollStep()
		return m, nil
	case "pgup", "ctrl+u":
		m.contentScroll -= m.scrollStep()
		if m.contentScroll < 0 {
			m.contentScroll = 0
		}
		return m, nil
	case "/":
		switch m.activeView {
		case viewBuilds:
			m.buildFilterForm = newBuildFilterForm(m.builds.filters)
			m.overlay = overlayBuildFilters
			m.overlayScroll = 0
		case viewReleases:
			m.releaseFilterForm = newReleaseFilterForm(m.session, m.releases)
			m.overlay = overlayReleaseFilters
			m.overlayScroll = 0
		case viewPackages:
			m.packageFilterForm = newPackageFilterForm(m.session, m.packages)
			m.overlay = overlayPackageFilters
			m.overlayScroll = 0
		case viewBugs:
			m.bugFilterForm = newBugFilterForm(m.session, m.bugs)
			m.overlay = overlayBugFilters
			m.overlayScroll = 0
		case viewReviews:
			m.reviewFilterForm = newReviewFilterForm(m.session, m.reviews)
			m.overlay = overlayReviewFilters
			m.overlayScroll = 0
		case viewCommits:
			m.commitFilterForm = newCommitFilterForm(m.session, m.commits)
			m.overlay = overlayCommitFilters
			m.overlayScroll = 0
		case viewProjects:
			m.projectFilterForm = newProjectFilterForm(m.projects)
			m.overlay = overlayProjectFilters
			m.overlayScroll = 0
		}
		return m, nil
	case "[":
		switch m.activeView {
		case viewPackages:
			m.packages.filters.mode--
			if m.packages.filters.mode < packageModeInventory {
				m.packages.filters.mode = packageModeExcuses
			}
			m.packages.index = 0
			m.packages.clearDetails()
			return m, loadPackagesCmd(m.session, m.packages.filters)
		case viewCommits:
			m.commits.filters.mode--
			if m.commits.filters.mode < commitModeLog {
				m.commits.filters.mode = commitModeTrack
			}
			m.commits.index = 0
			return m, loadCommitsCmd(m.session, m.commits.filters)
		}
	case "]":
		switch m.activeView {
		case viewPackages:
			m.packages.filters.mode = (m.packages.filters.mode + 1) % 3
			m.packages.index = 0
			m.packages.clearDetails()
			return m, loadPackagesCmd(m.session, m.packages.filters)
		case viewCommits:
			m.commits.filters.mode = (m.commits.filters.mode + 1) % 2
			m.commits.index = 0
			return m, loadCommitsCmd(m.session, m.commits.filters)
		}
	case "up", "k":
		switch m.activeView {
		case viewDashboard:
			if m.dashboard.section > 0 {
				m.dashboard.section--
			}
		case viewBuilds:
			if m.builds.index > 0 {
				m.builds.index--
			}
		case viewReleases:
			if m.releases.index > 0 {
				m.releases.index--
				if artifact := m.selectedReleaseArtifact(); artifact != nil {
					return m, loadReleaseDetailCmd(m.session, *artifact, m.releases.filters)
				}
			}
		case viewPackages:
			if m.packages.index > 0 {
				m.packages.index--
				return m, loadPackageDetailCmd(m.session, m.packages)
			}
		case viewBugs:
			if m.bugs.index > 0 {
				m.bugs.index--
				if task := selectedBug(m.bugs.rows, m.bugs.index); task != nil {
					return m, loadBugDetailCmd(m.session, *task)
				}
			}
		case viewReviews:
			if m.reviews.index > 0 {
				m.reviews.index--
				if review := selectedReview(m.reviews.rows, m.reviews.index); review != nil {
					return m, loadReviewDetailCmd(m.session, *review)
				}
			}
		case viewCommits:
			if m.commits.index > 0 {
				m.commits.index--
			}
		case viewProjects:
			if m.projects.index > 0 {
				m.projects.index--
			}
		}
	case "down", "j":
		switch m.activeView {
		case viewDashboard:
			if m.dashboard.section < 3 {
				m.dashboard.section++
			}
		case viewBuilds:
			if m.builds.index < len(m.builds.rows)-1 {
				m.builds.index++
			}
		case viewReleases:
			if m.releases.index < len(m.releases.artifacts)-1 {
				m.releases.index++
				if artifact := m.selectedReleaseArtifact(); artifact != nil {
					return m, loadReleaseDetailCmd(m.session, *artifact, m.releases.filters)
				}
			}
		case viewPackages:
			if m.packages.index < m.packages.rowCount()-1 {
				m.packages.index++
				return m, loadPackageDetailCmd(m.session, m.packages)
			}
		case viewBugs:
			if m.bugs.index < len(m.bugs.rows)-1 {
				m.bugs.index++
				if task := selectedBug(m.bugs.rows, m.bugs.index); task != nil {
					return m, loadBugDetailCmd(m.session, *task)
				}
			}
		case viewReviews:
			if m.reviews.index < len(m.reviews.rows)-1 {
				m.reviews.index++
				if review := selectedReview(m.reviews.rows, m.reviews.index); review != nil {
					return m, loadReviewDetailCmd(m.session, *review)
				}
			}
		case viewCommits:
			if m.commits.index < len(m.commits.rows)-1 {
				m.commits.index++
			}
		case viewProjects:
			if m.projects.index < len(m.projects.rows)-1 {
				m.projects.index++
			}
		}
	case "enter":
		switch m.activeView {
		case viewDashboard:
			switch m.dashboard.section {
			case 0:
				m.overlay = overlayAuth
				m.overlayScroll = 0
				return m, loadAuthStatusCmd(m.session)
			case 1:
				m.overlay = overlayOperations
				m.overlayScroll = 0
				return m, tea.Batch(loadOperationsCmd(m.session, selectedOperationID(m.ops.rows, m.ops.index)), tickOperationsCmd())
			case 2:
				m.activeView = viewBuilds
			case 3:
				m.activeView = viewReleases
			}
		case viewReleases:
			if artifact := m.selectedReleaseArtifact(); artifact != nil {
				return m, loadReleaseDetailCmd(m.session, *artifact, m.releases.filters)
			}
		case viewPackages:
			return m, loadPackageDetailCmd(m.session, m.packages)
		case viewBugs:
			if task := selectedBug(m.bugs.rows, m.bugs.index); task != nil {
				return m, loadBugDetailCmd(m.session, *task)
			}
		case viewReviews:
			if review := selectedReview(m.reviews.rows, m.reviews.index); review != nil {
				return m, loadReviewDetailCmd(m.session, *review)
			}
		}
	case "t":
		if m.activeView == viewBuilds {
			m.buildTriggerForm = newBuildTriggerForm(m.session)
			m.overlay = overlayBuildTrigger
			m.overlayScroll = 0
			return m, nil
		}
	}
	return m, nil
}

func (m rootModel) updateOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.overlay {
	case overlayHelp:
		switch msg.String() {
		case "g":
			if m.pendingG {
				m.pendingG = false
				m.overlayScroll = 0
				return m, nil
			}
			m.pendingG = true
			return m, nil
		case "G":
			m.pendingG = false
			m.overlayScroll = viewportEndOffset()
			return m, nil
		default:
			m.pendingG = false
		}
		switch msg.String() {
		case "esc", "q", "?":
			m.overlay = overlayNone
			m.overlayScroll = 0
		case "pgdown", "ctrl+d", "down", "j":
			m.overlayScroll += m.scrollStep()
		case "pgup", "ctrl+u", "up", "k":
			m.overlayScroll -= m.scrollStep()
			if m.overlayScroll < 0 {
				m.overlayScroll = 0
			}
		}
		return m, nil
	case overlayAuth:
		m.pendingG = false
		switch msg.String() {
		case "esc", "q":
			m.overlay = overlayNone
			m.overlayScroll = 0
			return m, nil
		case "l":
			if m.session.Target().Kind == runtimeadapter.TargetKindEmbedded {
				m.openUpgradePrompt(deferredAction{kind: deferredAuthLogin})
				return m, nil
			}
			return m, beginAuthCmd(m.session)
		case "x":
			if m.session.Target().Kind == runtimeadapter.TargetKindEmbedded {
				m.openUpgradePrompt(deferredAction{kind: deferredAuthLogout})
				return m, nil
			}
			return m, logoutAuthCmd(m.session)
		case "o":
			if m.auth.begin != nil {
				return m, openBrowserCmd(m.auth.begin.AuthorizeURL)
			}
		case "enter":
			if m.auth.begin != nil {
				return m, finalizeAuthCmd(m.session, m.auth.begin.FlowID)
			}
		}
		return m, nil
	case overlayOperations:
		switch msg.String() {
		case "g":
			if m.pendingG {
				m.pendingG = false
				m.ops.index = 0
				return m, loadOperationsCmd(m.session, selectedOperationID(m.ops.rows, m.ops.index))
			}
			m.pendingG = true
			return m, nil
		case "G":
			m.pendingG = false
			if len(m.ops.rows) == 0 {
				return m, nil
			}
			m.ops.index = len(m.ops.rows) - 1
			return m, loadOperationsCmd(m.session, selectedOperationID(m.ops.rows, m.ops.index))
		default:
			m.pendingG = false
		}
		switch msg.String() {
		case "esc", "q":
			m.overlay = overlayNone
			m.overlayScroll = 0
			return m, nil
		case "up", "k":
			if m.ops.index > 0 {
				m.ops.index--
				return m, loadOperationsCmd(m.session, selectedOperationID(m.ops.rows, m.ops.index))
			}
		case "down", "j":
			if m.ops.index < len(m.ops.rows)-1 {
				m.ops.index++
				return m, loadOperationsCmd(m.session, selectedOperationID(m.ops.rows, m.ops.index))
			}
		case "r":
			return m, loadOperationsCmd(m.session, selectedOperationID(m.ops.rows, m.ops.index))
		case "x":
			job := selectedOperation(m.ops.rows, m.ops.index)
			if job == nil || !job.Cancellable {
				return m, nil
			}
			if m.session.Target().Kind == runtimeadapter.TargetKindEmbedded {
				m.openUpgradePrompt(deferredAction{kind: deferredOperationCancel, operationID: job.ID})
				return m, nil
			}
			return m, cancelOperationCmd(m.session, job.ID)
		}
		return m, nil
	case overlayCache:
		switch msg.String() {
		case "g":
			if m.pendingG {
				m.pendingG = false
				m.overlayScroll = 0
				return m, nil
			}
			m.pendingG = true
			return m, nil
		case "G":
			m.pendingG = false
			m.overlayScroll = viewportEndOffset()
			return m, nil
		default:
			m.pendingG = false
		}
		switch msg.String() {
		case "esc", "q", "r":
			if msg.String() == "r" {
				return m, loadCacheCmd(m.session)
			}
			m.overlay = overlayNone
			m.overlayScroll = 0
		case "pgdown", "ctrl+d", "down", "j":
			m.overlayScroll += m.scrollStep()
		case "pgup", "ctrl+u", "up", "k":
			m.overlayScroll -= m.scrollStep()
			if m.overlayScroll < 0 {
				m.overlayScroll = 0
			}
		}
		return m, nil
	case overlayLogs:
		switch msg.String() {
		case "g":
			if m.pendingG {
				m.pendingG = false
				m.overlayScroll = 0
				return m, nil
			}
			m.pendingG = true
			return m, nil
		case "G":
			m.pendingG = false
			m.overlayScroll = viewportEndOffset()
			return m, nil
		default:
			m.pendingG = false
		}
		switch msg.String() {
		case "esc", "q":
			m.overlay = overlayNone
			m.overlayScroll = 0
			return m, nil
		case "r":
			return m, loadLogsCmd(m.session, m.logs)
		case "pgdown", "ctrl+d", "down", "j":
			m.overlayScroll += m.scrollStep()
		case "pgup", "ctrl+u", "up", "k":
			m.overlayScroll -= m.scrollStep()
			if m.overlayScroll < 0 {
				m.overlayScroll = 0
			}
		}
		return m, nil
	case overlayServer:
		m.pendingG = false
		switch msg.String() {
		case "esc", "q":
			m.overlay = overlayNone
			m.overlayScroll = 0
			return m, nil
		case "s", "enter":
			if m.session.Target().Kind == runtimeadapter.TargetKindEmbedded {
				m.openUpgradePrompt(deferredAction{kind: deferredSwitchServer})
			}
			return m, nil
		}
		return m, nil
	case overlayPrompt:
		m.pendingG = false
		switch msg.String() {
		case "esc", "q":
			m.overlay = overlayNone
			m.overlayScroll = 0
			m.deferred = deferredAction{}
			return m, nil
		case "enter", "y":
			return m, upgradeSessionCmd(m.session)
		case "n":
			m.overlay = overlayNone
			m.deferred = deferredAction{}
			return m, nil
		}
		return m, nil
	case overlayBuildFilters:
		m.pendingG = false
		return m.updateBuildFilterForm(msg)
	case overlayReleaseFilters:
		m.pendingG = false
		return m.updateReleaseFilterForm(msg)
	case overlayBuildTrigger:
		m.pendingG = false
		return m.updateBuildTriggerForm(msg)
	case overlayPackageFilters:
		m.pendingG = false
		return m.updatePackageFilterForm(msg)
	case overlayBugFilters:
		m.pendingG = false
		return m.updateBugFilterForm(msg)
	case overlayReviewFilters:
		m.pendingG = false
		return m.updateReviewFilterForm(msg)
	case overlayCommitFilters:
		m.pendingG = false
		return m.updateCommitFilterForm(msg)
	case overlayProjectFilters:
		m.pendingG = false
		return m.updateProjectFilterForm(msg)
	}
	return m, nil
}

func (m *rootModel) openUpgradePrompt(action deferredAction) {
	m.deferred = action
	m.prompt = promptModel{
		title:  "Switch to local daemon?",
		body:   "This action needs durable state. Press Enter to switch, or Esc to stay embedded.",
		accept: "Switch",
		reject: "Stay embedded",
	}
	m.overlay = overlayPrompt
	m.overlayScroll = 0
}

func (m rootModel) resumeDeferredAction(action deferredAction) tea.Cmd {
	switch action.kind {
	case deferredAuthLogin:
		return beginAuthCmd(m.session)
	case deferredAuthLogout:
		return logoutAuthCmd(m.session)
	case deferredOperationCancel:
		return cancelOperationCmd(m.session, action.operationID)
	case deferredBuildTrigger:
		return triggerBuildCmd(m.session, action.buildReq)
	case deferredSwitchServer:
		return loadLocalServerStatusCmd(m.session)
	default:
		return nil
	}
}

func (m *rootModel) setToast(message, level string) {
	m.toast = toastState{message: message, level: level}
}

func (m rootModel) View() string {
	if m.width == 0 {
		m.width = 120
	}
	if m.height == 0 {
		m.height = 40
	}

	header := m.renderHeader()
	tabs := m.renderTabs()
	content := m.renderContent()
	status := m.renderStatusBar()

	bodyHeight := m.height - 4
	if bodyHeight < 10 {
		bodyHeight = 10
	}
	content = renderViewport(content, bodyHeight, m.contentScroll)
	base := lipgloss.JoinVertical(lipgloss.Left, header, tabs, content, status)
	if m.overlay != overlayNone {
		return m.renderOverlay(base)
	}
	return base
}

func (m rootModel) renderHeader() string {
	target := m.session.Target()
	authText := "LP: not authenticated"
	if m.dashboard.auth != nil && m.dashboard.auth.Launchpad.Authenticated {
		authText = "LP: " + displayLaunchpadName(m.dashboard.auth)
	}
	left := m.theme.header.Render("watchtower-tui") + " " + m.theme.badge.Render(string(target.Kind))
	if target.Address != "" {
		left += " " + m.theme.metadata.Render(target.Address)
	}
	right := m.theme.subtle.Render(authText)
	if !m.lastRefresh.IsZero() {
		right += "  " + m.theme.subtle.Render("Refreshed "+m.lastRefresh.Format("15:04:05"))
	}
	if m.toast.message != "" {
		right += "  " + renderToast(m.theme, m.toast)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer(maxInt(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))), right)
}

func (m rootModel) renderTabs() string {
	tabs := []string{
		m.renderTab("1 Dashboard", m.activeView == viewDashboard),
		m.renderTab("2 Builds", m.activeView == viewBuilds),
		m.renderTab("3 Releases", m.activeView == viewReleases),
		m.renderTab("4 Packages", m.activeView == viewPackages),
		m.renderTab("5 Bugs", m.activeView == viewBugs),
		m.renderTab("6 Reviews", m.activeView == viewReviews),
		m.renderTab("7 Commits", m.activeView == viewCommits),
		m.renderTab("8 Projects", m.activeView == viewProjects),
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

func (m rootModel) renderTab(label string, active bool) string {
	if active {
		return m.theme.tabActive.Render(label)
	}
	return m.theme.tab.Render(label)
}

func (m rootModel) renderContent() string {
	switch m.activeView {
	case viewDashboard:
		return m.renderDashboard()
	case viewBuilds:
		return m.renderBuilds()
	case viewReleases:
		return m.renderReleases()
	case viewPackages:
		return m.renderPackages()
	case viewBugs:
		return m.renderBugs()
	case viewReviews:
		return m.renderReviews()
	case viewCommits:
		return m.renderCommits()
	case viewProjects:
		return m.renderProjects()
	default:
		return ""
	}
}

func (m rootModel) renderStatusBar() string {
	target := strings.ToUpper(string(m.session.Target().Kind))
	auth := "guest"
	if m.dashboard.auth != nil && m.dashboard.auth.Launchpad.Authenticated {
		auth = displayLaunchpadName(m.dashboard.auth)
	}
	runningOps := countRunningOperations(m.dashboard.ops)
	left := fmt.Sprintf("Mode %s  LP %s  Ops %d/%d", target, auth, runningOps, len(m.dashboard.ops))
	right := "m Meta  a Auth  o Ops  c Cache  l Logs  s Server  r Refresh  q Quit"
	return m.theme.statusBar.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Top,
			m.theme.statusLeft.Render(left),
			spacer(maxInt(1, m.width-lipgloss.Width(left)-lipgloss.Width(right)-2)),
			m.theme.statusRight.Render(right),
		),
	)
}

func (m rootModel) renderDashboard() string {
	const gap = 1
	sections := []string{
		m.renderDashboardSection(0, "Runtime / Auth", m.renderDashboardRuntime(), dashboardSectionWidth(m.width, false)),
		m.renderDashboardSection(1, "Active Operations", m.renderDashboardOperations(), dashboardSectionWidth(m.width, false)),
		m.renderDashboardSection(2, "Recent Builds", m.renderDashboardBuilds(), dashboardSectionWidth(m.width, false)),
		m.renderDashboardSection(3, "Release Cache Freshness", m.renderDashboardReleases(), dashboardSectionWidth(m.width, false)),
	}
	if m.width >= 120 {
		colWidth := dashboardSectionWidth(m.width, true)
		sections[0] = m.renderDashboardSection(0, "Runtime / Auth", m.renderDashboardRuntime(), colWidth)
		sections[1] = m.renderDashboardSection(1, "Active Operations", m.renderDashboardOperations(), colWidth)
		sections[2] = m.renderDashboardSection(2, "Recent Builds", m.renderDashboardBuilds(), colWidth)
		sections[3] = m.renderDashboardSection(3, "Release Cache Freshness", m.renderDashboardReleases(), colWidth)
		left := lipgloss.JoinVertical(lipgloss.Left, sections[0], sections[1])
		right := lipgloss.JoinVertical(lipgloss.Left, sections[2], sections[3])
		return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer(gap), right)
	}
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m rootModel) renderDashboardSection(idx int, title, body string, width int) string {
	style := m.theme.panel
	if m.dashboard.section == idx {
		style = style.BorderForeground(lipgloss.Color("#7DD3FC"))
	}
	return renderPanel(style, width, m.theme.panelTitle.Render(title), body)
}

func (m rootModel) renderDashboardRuntime() string {
	target := m.session.Target()
	lines := []string{
		fmt.Sprintf("Target: %s", target.Kind),
		fmt.Sprintf("Address: %s", target.Address),
	}
	if m.dashboard.auth != nil && m.dashboard.auth.Launchpad.Authenticated {
		lines = append(lines, fmt.Sprintf("Launchpad: %s", displayLaunchpadName(m.dashboard.auth)))
	} else {
		lines = append(lines, "Launchpad: not authenticated")
	}
	return strings.Join(lines, "\n")
}

func (m rootModel) renderDashboardOperations() string {
	if len(m.dashboard.ops) == 0 {
		return m.theme.subtle.Render("No operations yet.")
	}
	lines := make([]string, 0, minInt(5, len(m.dashboard.ops)))
	for _, job := range m.dashboard.ops[:minInt(5, len(m.dashboard.ops))] {
		lines = append(lines, fmt.Sprintf("%s  %s", m.theme.semantic(string(job.State)), job.Kind))
	}
	return strings.Join(lines, "\n")
}

func (m rootModel) renderDashboardBuilds() string {
	if len(m.dashboard.builds) == 0 {
		return m.theme.subtle.Render("No builds loaded.")
	}
	lines := make([]string, 0, minInt(5, len(m.dashboard.builds)))
	for _, build := range m.dashboard.builds[:minInt(5, len(m.dashboard.builds))] {
		lines = append(lines, fmt.Sprintf("%s  %s  %s", build.State.String(), build.Project, build.Title))
	}
	return strings.Join(lines, "\n")
}

func (m rootModel) renderDashboardReleases() string {
	if m.dashboard.cache == nil || len(m.dashboard.cache.Releases.Entries) == 0 {
		return m.theme.subtle.Render("No release cache status.")
	}
	entries := append([]dto.ReleaseCacheStatus(nil), m.dashboard.cache.Releases.Entries...)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastUpdated.After(entries[j].LastUpdated)
	})
	lines := make([]string, 0, minInt(5, len(entries)))
	for _, entry := range entries[:minInt(5, len(entries))] {
		lines = append(lines, fmt.Sprintf("%s  %s  %s  %s", entry.Project, entry.ArtifactType.String(), entry.Name, entry.LastUpdated.Format("2006-01-02 15:04")))
	}
	return strings.Join(lines, "\n")
}

func (m rootModel) renderBuilds() string {
	const gap = 1
	listWidth, detailWidth := splitColumns(m.width, gap)
	header := m.theme.panelTitle.Render("Filters") + "\n" +
		fmt.Sprintf("project=%s  state=%s  active=%t  source=%s",
			emptyAsAny(m.builds.filters.project),
			emptyAsAny(m.builds.filters.state),
			m.builds.filters.active,
			emptyAsAny(m.builds.filters.source),
		)
	list := renderBuildRows(m.theme, m.builds.rows, m.builds.index, innerPanelWidth(m.theme.panel, listWidth))
	detail := renderBuildDetail(m.theme, selectedBuild(m.builds.rows, m.builds.index), innerPanelWidth(m.theme.panel, detailWidth))
	if m.width >= 120 {
		left := renderPanel(m.theme.panel, listWidth, "", header+"\n\n"+list)
		right := renderPanel(m.theme.panel, detailWidth, m.theme.panelTitle.Render("Detail"), detail)
		return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer(gap), right)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		renderPanel(m.theme.panel, m.width, "", header),
		renderPanel(m.theme.panel, m.width, m.theme.panelTitle.Render("Builds"), list),
		renderPanel(m.theme.panel, m.width, m.theme.panelTitle.Render("Detail"), detail),
	)
}

func (m rootModel) renderReleases() string {
	const gap = 1
	listWidth, detailWidth := splitColumns(m.width, gap)
	header := m.theme.panelTitle.Render("Filters") + "\n" +
		fmt.Sprintf("project=%s  type=%s  risk=%s  track=%s  branch=%s",
			emptyAsAny(m.releases.filters.project),
			emptyAsAny(m.releases.filters.artifactType),
			emptyAsAny(m.releases.filters.risk),
			emptyAsAny(m.releases.filters.track),
			emptyAsAny(m.releases.filters.branch),
		)
	list := renderReleaseArtifacts(m.theme, m.releases.artifacts, m.releases.index, innerPanelWidth(m.theme.panel, listWidth))
	detail := renderReleaseDetail(m.theme, m.releases.detail, m.selectedReleaseArtifact(), innerPanelWidth(m.theme.panel, detailWidth))
	if m.width >= 120 {
		left := renderPanel(m.theme.panel, listWidth, m.theme.panelTitle.Render("Artifacts"), header+"\n\n"+list)
		right := renderPanel(m.theme.panel, detailWidth, m.theme.panelTitle.Render("Detail"), detail)
		return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer(gap), right)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		renderPanel(m.theme.panel, m.width, "", header),
		renderPanel(m.theme.panel, m.width, m.theme.panelTitle.Render("Releases"), list),
		renderPanel(m.theme.panel, m.width, m.theme.panelTitle.Render("Detail"), detail),
	)
}

func (m rootModel) renderOverlay(base string) string {
	var content string
	fullscreen := m.width < 90 || m.overlay == overlayHelp
	switch m.overlay {
	case overlayHelp:
		content = m.renderHelp()
	case overlayAuth:
		content = m.renderAuthModal()
	case overlayOperations:
		content = m.renderOperationsDrawer(fullscreen)
	case overlayCache:
		content = m.renderCacheModal()
	case overlayLogs:
		content = m.renderLogsModal()
	case overlayServer:
		content = m.renderServerModal()
	case overlayPrompt:
		content = m.renderPrompt()
	case overlayBuildFilters:
		content = renderFormModal(m.theme, m.buildFilterForm)
	case overlayReleaseFilters:
		content = renderFormModal(m.theme, m.releaseFilterForm)
	case overlayBuildTrigger:
		content = renderFormModal(m.theme, m.buildTriggerForm)
	case overlayPackageFilters:
		content = renderFormModal(m.theme, m.packageFilterForm)
	case overlayBugFilters:
		content = renderFormModal(m.theme, m.bugFilterForm)
	case overlayReviewFilters:
		content = renderFormModal(m.theme, m.reviewFilterForm)
	case overlayCommitFilters:
		content = renderFormModal(m.theme, m.commitFilterForm)
	case overlayProjectFilters:
		content = renderFormModal(m.theme, m.projectFilterForm)
	}
	if fullscreen {
		return renderViewport(content, m.height-1, m.overlayScroll)
	}
	return lipgloss.JoinVertical(lipgloss.Left, base, renderViewport(content, maxInt(8, m.height/3), m.overlayScroll))
}

func (m rootModel) renderHelp() string {
	body := strings.Join([]string{
		"Shortcuts",
		"1..8 switch workflow",
		"Tab / Shift+Tab cycle workflows",
		"[/] cycle submodes on Packages and Commits",
		"j/k or arrows move selection",
		"gg jump to the beginning",
		"G jump to the end",
		"PgUp/PgDn or Ctrl+U/Ctrl+D scroll",
		"Enter open detail or action",
		"/ edit filters",
		"m or ? open this meta pane",
		"a auth  o operations  c cache  l logs  s server",
		"r refresh  q quit  esc close overlay",
		"",
		"Forms",
		"Up/Down cycle autocomplete suggestions",
		"Tab accepts a suggestion when shown, otherwise moves forward",
		"Shift+Tab moves backward between fields",
		"Enter moves forward or submits on the last field",
		"",
		"Embedded rule",
		"Auth, build trigger, and operation cancel prompt before switching to the local daemon.",
	}, "\n")
	return m.theme.panel.Width(m.width - 2).Height(m.height - 2).Render(m.theme.panelTitle.Render("Meta") + "\n\n" + body)
}

func (m rootModel) renderAuthModal() string {
	lines := []string{m.theme.panelTitle.Render("Auth")}
	if m.auth.err != "" {
		lines = append(lines, m.theme.errorText.Render(m.auth.err))
	}
	if m.auth.status != nil && m.auth.status.Launchpad.Authenticated {
		lines = append(lines,
			"Launchpad: "+displayLaunchpadName(m.auth.status),
			"Source: "+emptyAsDash(m.auth.status.Launchpad.Source),
			"Path: "+emptyAsDash(m.auth.status.Launchpad.CredentialsPath),
		)
	} else {
		lines = append(lines, "Launchpad: not authenticated")
	}
	if m.auth.begin != nil {
		lines = append(lines, "", "Authorize URL:", m.auth.begin.AuthorizeURL, "", "[o] open browser  [Enter] finalize")
	}
	lines = append(lines, "", "[l] login  [x] logout  [Esc] close")
	return m.theme.panel.Width(maxInt(50, m.width-4)).Render(strings.Join(lines, "\n"))
}

func (m rootModel) renderOperationsDrawer(fullscreen bool) string {
	title := m.theme.panelTitle.Render("Operations")
	rows := renderOperationRows(m.theme, m.ops.rows, m.ops.index)
	events := renderOperationEvents(m.theme, m.ops.events)
	box := m.theme.panel.Width(m.width - 2).Render(title + "\n\n" + rows + "\n\n" + m.theme.panelTitle.Render("Events") + "\n" + events + "\n\n[x] cancel  [r] refresh  [Esc] close")
	if fullscreen {
		return lipgloss.NewStyle().Height(m.height - 2).Render(box)
	}
	return box
}

func (m rootModel) renderCacheModal() string {
	lines := []string{m.theme.panelTitle.Render("Cache")}
	if m.cache.err != "" {
		lines = append(lines, m.theme.errorText.Render(m.cache.err))
	}
	if m.cache.status == nil {
		lines = append(lines, m.theme.subtle.Render("No cache status loaded."))
	} else {
		lines = append(lines,
			fmt.Sprintf("Git repos: %d", len(m.cache.status.Git.Repos)),
			fmt.Sprintf("Package sources: %d", len(m.cache.status.Packages.Sources)),
			fmt.Sprintf("Bug entries: %d", len(m.cache.status.Bugs.Entries)),
			fmt.Sprintf("Excuses entries: %d", len(m.cache.status.Excuses.Entries)),
			fmt.Sprintf("Release entries: %d", len(m.cache.status.Releases.Entries)),
		)
	}
	lines = append(lines, "", "[Esc] close")
	return m.theme.panel.Width(maxInt(50, m.width-4)).Render(strings.Join(lines, "\n"))
}

func (m rootModel) renderLogsModal() string {
	lines := []string{m.theme.panelTitle.Render("Logs")}
	if m.logsModal.err != "" {
		lines = append(lines, m.theme.errorText.Render(m.logsModal.err))
	}
	lines = append(lines, "", "Session Logs:")
	if len(m.logsModal.sessionLines) == 0 {
		lines = append(lines, m.theme.subtle.Render("(none)"))
	} else {
		lines = append(lines, m.logsModal.sessionLines...)
	}
	lines = append(lines, "")
	if m.logsModal.daemonNote != "" {
		lines = append(lines, m.theme.subtle.Render(m.logsModal.daemonNote))
	} else {
		lines = append(lines, "Daemon Logs:")
		if len(m.logsModal.daemonLines) == 0 {
			lines = append(lines, m.theme.subtle.Render("(none)"))
		} else {
			lines = append(lines, m.logsModal.daemonLines...)
		}
	}
	lines = append(lines, "", "[r] refresh  [Esc] close")
	return m.theme.panel.Width(maxInt(60, m.width-4)).Render(strings.Join(lines, "\n"))
}

func (m rootModel) renderServerModal() string {
	target := m.session.Target()
	lines := []string{
		m.theme.panelTitle.Render("Server / About"),
		fmt.Sprintf("Version: %s", Version),
		fmt.Sprintf("Target: %s", target.Kind),
		fmt.Sprintf("Address: %s", emptyAsDash(target.Address)),
	}
	if m.server.local != nil {
		lines = append(lines,
			fmt.Sprintf("Local daemon running: %t", m.server.local.Running),
			fmt.Sprintf("PID: %d", m.server.local.PID),
			fmt.Sprintf("Log file: %s", emptyAsDash(m.server.local.LogFile)),
		)
	}
	if target.Kind == runtimeadapter.TargetKindEmbedded {
		lines = append(lines, "", "[Enter] switch to local daemon")
	}
	lines = append(lines, "[Esc] close")
	return m.theme.panel.Width(maxInt(50, m.width-4)).Render(strings.Join(lines, "\n"))
}

func (m rootModel) renderPrompt() string {
	lines := []string{
		m.theme.panelTitle.Render(m.prompt.title),
		"",
		m.prompt.body,
		"",
		"[Enter] " + m.prompt.accept,
		"[Esc] " + m.prompt.reject,
	}
	return m.theme.panel.Width(maxInt(50, m.width-4)).Render(strings.Join(lines, "\n"))
}

func (m rootModel) refreshActiveView() tea.Cmd {
	switch m.activeView {
	case viewDashboard:
		return loadDashboardCmd(m.session)
	case viewBuilds:
		return loadBuildsCmd(m.session, m.builds.filters)
	case viewReleases:
		return tea.Batch(
			loadReleasesCmd(m.session, m.releases.filters),
			loadReleaseDetailCmdIfSelected(m.session, m.selectedReleaseArtifact(), m.releases.filters),
		)
	case viewPackages:
		return tea.Batch(loadPackagesCmd(m.session, m.packages.filters), loadPackageDetailCmd(m.session, m.packages))
	case viewBugs:
		return tea.Batch(loadBugsCmd(m.session, m.bugs.filters), loadBugDetailCmdIfSelected(m.session, selectedBug(m.bugs.rows, m.bugs.index)))
	case viewReviews:
		return tea.Batch(loadReviewsCmd(m.session, m.reviews.filters), loadReviewDetailCmdIfSelected(m.session, selectedReview(m.reviews.rows, m.reviews.index)))
	case viewCommits:
		return loadCommitsCmd(m.session, m.commits.filters)
	case viewProjects:
		return loadProjectsCmd(m.session, m.projects.filters)
	default:
		return nil
	}
}

func (m rootModel) jumpActiveTop() (tea.Model, tea.Cmd) {
	m.contentScroll = 0
	switch m.activeView {
	case viewDashboard:
		m.dashboard.section = 0
	case viewBuilds:
		m.builds.index = 0
	case viewReleases:
		if len(m.releases.artifacts) == 0 {
			return m, nil
		}
		m.releases.index = 0
		if artifact := m.selectedReleaseArtifact(); artifact != nil {
			return m, loadReleaseDetailCmd(m.session, *artifact, m.releases.filters)
		}
	case viewPackages:
		m.packages.index = 0
		return m, loadPackageDetailCmd(m.session, m.packages)
	case viewBugs:
		m.bugs.index = 0
		return m, loadBugDetailCmdIfSelected(m.session, selectedBug(m.bugs.rows, m.bugs.index))
	case viewReviews:
		m.reviews.index = 0
		return m, loadReviewDetailCmdIfSelected(m.session, selectedReview(m.reviews.rows, m.reviews.index))
	case viewCommits:
		m.commits.index = 0
	case viewProjects:
		m.projects.index = 0
	}
	return m, nil
}

func (m rootModel) jumpActiveBottom() (tea.Model, tea.Cmd) {
	m.contentScroll = viewportEndOffset()
	switch m.activeView {
	case viewDashboard:
		m.dashboard.section = 3
	case viewBuilds:
		if len(m.builds.rows) > 0 {
			m.builds.index = len(m.builds.rows) - 1
		}
	case viewReleases:
		if len(m.releases.artifacts) == 0 {
			return m, nil
		}
		m.releases.index = len(m.releases.artifacts) - 1
		if artifact := m.selectedReleaseArtifact(); artifact != nil {
			return m, loadReleaseDetailCmd(m.session, *artifact, m.releases.filters)
		}
	case viewPackages:
		if m.packages.rowCount() > 0 {
			m.packages.index = m.packages.rowCount() - 1
		}
		return m, loadPackageDetailCmd(m.session, m.packages)
	case viewBugs:
		if len(m.bugs.rows) > 0 {
			m.bugs.index = len(m.bugs.rows) - 1
		}
		return m, loadBugDetailCmdIfSelected(m.session, selectedBug(m.bugs.rows, m.bugs.index))
	case viewReviews:
		if len(m.reviews.rows) > 0 {
			m.reviews.index = len(m.reviews.rows) - 1
		}
		return m, loadReviewDetailCmdIfSelected(m.session, selectedReview(m.reviews.rows, m.reviews.index))
	case viewCommits:
		if len(m.commits.rows) > 0 {
			m.commits.index = len(m.commits.rows) - 1
		}
	case viewProjects:
		if len(m.projects.rows) > 0 {
			m.projects.index = len(m.projects.rows) - 1
		}
	}
	return m, nil
}

func (m rootModel) selectedReleaseArtifact() *releaseArtifactSummary {
	if m.releases.index < 0 || m.releases.index >= len(m.releases.artifacts) {
		return nil
	}
	artifact := m.releases.artifacts[m.releases.index]
	return &artifact
}

func (m rootModel) releaseDetailKey() string {
	artifact := m.selectedReleaseArtifact()
	if artifact == nil {
		return ""
	}
	return fmt.Sprintf("%s|%s|%s", artifact.Project, artifact.Name, artifact.ArtifactType.String())
}

func (m rootModel) updateBuildFilterForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cmd := updateFormModal(msg, &m.buildFilterForm, func(values []string) tea.Cmd {
		m.builds.filters = buildsFilters{
			project: strings.TrimSpace(values[0]),
			state:   strings.TrimSpace(values[1]),
			active:  strings.TrimSpace(values[2]) != "false",
			source:  defaultString(strings.TrimSpace(values[3]), "remote"),
		}
		m.overlay = overlayNone
		return loadBuildsCmd(m.session, m.builds.filters)
	}, func() {
		m.overlay = overlayNone
	})
	return m, cmd
}

func (m rootModel) updateReleaseFilterForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cmd := updateFormModal(msg, &m.releaseFilterForm, func(values []string) tea.Cmd {
		allTargets, err := strconv.ParseBool(strings.TrimSpace(values[6]))
		if err != nil {
			m.releaseFilterForm.errorMsg = "all targets must be true or false"
			return nil
		}
		m.releases.filters = releasesFilters{
			project:       strings.TrimSpace(values[0]),
			artifactType:  strings.TrimSpace(values[1]),
			risk:          strings.TrimSpace(values[2]),
			track:         strings.TrimSpace(values[3]),
			branch:        strings.TrimSpace(values[4]),
			targetProfile: strings.TrimSpace(values[5]),
			allTargets:    allTargets,
		}
		m.overlay = overlayNone
		return loadReleasesCmd(m.session, m.releases.filters)
	}, func() {
		m.overlay = overlayNone
	})
	return m, cmd
}

func (m rootModel) updateBuildTriggerForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cmd := updateFormModal(msg, &m.buildTriggerForm, func(values []string) tea.Cmd {
		req, err := buildTriggerRequestFromValues(values)
		if err != nil {
			m.buildTriggerForm.errorMsg = err.Error()
			return nil
		}
		if req.Project == "" {
			m.buildTriggerForm.errorMsg = "project is required"
			return nil
		}
		m.overlay = overlayNone
		if m.session.Target().Kind == runtimeadapter.TargetKindEmbedded {
			m.openUpgradePrompt(deferredAction{kind: deferredBuildTrigger, buildReq: req})
			return nil
		}
		return triggerBuildCmd(m.session, req)
	}, func() {
		m.overlay = overlayNone
	})
	return m, cmd
}

func buildTriggerRequestFromValues(values []string) (frontend.BuildTriggerRequest, error) {
	req := frontend.BuildTriggerRequest{
		Project:   strings.TrimSpace(values[0]),
		Artifacts: splitCSV(values[1]),
		Source:    defaultString(strings.TrimSpace(values[2]), "remote"),
		LocalPath: strings.TrimSpace(values[3]),
		Async:     true,
	}
	if req.Source != "remote" && req.Source != "local" {
		return frontend.BuildTriggerRequest{}, fmt.Errorf("source must be remote or local")
	}
	if req.Source == "local" && req.LocalPath == "" {
		return frontend.BuildTriggerRequest{}, fmt.Errorf("local path is required for local source")
	}
	return req, nil
}

func loadDashboardCmd(session *runtimeadapter.Session) tea.Cmd {
	return guardSessionAction(session, frontend.ActionDashboardRefresh, func() tea.Msg {
		ctx := context.Background()
		auth, authErr := session.Frontend.Auth().Status(ctx)
		ops, opsErr := session.Frontend.Operations().List(ctx)
		builds, buildsErr := session.Frontend.Builds().List(ctx, frontend.BuildListRequest{All: false, DefaultAll: false, Source: "remote"})
		cacheStatus, cacheErr := session.Frontend.Cache().Status(ctx)
		return dashboardLoadedMsg{
			auth:   auth,
			ops:    ops,
			builds: builds,
			cache:  cacheStatus,
			err:    errorsJoin(authErr, opsErr, buildsErr, cacheErr),
		}
	})
}

func loadBuildsCmd(session *runtimeadapter.Session, filters buildsFilters) tea.Cmd {
	return guardSessionAction(session, frontend.ActionBuildsRefresh, func() tea.Msg {
		rows, err := session.Frontend.Builds().List(context.Background(), frontend.BuildListRequest{
			Projects: firstNonEmptySlice(filters.project),
			State:    filters.state,
			All:      !filters.active,
			Source:   defaultString(filters.source, "remote"),
		})
		return buildsLoadedMsg{rows: rows, err: err}
	})
}

func loadReleasesCmd(session *runtimeadapter.Session, filters releasesFilters) tea.Cmd {
	return guardSessionAction(session, frontend.ActionReleasesRefresh, func() tea.Msg {
		rows, err := session.Frontend.Releases().List(context.Background(), frontend.ReleasesListRequest{
			Projects:      firstNonEmptySlice(filters.project),
			ArtifactType:  filters.artifactType,
			Risks:         firstNonEmptySlice(filters.risk),
			Tracks:        firstNonEmptySlice(filters.track),
			Branches:      firstNonEmptySlice(filters.branch),
			TargetProfile: filters.targetProfile,
			AllTargets:    filters.allTargets,
		})
		return releasesLoadedMsg{rows: rows, err: err}
	})
}

func loadReleaseDetailCmdIfSelected(session *runtimeadapter.Session, artifact *releaseArtifactSummary, filters releasesFilters) tea.Cmd {
	if artifact == nil {
		return nil
	}
	return loadReleaseDetailCmd(session, *artifact, filters)
}

func loadReleaseDetailCmd(session *runtimeadapter.Session, artifact releaseArtifactSummary, filters releasesFilters) tea.Cmd {
	key := fmt.Sprintf("%s|%s|%s", artifact.Project, artifact.Name, artifact.ArtifactType.String())
	return guardSessionAction(session, frontend.ActionReleaseDetailRefresh, func() tea.Msg {
		detail, err := session.Frontend.Releases().Show(context.Background(), frontend.ReleasesShowRequest{
			Name:          artifact.Name,
			ArtifactType:  artifact.ArtifactType.String(),
			TargetProfile: filters.targetProfile,
			AllTargets:    filters.allTargets,
		})
		return releaseDetailLoadedMsg{key: key, detail: detail, err: err}
	})
}

func loadOperationsCmd(session *runtimeadapter.Session, selectedID string) tea.Cmd {
	return guardSessionAction(session, frontend.ActionOperationsRefresh, func() tea.Msg {
		rows, err := session.Frontend.Operations().List(context.Background())
		var events []dto.OperationEvent
		if err == nil && selectedID != "" {
			events, err = session.Frontend.Operations().Events(context.Background(), selectedID)
		}
		return opsLoadedMsg{rows: rows, events: events, err: err}
	})
}

func loadAuthStatusCmd(session *runtimeadapter.Session) tea.Cmd {
	return guardSessionAction(session, frontend.ActionAuthRefresh, func() tea.Msg {
		status, err := session.Frontend.Auth().Status(context.Background())
		return authStatusLoadedMsg{status: status, err: err}
	})
}

func beginAuthCmd(session *runtimeadapter.Session) tea.Cmd {
	return guardSessionAction(session, frontend.ActionAuthLaunchpadBegin, func() tea.Msg {
		begin, err := session.Frontend.Auth().BeginLaunchpad(context.Background())
		return authBeginMsg{begin: begin, err: err}
	})
}

func finalizeAuthCmd(session *runtimeadapter.Session, flowID string) tea.Cmd {
	return guardSessionAction(session, frontend.ActionAuthLaunchpadFinalize, func() tea.Msg {
		result, err := session.Frontend.Auth().FinalizeLaunchpad(context.Background(), flowID)
		return authFinalizeMsg{result: result, err: err}
	})
}

func logoutAuthCmd(session *runtimeadapter.Session) tea.Cmd {
	return guardSessionAction(session, frontend.ActionAuthLaunchpadLogout, func() tea.Msg {
		result, err := session.Frontend.Auth().LogoutLaunchpad(context.Background())
		return authLogoutMsg{result: result, err: err}
	})
}

func loadCacheCmd(session *runtimeadapter.Session) tea.Cmd {
	return guardSessionAction(session, frontend.ActionCacheRefresh, func() tea.Msg {
		status, err := session.Frontend.Cache().Status(context.Background())
		return cacheLoadedMsg{status: status, err: err}
	})
}

func loadLogsCmd(session *runtimeadapter.Session, logs *logBuffer) tea.Cmd {
	return guardSessionAction(session, frontend.ActionLogsRefresh, func() tea.Msg {
		var sessionLines []string
		if logs != nil {
			sessionLines = logs.Snapshot()
		}
		msg := logsLoadedMsg{sessionLines: sessionLines}
		switch session.Target().Kind {
		case runtimeadapter.TargetKindDaemon:
			lines, err := session.ReadDaemonLogTail(200)
			if err != nil {
				msg.err = err
				msg.daemonNote = "Daemon log unavailable."
				return msg
			}
			msg.daemonLines = lines
		case runtimeadapter.TargetKindRemote:
			msg.daemonNote = "Remote server logs are not available locally."
		default:
			msg.daemonNote = "Daemon logs are only available when connected to the local persistent daemon."
		}
		return msg
	})
}

func loadLocalServerStatusCmd(session *runtimeadapter.Session) tea.Cmd {
	return guardSessionAction(session, frontend.ActionServerStatus, func() tea.Msg {
		status, err := session.LocalServerStatus(context.Background())
		return localServerStatusMsg{status: status, err: err}
	})
}

func triggerBuildCmd(session *runtimeadapter.Session, req frontend.BuildTriggerRequest) tea.Cmd {
	return guardSessionAction(session, frontend.ActionBuildTrigger, func() tea.Msg {
		result, err := session.Frontend.Builds().Trigger(context.Background(), req)
		if err != nil {
			return buildTriggeredMsg{err: err}
		}
		return buildTriggeredMsg{job: result.Job}
	})
}

func cancelOperationCmd(session *runtimeadapter.Session, id string) tea.Cmd {
	return guardSessionAction(session, frontend.ActionOperationCancel, func() tea.Msg {
		job, err := session.Frontend.Operations().Cancel(context.Background(), id)
		return operationCancelledMsg{job: job, err: err}
	})
}

func upgradeSessionCmd(session *runtimeadapter.Session) tea.Cmd {
	return guardSessionAction(session, frontend.ActionServerSwitchTarget, func() tea.Msg {
		return upgradedMsg{err: session.UpgradeToPersistent(context.Background())}
	})
}

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		return browserOpenedMsg{err: openBrowser(url)}
	}
}

func tickDashboardCmd() tea.Cmd {
	return tea.Tick(15*time.Second, func(t time.Time) tea.Msg { return tickDashboardMsg(t) })
}

func tickOperationsCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickOperationsMsg(t) })
}

func tickLogsCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickLogsMsg(t) })
}

func clearToastLater() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearToastMsg{} })
}

func newBuildFilterForm(filters buildsFilters) formModalModel {
	return newFormModal("Build Filters", []fieldDef{
		{placeholder: "project", value: filters.project},
		{placeholder: "state", value: filters.state},
		{placeholder: "active (true/false)", value: fmt.Sprintf("%t", filters.active)},
		{placeholder: "source (remote|local)", value: filters.source},
	})
}

func newReleaseFilterForm(session *runtimeadapter.Session, releases releasesModel) formModalModel {
	suggestions := releaseFilterSuggestions(session, releases)
	return newFormModal("Release Filters", []fieldDef{
		{placeholder: "project", value: releases.filters.project, suggestions: suggestions.projects},
		{placeholder: "artifact type", value: releases.filters.artifactType, suggestions: suggestions.artifactTypes},
		{placeholder: "risk", value: releases.filters.risk, suggestions: suggestions.risks},
		{placeholder: "track", value: releases.filters.track, suggestions: suggestions.tracks},
		{placeholder: "branch", value: releases.filters.branch, suggestions: suggestions.branches},
		{placeholder: "target profile", value: releases.filters.targetProfile, suggestions: suggestions.targetProfiles},
		{placeholder: "all targets (true/false)", value: fmt.Sprintf("%t", releases.filters.allTargets), suggestions: []string{"false", "true"}},
	})
}

func newBuildTriggerForm(session *runtimeadapter.Session) formModalModel {
	project := ""
	if len(session.Config.Projects) > 0 {
		project = session.Config.Projects[0].Name
	}
	return newFormModal("Trigger Build", []fieldDef{
		{placeholder: "project", value: project},
		{placeholder: "artifacts (comma separated)", value: ""},
		{placeholder: "source (remote|local)", value: "remote"},
		{placeholder: "local path", value: "."},
	})
}

func releaseFilterSuggestions(session *runtimeadapter.Session, releases releasesModel) releaseFilterOptions {
	projects := []string{}
	targetProfiles := []string{}
	if session != nil {
		projects = make([]string, 0, len(session.Config.Projects))
		for _, project := range session.Config.Projects {
			projects = append(projects, project.Name)
		}
		targetProfiles = make([]string, 0, len(session.Config.Releases.TargetProfiles))
		for name := range session.Config.Releases.TargetProfiles {
			targetProfiles = append(targetProfiles, name)
		}
	}
	artifactTypes := make([]string, 0, 3+len(releases.rows)+1)
	artifactTypes = append(artifactTypes,
		dto.ArtifactRock.String(),
		dto.ArtifactCharm.String(),
		dto.ArtifactSnap.String(),
	)
	risks := make([]string, 0, len(dto.KnownReleaseRisks()))
	for _, risk := range dto.KnownReleaseRisks() {
		risks = append(risks, string(risk))
	}
	tracks := make([]string, 0, len(releases.rows))
	branches := make([]string, 0, len(releases.rows))
	for _, row := range releases.rows {
		projects = append(projects, row.Project)
		artifactTypes = append(artifactTypes, row.ArtifactType.String())
		risks = append(risks, string(row.Risk))
		tracks = append(tracks, row.Track)
		branches = append(branches, row.Branch)
	}
	projects = append(projects, releases.filters.project)
	artifactTypes = append(artifactTypes, releases.filters.artifactType)
	risks = append(risks, releases.filters.risk)
	tracks = append(tracks, releases.filters.track)
	branches = append(branches, releases.filters.branch)
	targetProfiles = append(targetProfiles, releases.filters.targetProfile)
	return releaseFilterOptions{
		projects:       uniqueSortedStrings(projects...),
		artifactTypes:  uniqueSortedStrings(artifactTypes...),
		risks:          orderedReleaseRiskSuggestions(risks...),
		tracks:         uniqueSortedStrings(tracks...),
		branches:       uniqueSortedStrings(branches...),
		targetProfiles: uniqueSortedStrings(targetProfiles...),
	}
}

type fieldDef struct {
	placeholder string
	value       string
	suggestions []string
}

func newFormModal(title string, fields []fieldDef) formModalModel {
	models := make([]textinput.Model, 0, len(fields))
	for i, field := range fields {
		input := textinput.New()
		input.Placeholder = field.placeholder
		input.SetValue(field.value)
		if len(field.suggestions) > 0 {
			input.ShowSuggestions = true
			input.SetSuggestions(field.suggestions)
		}
		if i == 0 {
			input.Focus()
		}
		models = append(models, input)
	}
	return formModalModel{
		title:  title,
		fields: models,
	}
}

func updateFormModal(msg tea.KeyMsg, modal *formModalModel, onSubmit func([]string) tea.Cmd, onCancel func()) tea.Cmd {
	switch msg.String() {
	case "esc", "q":
		onCancel()
		return nil
	case "tab":
		if acceptFormSuggestion(modal) {
			return nil
		}
		if modal.active == len(modal.fields)-1 {
			values := make([]string, 0, len(modal.fields))
			for _, field := range modal.fields {
				values = append(values, field.Value())
			}
			return onSubmit(values)
		}
		modal.fields[modal.active].Blur()
		modal.active++
		modal.fields[modal.active].Focus()
		return nil
	case "enter":
		if modal.active == len(modal.fields)-1 {
			values := make([]string, 0, len(modal.fields))
			for _, field := range modal.fields {
				values = append(values, field.Value())
			}
			return onSubmit(values)
		}
		modal.fields[modal.active].Blur()
		modal.active++
		modal.fields[modal.active].Focus()
		return nil
	case "shift+tab":
		if modal.active > 0 {
			modal.fields[modal.active].Blur()
			modal.active--
			modal.fields[modal.active].Focus()
		}
		return nil
	}
	var cmd tea.Cmd
	modal.fields[modal.active], cmd = modal.fields[modal.active].Update(msg)
	return cmd
}

func acceptFormSuggestion(modal *formModalModel) bool {
	if modal.active < 0 || modal.active >= len(modal.fields) {
		return false
	}
	field := &modal.fields[modal.active]
	suggestion := strings.TrimSpace(field.CurrentSuggestion())
	if suggestion == "" || suggestion == field.Value() {
		return false
	}
	field.SetValue(suggestion)
	field.CursorEnd()
	return true
}

func renderFormModal(t theme, modal formModalModel) string {
	lines := []string{t.panelTitle.Render(modal.title)}
	for i, field := range modal.fields {
		label := field.Placeholder
		value := field.View()
		style := t.input
		if i == modal.active {
			style = t.inputFocused
		}
		lines = append(lines, label, style.Render(value))
	}
	if modal.errorMsg != "" {
		lines = append(lines, t.errorText.Render(modal.errorMsg))
	}
	lines = append(lines, "", "[Up/Down] suggestions  [Tab] accept/next  [Enter] next/submit  [Esc] cancel")
	return t.panel.Width(70).Render(strings.Join(lines, "\n"))
}

func renderBuildRows(t theme, rows []dto.Build, selected int, width int) string {
	if len(rows) == 0 {
		return t.subtle.Render("No builds.")
	}
	lines := make([]string, 0, len(rows))
	for i, row := range rows {
		line := fitLine(fmt.Sprintf("%-10s  %-12s  %s", row.State.String(), row.Project, row.Title), width)
		if i == selected {
			line = t.selectedRow.Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func renderBuildDetail(t theme, build *dto.Build, width int) string {
	if build == nil {
		return t.subtle.Render("No build selected.")
	}
	return fitBlock(strings.Join([]string{
		"Project: " + build.Project,
		"Recipe: " + build.Recipe,
		"Title: " + build.Title,
		"State: " + build.State.String(),
		"Arch: " + build.Arch,
		"Created: " + build.CreatedAt.Format(time.RFC3339),
		"Started: " + emptyTime(build.StartedAt),
		"Built: " + emptyTime(build.BuiltAt),
		"Web: " + build.WebLink,
		"Log: " + emptyAsDash(build.BuildLogURL),
		fmt.Sprintf("Can retry: %t", build.CanRetry),
		fmt.Sprintf("Can cancel: %t", build.CanCancel),
		"",
		"[t] trigger async build",
	}, "\n"), width)
}

func renderReleaseArtifacts(t theme, artifacts []releaseArtifactSummary, selected int, width int) string {
	if len(artifacts) == 0 {
		return t.subtle.Render("No artifacts.")
	}
	const (
		gapWidth       = 2
		typeColWidth   = 5
		targetColWidth = 28
		dateColWidth   = 16
		minNameWidth   = 10
		maxNameWidth   = 28
	)
	lines := make([]string, 0, len(artifacts))
	nameColWidth := width - typeColWidth - targetColWidth - dateColWidth - (gapWidth * 3)
	if nameColWidth < minNameWidth {
		nameColWidth = minNameWidth
	}
	if nameColWidth > maxNameWidth {
		nameColWidth = maxNameWidth
	}
	for i, artifact := range artifacts {
		released := formatListTime(artifact.ReleasedAt)
		latestTarget := emptyAsDash(artifact.LatestVisibleTarget)
		line := padRight(truncateToWidth(artifact.Name, nameColWidth), nameColWidth) +
			spacer(gapWidth) +
			padRight(artifact.ArtifactType.String(), typeColWidth) +
			spacer(gapWidth) +
			padRight(truncateToWidth(latestTarget, targetColWidth), targetColWidth) +
			spacer(gapWidth) +
			padRight(released, dateColWidth)
		if i == selected {
			line = t.selectedRow.Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func renderReleaseDetail(t theme, detail *dto.ReleaseShowResult, selected *releaseArtifactSummary, width int) string {
	if detail == nil || selected == nil {
		return t.subtle.Render("Select an artifact to load its full release matrix.")
	}
	releasedAt := latestReleaseTime(detail)
	lines := []string{
		"Project: " + detail.Project,
		"Name: " + detail.Name,
		"Type: " + detail.ArtifactType.String(),
		"Released: " + emptyTime(releasedAt),
		"",
		"Tracks: " + emptyAsDash(strings.Join(detail.Tracks, ", ")),
		"",
		"Channels:",
	}
	for _, channel := range detail.Channels {
		channelReleased := latestChannelReleaseTime(channel)
		lines = append(lines,
			fmt.Sprintf("- %s", channel.Channel),
			fmt.Sprintf("  track=%s  risk=%s  branch=%s  released=%s", channel.Track, channel.Risk, emptyAsDash(channel.Branch), emptyTime(channelReleased)),
			fmt.Sprintf("  targets: %s", formatReleaseTargets(channel.Targets)),
			fmt.Sprintf("  resources: %s", formatReleaseResources(channel.Resources)),
			"",
		)
	}
	return fitBlock(strings.Join(lines, "\n"), width)
}

func renderOperationRows(t theme, rows []dto.OperationJob, selected int) string {
	if len(rows) == 0 {
		return t.subtle.Render("No operations.")
	}
	lines := make([]string, 0, len(rows))
	for i, row := range rows {
		line := fmt.Sprintf("%-12s  %-12s  %s", row.State, row.Kind, row.Summary)
		if i == selected {
			line = t.selectedRow.Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func renderOperationEvents(t theme, events []dto.OperationEvent) string {
	if len(events) == 0 {
		return t.subtle.Render("No events.")
	}
	lines := make([]string, 0, minInt(8, len(events)))
	for _, event := range events[maxInt(0, len(events)-8):] {
		lines = append(lines, fmt.Sprintf("%s  %s", event.Time.Format("15:04:05"), defaultString(event.Message, event.Type)))
	}
	return strings.Join(lines, "\n")
}

func summarizeReleaseArtifacts(rows []dto.ReleaseListEntry) []releaseArtifactSummary {
	type key struct {
		project string
		name    string
		kind    dto.ArtifactType
	}
	grouped := make(map[key]releaseArtifactSummary, len(rows))
	latestTargetAt := make(map[key]time.Time, len(rows))
	for _, row := range rows {
		k := key{project: row.Project, name: row.Name, kind: row.ArtifactType}
		summary := grouped[k]
		summary.Project = row.Project
		summary.Name = row.Name
		summary.ArtifactType = row.ArtifactType
		summary.ChannelCount++
		summary.ResourceCount += len(row.Resources)
		if row.ReleasedAt.After(summary.ReleasedAt) {
			summary.ReleasedAt = row.ReleasedAt
		}
		summary.VisibleTargetCount += len(row.Targets)
		for _, target := range row.Targets {
			if target.ReleasedAt.After(latestTargetAt[k]) || summary.LatestVisibleTarget == "" {
				latestTargetAt[k] = target.ReleasedAt
				summary.LatestVisibleTarget = frontend.FormatReleaseTargetCompact(target)
			}
		}
		grouped[k] = summary
	}
	artifacts := make([]releaseArtifactSummary, 0, len(grouped))
	for _, summary := range grouped {
		artifacts = append(artifacts, summary)
	}
	sort.Slice(artifacts, func(i, j int) bool {
		if artifacts[i].ReleasedAt.Equal(artifacts[j].ReleasedAt) {
			if artifacts[i].Project == artifacts[j].Project {
				return artifacts[i].Name < artifacts[j].Name
			}
			return artifacts[i].Project < artifacts[j].Project
		}
		return artifacts[i].ReleasedAt.After(artifacts[j].ReleasedAt)
	})
	return artifacts
}

func latestReleaseTime(detail *dto.ReleaseShowResult) time.Time {
	if detail == nil {
		return time.Time{}
	}
	var latest time.Time
	for _, channel := range detail.Channels {
		if ts := latestChannelReleaseTime(channel); ts.After(latest) {
			latest = ts
		}
	}
	return latest
}

func latestChannelReleaseTime(channel dto.ReleaseChannelSnapshot) time.Time {
	var latest time.Time
	for _, target := range channel.Targets {
		if target.ReleasedAt.After(latest) {
			latest = target.ReleasedAt
		}
	}
	return latest
}

func formatReleaseTargets(targets []dto.ReleaseTargetSnapshot) string {
	return frontend.FormatReleaseTargets(targets)
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

func dashboardSectionWidth(totalWidth int, twoColumns bool) int {
	if !twoColumns {
		return totalWidth
	}
	const gap = 1
	return maxInt(20, (totalWidth-gap)/2)
}

func splitColumns(totalWidth, gap int) (int, int) {
	left := maxInt(20, (totalWidth-gap)/2)
	right := maxInt(20, totalWidth-gap-left)
	return left, right
}

func renderPanel(style lipgloss.Style, totalWidth int, title, body string) string {
	if totalWidth < 8 {
		totalWidth = 8
	}
	innerWidth := innerPanelWidth(style, totalWidth)
	content := fitBlock(body, innerWidth)
	if title != "" {
		content = title + "\n" + content
	}
	return style.Width(innerWidth).MaxWidth(totalWidth).Render(content)
}

func innerPanelWidth(style lipgloss.Style, totalWidth int) int {
	innerWidth := totalWidth - style.GetHorizontalFrameSize()
	if innerWidth < 1 {
		return 1
	}
	return innerWidth
}

func fitBlock(text string, width int) string {
	if width <= 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	fit := make([]string, 0, len(lines))
	for _, line := range lines {
		fit = append(fit, fitLine(line, width))
	}
	return strings.Join(fit, "\n")
}

func fitLine(text string, width int) string {
	if width <= 0 {
		return text
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width == 1 {
		return "…"
	}
	runes := []rune(text)
	out := make([]rune, 0, len(runes))
	for _, r := range runes {
		candidate := string(append(out, r))
		if lipgloss.Width(candidate+"…") > width {
			break
		}
		out = append(out, r)
	}
	return string(out) + "…"
}

func truncateToWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width == 1 {
		return "…"
	}
	runes := []rune(text)
	out := make([]rune, 0, len(runes))
	for _, r := range runes {
		candidate := string(append(out, r))
		if lipgloss.Width(candidate+"…") > width {
			break
		}
		out = append(out, r)
	}
	return string(out) + "…"
}

func padRight(text string, width int) string {
	padding := width - lipgloss.Width(text)
	if padding <= 0 {
		return text
	}
	return text + spacer(padding)
}

func renderViewport(content string, height, offset int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) <= height {
		return lipgloss.NewStyle().Height(height).Render(strings.Join(lines, "\n"))
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(lines)-height {
		offset = len(lines) - height
	}
	end := offset + height
	return lipgloss.NewStyle().Height(height).Render(strings.Join(lines[offset:end], "\n"))
}

func viewportEndOffset() int {
	return 1 << 30
}

func (m rootModel) scrollStep() int {
	if m.height <= 0 {
		return 5
	}
	step := m.height / 4
	if step < 3 {
		return 3
	}
	return step
}

func selectedBuild(rows []dto.Build, idx int) *dto.Build {
	if idx < 0 || idx >= len(rows) {
		return nil
	}
	row := rows[idx]
	return &row
}

func selectedOperation(rows []dto.OperationJob, idx int) *dto.OperationJob {
	if idx < 0 || idx >= len(rows) {
		return nil
	}
	row := rows[idx]
	return &row
}

func selectedOperationID(rows []dto.OperationJob, idx int) string {
	if job := selectedOperation(rows, idx); job != nil {
		return job.ID
	}
	return ""
}

func countRunningOperations(rows []dto.OperationJob) int {
	count := 0
	for _, row := range rows {
		if row.State == dto.OperationStateQueued || row.State == dto.OperationStateRunning {
			count++
		}
	}
	return count
}

func hasRunningOperation(rows []dto.OperationJob) bool {
	return countRunningOperations(rows) > 0
}

func displayLaunchpadName(status *dto.AuthStatus) string {
	if status == nil {
		return "guest"
	}
	if status.Launchpad.DisplayName != "" {
		return status.Launchpad.DisplayName
	}
	if status.Launchpad.Username != "" {
		return status.Launchpad.Username
	}
	return "guest"
}

func renderToast(t theme, toast toastState) string {
	switch toast.level {
	case "success":
		return t.success.Render(toast.message)
	case "error":
		return t.errorText.Render(toast.message)
	default:
		return t.info.Render(toast.message)
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		if _, err := exec.LookPath("xdg-open"); err == nil {
			cmd = exec.Command("xdg-open", url)
		} else if _, err := exec.LookPath("wslview"); err == nil {
			cmd = exec.Command("wslview", url)
		} else {
			return fmt.Errorf("no browser opener available")
		}
	}
	return cmd.Start()
}

func emptyAsAny(value string) string {
	if strings.TrimSpace(value) == "" {
		return "any"
	}
	return value
}

func emptyAsDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func uniqueSortedStrings(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func orderedReleaseRiskSuggestions(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(dto.KnownReleaseRisks()))
	for _, risk := range dto.KnownReleaseRisks() {
		value := string(risk)
		seen[value] = struct{}{}
		out = append(out, value)
	}
	for _, value := range uniqueSortedStrings(values...) {
		if _, ok := seen[value]; ok {
			continue
		}
		out = append(out, value)
	}
	return out
}

func emptyTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format(time.RFC3339)
}

func formatListTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04")
}

func firstNonEmptySlice(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return []string{value}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func errorsJoin(errs ...error) error {
	var parts []string
	for _, err := range errs {
		if err != nil {
			parts = append(parts, err.Error())
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return errors.New(strings.Join(parts, "; "))
}

func spacer(width int) string {
	if width <= 0 {
		return ""
	}
	return strings.Repeat(" ", width)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
