#!/usr/bin/env python3
"""
Grafana Error Panel Configuration CLI Tool

This tool automates the configuration of error panels in Grafana dashboards
via the Grafana HTTP API.
"""

import json
import sys
import os
from pathlib import Path
from typing import Optional, Dict, Any
import requests
import click
import inquirer


class CredentialManager:
    """Manage Grafana credentials storage and retrieval"""

    def __init__(self, config_dir: Optional[str] = None):
        if config_dir:
            self.config_dir = Path(config_dir)
        else:
            self.config_dir = Path.home() / '.grafana_error_panel'
        self.config_file = self.config_dir / 'config.json'

    def _ensure_config_dir(self):
        """Ensure config directory exists"""
        self.config_dir.mkdir(parents=True, exist_ok=True)
        # Set restrictive permissions on config directory
        os.chmod(self.config_dir, 0o700)

    def save_credentials(self, url: str, username: str, password: str):
        """Save credentials to config file"""
        self._ensure_config_dir()

        config = {
            'url': url,
            'username': username,
            'password': password,
        }

        with open(self.config_file, 'w') as f:
            json.dump(config, f, indent=2)

        # Set restrictive permissions on config file
        os.chmod(self.config_file, 0o600)
        click.echo(f"✓ Credentials saved to {self.config_file}")

    def load_credentials(self) -> Optional[Dict[str, Any]]:
        """Load credentials from config file"""
        if not self.config_file.exists():
            return None

        try:
            with open(self.config_file, 'r') as f:
                return json.load(f)
        except (json.JSONDecodeError, IOError) as e:
            click.echo(f"⚠️  Error reading credentials: {e}", err=True)
            return None

    def clear_credentials(self):
        """Clear stored credentials"""
        if self.config_file.exists():
            self.config_file.unlink()
            click.echo("✓ Credentials cleared")
        else:
            click.echo("ℹ No credentials found")

    def has_credentials(self) -> bool:
        """Check if credentials are stored"""
        return self.config_file.exists()


class GrafanaClient:
    """Client for interacting with Grafana API"""

    def __init__(self, url: str, username: str, password: str):
        self.url = url.rstrip('/')
        self.username = username
        self.password = password
        self.session = requests.Session()
        self.session.auth = (username, password)
        self.session.headers.update({
            'Content-Type': 'application/json'
        })

    def _request(self, method: str, endpoint: str, **kwargs) -> requests.Response:
        """Make HTTP request to Grafana API"""
        url = f"{self.url}/api/{endpoint}"
        response = self.session.request(method, url, **kwargs)
        response.raise_for_status()
        return response

    def test_connection(self) -> bool:
        """Test connection to Grafana API"""
        try:
            response = self._request('GET', 'org')
            response.raise_for_status()
            return True
        except Exception:
            return False

    def list_orgs(self) -> list:
        """List all organizations the user has access to"""
        response = self._request('GET', 'orgs')
        return response.json()

    def get_current_org(self) -> Dict[str, Any]:
        """Get current organization"""
        response = self._request('GET', 'org')
        return response.json()

    def get_current_user(self) -> Dict[str, Any]:
        """Get current organization"""
        response = self._request('GET', 'user')
        return response.json()

    def switch_org(self, org_id: int) -> bool:
        """Switch to a different organization"""
        response = self._request('POST', f'user/using/{org_id}')
        return response.status_code == 200

    def list_dashboards(self, org_id) -> list:
        """List all dashboards"""
        response = self._request('GET', f'search?type=dash-db&orgId={org_id}')
        return response.json()

    def list_datasources(self) -> list:
        """List all datasources"""
        response = self._request('GET', 'datasources')
        return response.json()

    def create_infinity_datasource(self, name: str = "static") -> Dict[str, Any]:
        """Create an Infinity datasource"""
        payload = {
            "name": name,
            "type": "yesoreyeram-infinity-datasource",
            "access": "proxy",
            "isDefault": False,
            "jsonData": {},
            "secureJsonData": {}
        }

        try:
            response = self._request('POST', 'datasources', json=payload)
            click.echo(f"✓ Created Infinity datasource '{name}'")
            return response.json()
        except requests.exceptions.HTTPError as e:
            if e.response.status_code == 409:
                click.echo(f"ℹ Datasource '{name}' already exists")
                # Get existing datasource
                response = self._request('GET', f'datasources/name/{name}')
                return response.json()
            raise

    def get_dashboard(self, uid: str) -> Dict[str, Any]:
        """Get dashboard by UID"""
        response = self._request('GET', f'dashboards/uid/{uid}')
        return response.json()

    def create_or_update_dashboard(self, dashboard_config: Dict[str, Any],
                                   message: str = "Updated by CLI") -> Dict[str, Any]:
        """Create or update a dashboard"""
        payload = {
            "dashboard": dashboard_config,
            "message": message,
            "overwrite": True
        }

        response = self._request('POST', 'dashboards/db', json=payload)
        result = response.json()
        click.echo(f"✓ Dashboard saved: {result.get('url', '')}")
        return result

    def get_datasource_by_name(self, name: str) -> Optional[Dict[str, Any]]:
        """Get datasource by name"""
        try:
            response = self._request('GET', f'datasources/name/{name}')
            return response.json()
        except requests.exceptions.HTTPError as e:
            if e.response.status_code == 404:
                return None
            raise


