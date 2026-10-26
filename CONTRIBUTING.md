## Local development setup

### Prerequisites

Make sure you have the following dependencies installed first:

- [Git](https://git-scm.com/)
- [Go](https://golang.org/dl/) (see [go.mod](go.mod#L3) for minimum required version)
- [Mage](https://magefile.org/)
- [Node.js](https://nodejs.org) (LTS version recommended)

### Running the plugin locally

Follow these steps to set up and run the plugin in your local environment:

1. Clone this repository into your local environment. The frontend code lives in the [src](src) folder,
   alongside the [plugin.json](src/plugin.json) file. The backend Go code is in the [pkg](pkg) folder.
2. Go to the directory where you cloned this repository:
   ```shell
   cd <grafana-datasource-plugin>
   ```
3. Install the dependencies:
   ```shell
   npm install
   ```
4. Build the plugin frontend:
   ```shell
   npm run dev
   ```
5. In a new terminal window, build the plugin backend:
   ```shell
   mage -v build:linux
   ```
6. Start Grafana:
   ```shell
   docker compose up
   ```
7. Open Grafana at http://localhost:3000, and then go to **Administration** > **Plugins**. Make sure that
   this plugin is there.

### Git hooks

Checks run through [pre-commit](https://pre-commit.com/) so most CI failures show up before a push.
Install it once (for example `brew install pre-commit`), then enable the hooks in your clone:

```shell
pre-commit install
```

- **On every commit** (staged files only): gofmt, golangci-lint on changed code (includes gosec), `go mod tidy`
  drift, eslint, actionlint and zizmor for workflows, shellcheck for `.github/scripts`, gitleaks for secrets
  (test fixtures are allowlisted in `.gitleaks.toml`), and file hygiene checks (merge-conflict markers, case
  conflicts, line endings, large files, private keys, YAML syntax).
- **On `git push`**: golangci-lint on the whole module, `go test -race` (needs Docker), `npm run typecheck`
  and `npm run test:ci`. Each runs only when the push touches the matching files.
- **CI only**: e2e, the production build, the plugin validator and govulncheck.

Run everything by hand with `pre-commit run --all-files`, or the push-stage checks with
`pre-commit run --all-files --hook-stage pre-push`. Skip a hook in an emergency with `git commit --no-verify`
or `git push --no-verify`. CI re-runs golangci-lint, eslint, the typecheck and the tests, but not gofmt, `go mod tidy`,
gitleaks, actionlint, zizmor or shellcheck, so run those by hand before pushing.
