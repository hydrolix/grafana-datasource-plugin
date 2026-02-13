# grafana_error_panel_cli

A CLI tool that automates the configuration of error panels in Grafana dashboards via the Grafana HTTP API. It injects a [Business Text](https://grafana.com/grafana/plugins/marcusolsson-dynamictext-panel/) (Dynamic Text) panel into existing dashboards that displays query errors with expandable details and actionable solution suggestions.

## How It Works

The tool creates a repeating row panel backed by an [Infinity datasource](https://grafana.com/grafana/plugins/yesoreyeram-infinity-datasource/) that reads error data from a Grafana dashboard variable (`hdx_query_errors`). Errors are rendered in a table with timestamps, messages, and expandable "show more" details. When an error matches a known pattern (e.g. `SYNTAX_ERROR`, `TIMEOUT_EXCEEDED`, `ACCESS_DENIED`), the panel displays a tailored "How to fix" guide populated with context from the error itself.

```
CLI reads templates -> builds panel config -> pushes to Grafana API -> panel renders errors in dashboard
```

## Prerequisites

- Python 3.7+
- A running Grafana instance with API access
- The [Infinity datasource plugin](https://grafana.com/grafana/plugins/yesoreyeram-infinity-datasource/) installed in Grafana
- The [Business Text panel plugin](https://grafana.com/grafana/plugins/marcusolsson-dynamictext-panel/) installed in Grafana

## Installation

```bash
cd error_panel
pip install -r requirements.txt
```

Or install as a package:

```bash
pip install -e .
```

This registers the `grafana-error-panel` command.

## Quick Start

Run the interactive wizard (the default command):

```bash
python grafana_error_panel_cli.py
```

The wizard walks you through:

1. Connecting to Grafana (URL, username, password)
2. Selecting an organization
3. Selecting an existing dashboard
4. Choosing or creating an Infinity datasource
5. Applying the error panel configuration

## Commands

| Command | Description |
|---|---|
| `wizard` | Interactive wizard (runs by default) |
| `login` | Save Grafana credentials for reuse |
| `logout` | Clear stored credentials |
| `status` | Show current credential status |
| `setup-datasource` | Create the Infinity datasource |
| `configure-dashboard` | Configure a dashboard with error panels (non-interactive) |
| `show-dashboard` | Dump a dashboard's JSON configuration |
| `example-config` | Print the generated panel/variable/row JSON |

### Examples

```bash
# Interactive wizard
python grafana_error_panel_cli.py

# Save credentials
python grafana_error_panel_cli.py login

# Non-interactive: create datasource then configure dashboard
python grafana_error_panel_cli.py setup-datasource \
  --url http://localhost:3000 \
  --username admin \
  --password admin

python grafana_error_panel_cli.py configure-dashboard \
  --dashboard-uid abc123 \
  --datasource-name static

# Inspect a dashboard
python grafana_error_panel_cli.py show-dashboard --dashboard-uid abc123

# View generated panel config
python grafana_error_panel_cli.py example-config
```

## Credential Storage

Credentials are saved to `~/.grafana_error_panel/config.json` with `0600` permissions. Use `login` to store them and `logout` to clear them. When credentials are stored, all commands will use them automatically unless overridden with `--url`, `--username`, and `--password` flags.

## What Gets Added to the Dashboard

The tool adds three things to the target dashboard:

1. **Two hidden template variables** (`hdx_query_errors` and `hdx_query_errors_selected`) that hold error state
2. **A collapsed row** that repeats based on the error variable
3. **A Business Text panel** inside the row that renders errors using Handlebars templates with JavaScript pre/post-render hooks

## Solution Templates

The tool ships with built-in solution templates for common error types:

| Error | Description |
|---|---|
| `TIMERANGE_EXCEEDED` | Query time range exceeds the maximum allowed window |
| `NETWORK_ERROR` | Connection interrupted (broken pipe) |
| `SYNTAX_ERROR` | SQL syntax errors and macro issues |
| `ACCESS_DENIED` | Insufficient permissions |
| `QUERY_WAS_CANCELLED` | Query cancelled or client disconnected |
| `UNKNOWN_FUNCTION` | Unrecognized function name |
| `UNKNOWN_IDENTIFIER` | Column/identifier not found |
| `UNKNOWN_TABLE` | Table not found |
| `TIMEOUT_EXCEEDED` | Query execution time exceeded limit |
| `TYPE_MISMATCH` | Data type incompatibility |

Templates use placeholder substitution (e.g. `{table_name}`, `{username}`) to produce context-specific guidance. They are defined in `templates/solution_templates.json`.

## Configuration

The default configuration can be customized during the wizard when you decline the default settings:

| Setting | Default | Description |
|---|---|---|
| Dashboard variable name | `hdx_query_errors` | Variable name used for error storage |
| Error TTL | `300` (5 min) | How long errors remain visible (seconds) |
| Max error count | `5` | Maximum number of errors displayed |

## Project Structure

```
error_panel/
  grafana_error_panel_cli.py   # Main CLI application
  setup.py                     # Package configuration
  requirements.txt             # Python dependencies
  templates/
    error_panel.html           # Handlebars HTML template for the panel
    before_render.js           # Pre-render hook (filters errors, registers helpers)
    after_render.js            # Post-render hook (show more/less interactivity)
    solution_templates.json    # Error-to-solution mapping definitions
    README.md                  # Template customization guide
```

## Dependencies

- [click](https://click.palletsprojects.com/) - CLI framework
- [requests](https://requests.readthedocs.io/) - HTTP client for the Grafana API
- [inquirer](https://github.com/magmax/python-inquirer) - Interactive prompts in the wizard