def create_error_panel_template(dashboard_var_name, datasource_uid) -> Dict[str, Any]:
    """Create the Business Text panel configuration for error display"""

    # Get the directory where this script is located
    script_dir = Path(__file__).parent
    templates_dir = script_dir / 'templates'

    # Load templates from files
    try:
        with open(templates_dir / 'error_panel.html', 'r') as f:
            html_content = f.read()

        with open(templates_dir / 'before_render.js', 'r') as f:
            before_render_js = f.read()

        with open(templates_dir / 'after_render.js', 'r') as f:
            after_render_js = f.read()
        with open(templates_dir / 'solution_templates.json', 'r') as f:
            solution_templates = f.read()
    except FileNotFoundError as e:
        click.echo(f"Error: Template file not found: {e}", err=True)
        sys.exit(1)

    panel = {
        "type": "marcusolsson-dynamictext-panel",
        "title": "Query Errors",
        "gridPos": {"h": 7, "w": 24, "x": 0, "y": 1},
        "options": {
            "afterRender": after_render_js,
            "content": html_content,
            "contentPartials": [],
            "defaultContent": "The query didn't return any results.",
            "editor": {
                "format": "none",
                "language": "html"
            },
            "editors": [
                "helpers",
                "afterRender"
            ],
            "externalStyles": [],
            "helpers": before_render_js,
            "renderMode": "data",
            "styles": "",
            "wrap": False
        },
        "targets": [{
            "datasource": {"type": "yesoreyeram-infinity-datasource", "uid": datasource_uid},
            "columns": [],
            "data": solution_templates,
            "refId": "A",
            "filters": [],
            "format": "table",
            "global_query_id": "",
            "parser": "simple",
            "root_selector": "",
            "source": "inline",
            "type": "json",
            "url": "",
            "url_options": {
                "data": "",
                "method": "GET"
            }
        }],
        "repeat": dashboard_var_name + "_selected",
        "repeatDirection": "v"
    }

    return panel


def create_dashboard_variables(dashboard_var_name) -> list:
    """Create the required dashboard variables"""
    return [
        {
            "name": dashboard_var_name,
            "type": "custom",
            "hide": 2,  # Variable hidden from dashboard
            "query": "",
            "current": {"value": "", "text": ""},
            "options": []
        },
        {
            "name": dashboard_var_name + "_selected",
            "type": "custom",
            "hide": 2,  # Variable hidden from dashboard
            "query": "",
            "current": {"value": "", "text": ""},
            "options": []
        },
    ]


