# Sunbeam Watchtower

A unified CLI dashboard for tracking code, reviews, bugs, and builds across GitHub, Launchpad, and Gerrit forges.

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

2. Sync the local git cache (clones repos for fast commit lookups):

```bash
watchtower cache sync
```

3. Start exploring:

```bash
watchtower commit log
watchtower review list
watchtower bug list
```

## Configuration

The config file lives at `~/.config/sunbeam-watchtower/config.yaml` by default. Override with `--config <path>`.

### Top-level sections

| Section     | Description |
|-------------|-------------|
| `launchpad` | Launchpad settings (`default_owner`, `use_keyring`) |
| `github`    | GitHub settings (`use_keyring`) |
| `gerrit`    | Gerrit settings (`hosts` list with `url` entries) |
| `projects`  | List of tracked projects |
| `build`     | Build pipeline settings (`default_prefix`, `timeout_minutes`, `artifacts_dir`) |

### Project configuration

Each project has:

```yaml
- name: my-project              # required, unique identifier
  artifact_type: rock            # optional: rock, charm, or snap (required if build is set)
  code:
    forge: github                # required: github, gerrit, or launchpad
    owner: my-org                # required for github
    host: https://review.dev.org # required for gerrit
    project: my-project          # required: project name/path
    git_url: https://custom/repo # optional: explicit clone URL override
  bugs:
    - forge: launchpad
      project: my-project
  build:                         # optional
    owner: my-team
    recipes:
      - recipe-name
    prepare_command: make prepare
```

The `git_url` field is useful when the clone URL cannot be derived from the forge type and project name (e.g., Launchpad repos with complex paths like `~owner/project/+git/repo`).

## Commands

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

### `watchtower cache`

Manage the local git repo cache used by `commit log` and `commit track`.

```bash
# Clone missing repos and fetch all existing ones
watchtower cache sync

# Sync a specific project only
watchtower cache sync --project snap-openstack

# Show cached repos and their disk usage
watchtower cache status

# Remove all cached repos
watchtower cache clear

# Remove a specific project's cache
watchtower cache clear --project snap-openstack
```

The cache is stored at `$XDG_CACHE_HOME/sunbeam-watchtower/repos/` (defaults to `~/.cache/sunbeam-watchtower/repos/`). Repos are bare git clones organized by host:

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

## License

Apache-2.0
