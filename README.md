# Sunbeam Watchtower

A unified CLI dashboard for tracking packages, code, reviews, bugs, builds, and Launchpad project metadata across GitHub, Launchpad, and Gerrit forges.

The CLI runs on top of a local HTTP API server. The codebase now uses a hexagonal split where `internal/adapter/primary/*` drives `internal/app`, which wires `internal/core/service/*` to `internal/adapter/secondary/*` through `internal/core/port/*`. Public reusable contracts and helpers live under `pkg/`, notably `pkg/client`, `pkg/dto/v1`, `pkg/distro/v1`, `pkg/forge/v1`, and `pkg/launchpad/v1`.

## Installation

```bash
go install github.com/gboutry/sunbeam-watchtower/cmd/watchtower@latest
```

Or build from source:

```bash
git clone https://github.com/gboutry/sunbeam-watchtower.git
cd sunbeam-watchtower
go build -o watchtower ./cmd/watchtower
```

## Quick start

1. Create a configuration file at `~/.config/sunbeam-watchtower/config.yaml`:

```yaml
projects:
  - name: snap-openstack
    code:
      forge: github
      owner: canonical
      project: snap-openstack
    bugs:
      - forge: launchpad
        project: snap-openstack

  - name: sunbeam-charms
    code:
      forge: gerrit
      host: https://review.opendev.org
      project: openstack/sunbeam-charms

  - name: charm-keystone
    code:
      forge: launchpad
      project: charm-keystone-k8s
```

2. Sync the local caches:

```bash
watchtower cache sync
```

3. Start exploring:

```bash
watchtower packages diff released
watchtower commit log
watchtower review list
watchtower bug list
watchtower project sync --dry-run
```

## Architecture

- `cmd/watchtower` is a thin entrypoint into the CLI adapter.
- `internal/adapter/primary/cli` contains Cobra command logic.
- `internal/adapter/primary/api` exposes the application over HTTP/OpenAPI.
- `internal/app` is the composition root used by the primary adapters.
- `internal/core/port` contains interfaces only.
- `internal/core/service/*` contains domain logic and use cases.
- `internal/adapter/secondary/*` contains concrete integrations for git, Launchpad, caches, and OpenStack.
- `pkg/*` contains reusable client and DTO packages that can be consumed outside the repository.

## Configuration

The config file lives at `~/.config/sunbeam-watchtower/config.yaml` by default. Override with `--config <path>`.

### Top-level sections

| Section     | Description |
|-------------|-------------|
| `launchpad` | Launchpad settings (`default_owner`, `use_keyring`, default `series`, `development_focus`) |
| `github`    | GitHub settings (`use_keyring`) |
| `gerrit`    | Gerrit settings (`hosts` list with `url` entries) |
| `projects`  | List of tracked projects |
| `build`     | Build pipeline settings (`default_prefix`, `timeout_minutes`, `artifacts_dir`) |
| `packages`  | Package source, set, and upstream configuration |

### Project configuration

Each project has:

```yaml
- name: my-project              # required, unique identifier
  artifact_type: rock           # optional: rock, charm, or snap (required if build is set)
  code:
    forge: github               # required: github, gerrit, or launchpad
    owner: my-org               # required for github
    host: https://review.dev.org # required for gerrit
    project: my-project         # required: project name/path
    git_url: https://custom/repo # optional: explicit clone URL override
  bugs:
    - forge: launchpad
      project: my-project
  series:                       # optional: Launchpad series to ensure on the project
    - 2024.1
    - 2024.2
  development_focus: 2024.2     # optional: must be one of the declared series
  build:                        # optional
    owner: my-team
    recipes:
      - recipe-name
    prepare_command: make prepare
```

The `git_url` field is useful when the clone URL cannot be derived from the forge type and project name (e.g., Launchpad repos with complex paths like `~owner/project/+git/repo`).

Projects can declare the Launchpad series that must exist on the project, plus the desired focus of development:

```yaml
- name: snap-openstack
  code:
    forge: github
    owner: canonical
    project: snap-openstack
  bugs:
    - forge: launchpad
      project: snap-openstack
  series:
    - 2024.1
    - 2024.2
  development_focus: 2024.2
```

`watchtower project sync` ensures those series exist and sets the declared focus of development.

## Commands

### `watchtower packages`

Compare package versions and inspect package metadata across configured distro sources.

```bash
# Compare package sets
watchtower packages diff released

# Show all versions of a package
watchtower packages show nova

# List packages in a configured distro release
watchtower packages list --distro ubuntu --release noble
```

### `watchtower commit`

Read commit history from the local git cache.

```bash
# List recent commits across all projects
watchtower commit log

# Filter by project, forge, branch, or author
watchtower commit log --project snap-openstack --branch main --author alice

# Find commits referencing a Launchpad bug
watchtower commit track --bug-id 12345
```

### `watchtower review`

List and inspect merge requests across forges.

```bash
# List open merge requests
watchtower review list --state open

# Filter by project or forge
watchtower review list --project snap-openstack --forge github

# Show details for a specific merge request
watchtower review show --project snap-openstack "#42"
```

### `watchtower bug`

Track bugs across Launchpad bug trackers.

```bash
# List bug tasks
watchtower bug list

# Filter by status, importance, assignee, or tag
watchtower bug list --status "In Progress" --importance High --assignee alice

# Show a specific bug with its tasks
watchtower bug show 12345
```