def create_error_panel_row(dashboard_var_name, datasource_uid) -> Dict[str, Any]:
    """Create a row that repeats for hdx_query_errors"""
    return {
        "type": "row",
        "title": "Error Panel Row",
        "gridPos": {"h": 1, "w": 24, "x": 0, "y": 0},
        "repeat": dashboard_var_name,
        "collapsed": True,
        "panels": [create_error_panel_template(dashboard_var_name, datasource_uid)]
    }


def get_credentials(url: Optional[str], username: Optional[str], password: Optional[str]) -> tuple[str, str, str]:
    """Get credentials from CLI args or stored config, with interactive prompts"""
    cred_manager = CredentialManager()

    # If credentials provided via CLI, use them
    if url and username and password:
        return url, username, password

    # Try to load stored credentials
    stored_creds = cred_manager.load_credentials()

    # If no URL provided, try stored or prompt
    if not url:
        if stored_creds and stored_creds.get('url'):
            url = stored_creds['url']
            click.echo(f"Using stored Grafana URL: {url}")
        else:
            url = click.prompt('Grafana URL (e.g., http://localhost:3000)', type=str)

    # If no username provided, try stored or prompt
    if not username:
        if stored_creds and stored_creds.get('username'):
            username = stored_creds['username']
            click.echo(f"Using stored username: {username}")
        else:
            username = click.prompt('Grafana Username', type=str)

    # If no password provided, try stored or prompt
    if not password:
        if stored_creds and stored_creds.get('password'):
            password = stored_creds['password']
            click.echo("Using stored password")
        else:
            password = click.prompt('Grafana Password', type=str, hide_input=True)


    # Ask if user wants to save credentials
    if not stored_creds or (url != stored_creds.get('url') or username != stored_creds.get('username') or password != stored_creds.get('password')):
        if click.confirm('Save these credentials for future use?', default=True):
            cred_manager.save_credentials(url, username, password)

    return url, username, password


@click.group(invoke_without_command=True)
@click.pass_context
def cli(ctx):
    """Grafana Error Panel Configuration Tool

    Run without arguments to start the interactive wizard.
    """
    if ctx.invoked_subcommand is None:
        # No command specified, run wizard by default
        ctx.invoke(wizard)


@cli.command()
def login():
    """Set Grafana credentials (username and password)"""
    click.echo("=== Grafana Credentials Setup ===\n")

    url = click.prompt('Grafana URL (e.g., http://localhost:3000)', type=str)
    username = click.prompt('Grafana Username', type=str)
    password = click.prompt('Grafana Password', type=str, hide_input=True)

    # Test credentials
    click.echo("\nTesting connection...")
    try:
        client = GrafanaClient(url, username, password)
        # Try a simple API call to verify credentials
        client._request('GET', 'org')
        click.echo("✓ Connection successful!")

        # Save credentials
        cred_manager = CredentialManager()
        cred_manager.save_credentials(url, username, password)

    except requests.exceptions.HTTPError as e:
        if e.response.status_code == 401:
            click.echo("✗ Authentication failed. Please check your username and password.", err=True)
        else:
            click.echo(f"✗ Error: {e}", err=True)
        sys.exit(1)
    except Exception as e:
        click.echo(f"✗ Connection failed: {e}", err=True)
        sys.exit(1)


@cli.command()
def logout():
    """Clear stored Grafana credentials"""
    cred_manager = CredentialManager()
    cred_manager.clear_credentials()


@cli.command()
def status():
    """Show current credential status"""
    cred_manager = CredentialManager()

    if cred_manager.has_credentials():
        creds = cred_manager.load_credentials()
        if creds:
            click.echo("✓ Credentials are configured")
            click.echo(f"  URL: {creds.get('url')}")
            click.echo(f"  Config file: {cred_manager.config_file}")
        else:
            click.echo("⚠️  Credentials file exists but could not be read")
    else:
        click.echo("ℹ No credentials configured")
        click.echo("Run 'login' command to set up credentials")


@cli.command()
@click.option('--url', help='Grafana URL (e.g., http://localhost:3000)')
@click.option('--username', help='Grafana username')
@click.option('--password', help='Grafana password')
@click.option('--datasource-name', default='static', help='Name for the Infinity datasource')
def setup_datasource(url: Optional[str], username: Optional[str], password: Optional[str],
                     datasource_name: str):
    """Create the Infinity datasource required for error panels"""
    try:
        url, username, password = get_credentials(url, username, password)
        client = GrafanaClient(url, username, password)
        datasource = client.create_infinity_datasource(datasource_name)
        click.echo(f"\nDatasource UID: {datasource.get('uid')}")
        click.echo("Use this UID when configuring dashboards")
    except Exception as e:
        click.echo(f"✗ Error: {e}", err=True)
        sys.exit(1)


@cli.command()
@click.option('--url', help='Grafana URL')
@click.option('--username', help='Grafana username')
@click.option('--password', help='Grafana password')
@click.option('--dashboard-uid', help='UID of existing dashboard to update')
@click.option('--dashboard-title', default='Error Panel Dashboard', help='Title for new dashboard')
@click.option('--datasource-name', default='static', help='Name of the Infinity datasource')
def configure_dashboard(url: Optional[str], username: Optional[str], password: Optional[str],
                       dashboard_uid: Optional[str], dashboard_title: str, datasource_name: str):
    """Configure a dashboard with error panel setup"""
    try:
        url, username, password = get_credentials(url, username, password)
        client = GrafanaClient(url, username, password)

        # Get datasource UID
        datasource = client.get_datasource_by_name(datasource_name)
        if not datasource:
            click.echo(f"✗ Datasource '{datasource_name}' not found. Run 'setup-datasource' first.", err=True)
            sys.exit(1)

        ds_uid = datasource['uid']
        click.echo(f"✓ Found datasource '{datasource_name}' (UID: {ds_uid})")

        # Get existing dashboard or create new one
        if dashboard_uid:
            dashboard_data = client.get_dashboard(dashboard_uid)
            dashboard = dashboard_data['dashboard']
            click.echo(f"✓ Loaded existing dashboard '{dashboard['title']}'")
        else:
            dashboard = {
                "title": dashboard_title,
                "tags": ["error-monitoring"],
                "timezone": "browser",
                "schemaVersion": 38,
                "version": 0,
                "refresh": "30s"
            }
            click.echo(f"✓ Creating new dashboard '{dashboard_title}'")

        # Add/update variables
        dashboard['templating'] = {'list': create_dashboard_variables()}
        click.echo("✓ Added dashboard variables")

        # Create error panel
        error_panel = create_error_panel_template()

        # Update datasource UID in panel
        if error_panel['targets']:
            error_panel['targets'][0]['datasource']['uid'] = ds_uid

        # Create row and add panel
        row = create_error_panel_row()

        # Initialize panels list if not exists
        if 'panels' not in dashboard:
            dashboard['panels'] = []

        # Add row and panel
        dashboard['panels'].extend([row, error_panel])
        click.echo("✓ Added error panel and row configuration")

        # Save dashboard
        result = client.create_or_update_dashboard(dashboard, "Configured error panel via CLI")
        click.echo(f"\n✓ Dashboard configured successfully!")
        click.echo(f"  UID: {result.get('uid')}")
        click.echo(f"  URL: {url}{result.get('url')}")

    except Exception as e:
        click.echo(f"✗ Error: {e}", err=True)
        import traceback
        traceback.print_exc()
        sys.exit(1)