### `watchtower build`

Manage Launchpad builds (rocks, charms, snaps).

```bash
# Trigger builds for a project
watchtower build trigger my-project

# Trigger and wait for completion
watchtower build trigger my-project --wait --timeout 2h

# Trigger from a local git repo
watchtower build trigger my-project --source local --local-path ./my-repo

# List builds
watchtower build list --project my-project

# Download build artifacts
watchtower build download my-project --artifacts-dir ./output

# Clean up temporary recipes
watchtower build cleanup --project my-project --dry-run
```

### `watchtower project`

Manage Launchpad project metadata derived from configuration.

```bash
# Preview Launchpad project changes
watchtower project sync --dry-run

# Sync a subset of Launchpad projects
watchtower project sync --project snap-openstack --project sunbeam-charms
```

### `watchtower cache`

Manage local caches used by commit tracking, package lookups, upstream repository inspection, and bug tracking.

```bash
# Sync all cache types
watchtower cache sync

# Sync only git repos
watchtower cache sync git

# Sync only package indexes for selected distros/releases
watchtower cache sync packages-index --distro ubuntu --release noble

# Sync only configured upstream repos
watchtower cache sync upstream-repos

# Sync only bug/task caches
watchtower cache sync bugs

# Sync a specific project's git cache
watchtower cache sync git --project snap-openstack

# Show cached repos and their disk usage
watchtower cache status

# Remove all cached data
watchtower cache clear

# Remove a specific project's git cache
watchtower cache clear git --project snap-openstack
```

Valid cache types are `git`, `packages-index`, `upstream-repos`, and `bugs`.

Git cache data is stored at `$XDG_CACHE_HOME/sunbeam-watchtower/repos/` (defaults to `~/.cache/sunbeam-watchtower/repos/`). Repos are bare git clones organized by host:

```
~/.cache/sunbeam-watchtower/repos/
├── github.com/canonical/snap-openstack.git
├── review.opendev.org/openstack/sunbeam-charms.git
└── git.launchpad.net/charm-keystone-k8s.git
```

### `watchtower auth`

Manage Launchpad authentication. Required for bug tracking, reviews on Launchpad, and builds.

```bash
# Interactive browser-based OAuth login
watchtower auth login

# Check authentication status
watchtower auth status
```

### `watchtower config`

```bash
# Display the current configuration
watchtower config show
```

### `watchtower serve`

Run the HTTP API server directly instead of using the embedded per-command server.

```bash
# Serve on TCP
watchtower serve --listen 127.0.0.1:8472

# Serve on a Unix domain socket
watchtower serve --listen unix:///tmp/watchtower.sock
```

When served over TCP:

- OpenAPI spec: `http://127.0.0.1:8472/openapi.json`
- Interactive docs: `http://127.0.0.1:8472/docs`

## Output formats

All list/show commands support `--output` (`-o`):

| Format  | Flag            | Description |
|---------|-----------------|-------------|
| Table   | `-o table`      | Human-readable aligned columns (default) |
| JSON    | `-o json`       | Machine-readable JSON |
| YAML    | `-o yaml`       | Machine-readable YAML |

## Global flags

| Flag        | Description |
|-------------|-------------|
| `--config`  | Path to config file (default: `~/.config/sunbeam-watchtower/config.yaml`) |
| `--verbose` | Enable debug logging (written to stderr) |
| `-o, --output` | Output format: `table`, `json`, `yaml` |
| `--no-color` | Disable colored output |

## Debug logging

Enable verbose logging to see exactly what watchtower is doing:

```bash
watchtower --verbose commit log
```

This writes structured debug logs to stderr, including:
- Which forge clients are being configured
- Which projects are being queried or skipped by filters
- How many commits/reviews/bugs were fetched per project
- Git cache operations (clone, fetch, path resolution)
- Cache directory resolution

Example output:

```
time=... level=DEBUG msg="building commit sources" project_count=3
time=... level=DEBUG msg="resolved cache directory" path=/home/user/.cache/sunbeam-watchtower
time=... level=DEBUG msg="initializing git cache" cacheDir=/home/user/.cache/sunbeam-watchtower/repos
time=... level=DEBUG msg="configured commit source" project=snap-openstack cloneURL=https://github.com/canonical/snap-openstack.git
time=... level=DEBUG msg="listing commits" project_count=3 branch=main author=""
time=... level=DEBUG msg="querying project" project=snap-openstack forge=GitHub
time=... level=DEBUG msg="listing commits from cache" cloneURL=https://github.com/canonical/snap-openstack.git branch=main
time=... level=DEBUG msg="commits read from cache" cloneURL=https://github.com/canonical/snap-openstack.git count=47
time=... level=DEBUG msg="project commits fetched" project=snap-openstack commit_count=47
time=... level=DEBUG msg="commits aggregated" total_count=142
time=... level=DEBUG msg="commit log complete" total_commits=142
```

## Developer tooling

This repository ships with pre-commit hooks, `arch-go` architecture checks, and a curated `golangci-lint` v2 configuration.

```bash
pre-commit install
pre-commit run --all-files
```

The hooks run whitespace/YAML checks plus `arch-go`, `golangci-lint`, `go build`, `go test`, and `go mod tidy`.

## License

Apache-2.0