@cli.command()
@click.option('--url', help='Grafana URL')
@click.option('--username', help='Grafana username')
@click.option('--password', help='Grafana password')
@click.option('--dashboard-uid', required=True, help='Dashboard UID')
def show_dashboard(url: Optional[str], username: Optional[str], password: Optional[str],
                   dashboard_uid: str):
    """Show dashboard configuration as JSON"""
    try:
        url, username, password = get_credentials(url, username, password)
        client = GrafanaClient(url, username, password)
        dashboard_data = client.get_dashboard(dashboard_uid)
        click.echo(json.dumps(dashboard_data, indent=2))
    except Exception as e:
        click.echo(f"✗ Error: {e}", err=True)
        sys.exit(1)


@cli.command()
def example_config():
    """Show example configuration for error panel"""
    config = {
        "panel": create_error_panel_template(),
        "variables": create_dashboard_variables(),
        "row": create_error_panel_row()
    }
    click.echo(json.dumps(config, indent=2))


@cli.command()
def wizard():
    """Interactive wizard to configure error panel"""
    click.clear()
    click.echo("=" * 70)
    click.echo("  Grafana Error Panel Configuration Wizard")
    click.echo("=" * 70)
    click.echo()

    # Credentials
    click.echo("Grafana Credentials")
    click.echo("-" * 70)

    cred_manager = CredentialManager()
    stored_creds = cred_manager.load_credentials()

    if stored_creds:
        click.echo(f"✓ Found stored credentials for: {stored_creds.get('username')}@{stored_creds.get('url')}")

        questions = [
            inquirer.Confirm('use_stored',
                           message="Use stored credentials?",
                           default=True)
        ]
        answers = inquirer.prompt(questions)

        if answers and answers['use_stored']:
            url = stored_creds['url']
            username = stored_creds['username']
            password = stored_creds['password']
        else:
            url = click.prompt('\nGrafana URL', type=str, default=stored_creds.get('url', 'http://localhost:3000'))
            username = click.prompt('Grafana Username', type=str, default=stored_creds.get('username', 'admin'))
            password = click.prompt('Grafana Password', type=str, hide_input=True)
    else:
        click.echo("No stored credentials found.")
        url = click.prompt('\nGrafana URL', type=str, default='http://localhost:3000')
        username = click.prompt('Grafana Username', type=str, default='admin')
        password = click.prompt('Grafana Password', type=str, hide_input=True)

    # Test connection
    click.echo("\nTesting connection...")
    try:
        client = GrafanaClient(url, username, password)
        client.test_connection()
        click.echo("✓ Connection successful!\n")
    except Exception as e:
        click.echo(f"✗ Connection failed: {e}", err=True)
        sys.exit(1)

    # Ask to save credentials
    if not stored_creds or (url != stored_creds.get('url') or username != stored_creds.get('username') or password != stored_creds.get('password')):
        questions = [
            inquirer.Confirm('save_creds',
                           message="Save credentials for future use?",
                           default=True)
        ]
        answers = inquirer.prompt(questions)

        if answers and answers['save_creds']:
            cred_manager.save_credentials(url, username, password)

    click.echo()

    # Organization
    click.echo("Organization Selection")
    click.echo("-" * 70)


    current_org = client.get_current_org()
    current_org_id = current_org.get('id')
    click.echo(f"Current organization: {current_org.get('name')} (ID: {current_org_id})")
    questions = [
        inquirer.Confirm('org_change',
                         message="Would you like to change organization?",
                         default=False)
    ]
    answers = inquirer.prompt(questions)

    if answers and answers['org_change']:
        try:
            # Try to list organizations
            try:
                orgs = client.list_orgs()
            except requests.exceptions.HTTPError as e:
                if e.response.status_code == 401:
                    # User doesn't have permission to list orgs, use current org
                    click.echo("⚠ Unable to list organizations (insufficient permissions)")
                    click.echo("Using current organization...")
                    orgs = None
                else:
                    raise


            if orgs is None:
                # Could not list orgs, just use current
                click.echo(f"Using organization: {current_org.get('name')} ({current_org_id})")
            elif len(orgs) > 1:
                click.echo(f"Found {len(orgs)} organizations\n")
                click.echo()

                # Prepare choices for inquirer
                org_choices = []
                for org in orgs:
                    is_current = " (current)" if org['id'] == current_org_id else ""
                    label = f"{org.get('name')}{is_current} (ID: {org.get('id')})"
                    org_choices.append((label, org))

                questions = [
                    inquirer.List('organization',
                                 message="Select an organization",
                                 choices=org_choices,
                                 )
                ]
                answers = inquirer.prompt(questions)

                if not answers:  # User pressed Ctrl+C
                    click.echo("\nCancelled by user")
                    sys.exit(0)

                selected_org = answers['organization']
                selected_org_id = selected_org['id']

                # Switch org if different from current
                if selected_org_id != current_org_id:
                    click.echo(f"\nSwitching to organization: {selected_org.get('name')}")
                    client.switch_org(selected_org_id)
                    click.echo("✓ Organization switched")
                    current_org_id = selected_org_id
                else:
                    click.echo(f"✓ Using current organization: {selected_org.get('name')}")
            else:
                # Only one org, use it
                org = orgs[0]
                click.echo(f"Using organization: {org.get('name')} (ID: {org.get('id')})")
                current_org_id = org.get('orgId')

        except Exception as e:

            click.echo(f"✗ Error with organization: {e}", err=True)
            click.echo("Continuing with current organization...")
        # Don't exit, try to continue

    click.echo()

    # Dashboard
    click.echo("Dashboard Selection")
    click.echo("-" * 70)

    try:
        dashboards = client.list_dashboards(org_id=current_org_id)

        if not dashboards:
            click.echo("✗ No dashboards found. Please create a dashboard in Grafana first.", err=True)
            sys.exit(1)

        click.echo(f"Found {len(dashboards)} existing dashboards\n")

        # Prepare dashboard choices for inquirer
        dash_choices = []
        for dash in dashboards:
            label = f"{dash.get('title')} (UID: {dash.get('uid')})"
            dash_choices.append((label, dash))

        questions = [
            inquirer.List('dashboard',
                         message="Select a dashboard",
                         choices=dash_choices,
                         )
        ]
        answers = inquirer.prompt(questions)

        if not answers:  # User pressed Ctrl+C
            click.echo("\nCancelled by user")
            sys.exit(0)

        selected_dash = answers['dashboard']
        dashboard_uid = selected_dash['uid']
        dashboard_title = selected_dash['title']

        click.echo(f"✓ Selected dashboard: '{dashboard_title}' (UID: {dashboard_uid})")

    except Exception as e:
        click.echo(f"✗ Error loading dashboards: {e}", err=True)
        sys.exit(1)

    click.echo()

    # Dashboard Variable names
    questions = [
        inquirer.Confirm('default_conf',
                         message="Would you like to use default configuration?",
                         default=True)
    ]
    answers = inquirer.prompt(questions)
    dashboard_var_name = 'hdx_query_errors'
    if not answers or not answers['default_conf']:
        dashboard_var_name = click.prompt(
            'Dashboard variable name',
            type=str,
            default='hdx_query_errors'
        )

        error_ttl = click.prompt(
            'Error TTL',
            type=int,
            default=300
        )
        max_error_count = click.prompt(
            'Max error count',
            type=int,
            default=5
        )

    # Datasource
    click.echo("Datasource Configuration")
    click.echo("-" * 70)

    try:
        datasources = client.list_datasources()
        infinity_datasources = [
            ds for ds in datasources
            if ds.get('type') == 'yesoreyeram-infinity-datasource'
        ]

        click.echo(f"Found {len(infinity_datasources)} Infinity datasources\n")

        datasource_name = None
        datasource_uid = None

        if infinity_datasources:
            # Prepare choices for inquirer
            ds_choices = []
            for ds in infinity_datasources:
                label = f"{ds.get('name')} (UID: {ds.get('uid')})"
                ds_choices.append((label, ds))

            # Add create new option
            ds_choices.append(('Create new datasource', None))

            questions = [
                inquirer.List('datasource',
                             message="Select a datasource",
                             choices=ds_choices,
                             )
            ]
            answers = inquirer.prompt(questions)

            if not answers:  # User pressed Ctrl+C
                click.echo("\nCancelled by user")
                sys.exit(0)

            selected_ds = answers['datasource']

            if selected_ds is None:
                # Create new
                datasource_name = click.prompt(
                    '\nDatasource name',
                    type=str,
                    default='static'
                )
                click.echo(f"Creating datasource '{datasource_name}'...")
                result = client.create_infinity_datasource(datasource_name)
                datasource_uid = result['uid']
            else:
                # Use existing
                datasource_name = selected_ds['name']
                datasource_uid = selected_ds['uid']
                click.echo(f"✓ Using existing datasource: '{datasource_name}'")
        else:
            click.echo("No Infinity datasources found. Creating new one...")
            datasource_name = click.prompt(
                'Datasource name',
                type=str,
                default='static'
            )
            click.echo(f"\nCreating datasource '{datasource_name}'...")
            result = client.create_infinity_datasource(datasource_name)
            datasource_uid = result['datasource']['uid']

        click.echo(f"✓ Datasource ready (UID: {datasource_uid})")

    except Exception as e:
        click.echo(f"✗ Error with datasource: {e}", err=True)
        sys.exit(1)

    click.echo()

    # Configuration
    click.echo("Final Configuration")
    click.echo("-" * 70)

    click.echo("\nConfiguration Summary:")
    click.echo(f"  Grafana URL: {url}")
    click.echo(f"  Dashboard: {dashboard_title}")
    click.echo(f"  Dashboard UID: {dashboard_uid}")
    click.echo(f"  Dashboard variable name: {dashboard_var_name}")
    click.echo(f"  Datasource: {datasource_name} (UID: {datasource_uid})")
    click.echo()

    questions = [
        inquirer.Confirm('proceed',
                        message="Proceed with configuration?",
                        default=True)
    ]
    answers = inquirer.prompt(questions)

    if not answers or not answers['proceed']:
        click.echo("Configuration cancelled.")
        sys.exit(0)

    try:
        click.echo("\nConfiguring error panel...")

        # Load the selected dashboard
        dashboard_data = client.get_dashboard(dashboard_uid)
        dashboard = dashboard_data['dashboard']
        click.echo(f"✓ Loaded dashboard: {dashboard_title}")

        # Add variables
        variables = dashboard.get("templating", {}).get("list")
        if not variables:
            dashboard['templating'] = {'list': create_dashboard_variables(dashboard_var_name)}
        else:
            variables = list(filter(lambda v: v["name"] !=  dashboard_var_name and v["name"] !=  dashboard_var_name + "_selected", variables))
            variables.extend(create_dashboard_variables(dashboard_var_name))
            dashboard['templating'] = {'list': variables}


        click.echo("✓ Added dashboard variables")

        # Create row
        row = create_error_panel_row(dashboard_var_name, datasource_uid)

        # Add panels
        if 'panels' not in dashboard:
            dashboard['panels'] = []
        dashboard['panels'].insert(0, row)
        click.echo("✓ Added error panel and row")

        # Save dashboard
        result = client.create_or_update_dashboard(
            dashboard,
            "Configured error panel via wizard"
        )

        click.echo("\n" + "=" * 70)
        click.echo("✅ SUCCESS! Error panel configured successfully!")
        click.echo("=" * 70)
        click.echo(f"\nDashboard UID: {result.get('uid')}")
        click.echo()

    except Exception as e:
        click.echo(f"\n✗ Error during configuration: {e}", err=True)
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == '__main__':
    cli()
