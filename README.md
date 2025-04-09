# Hydrolix data source for Grafana

The Hydrolix data source plugin integrates Hydrolix with Grafana, enabling seamless querying, analysis, and
visualization of Hydrolix data.

## Install the plugin

To install the Hydrolix data source plugin:

1. Open the [Grafana Plugin Catalog](https://grafana.com/grafana/plugins/).
2. Search for **Hydrolix Data Source**.
3. Select the plugin and click **Install Plugin**.

After installation:

1. In Grafana, go to **Connections > Data Sources > Add new data source**.
2. Select **Hydrolix** from the list.

> For more details about installation, see
> Grafana’s [Plugin management documentation](https://grafana.com/docs/grafana/latest/administration/plugin-management/).

## Configure the data source

You can configure the Hydrolix data source directly within Grafana or via configuration files.

Following is the list of Hydrolix configuration options:

| Name                                                   | Description                                                                                                                                                                   |
|--------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Name**                                               | The name used to reference this data source in panels and queries                                                                                                             |
| **Default**                                            | Toggle to set this Hydrolix data source as the default in panels and visualizations                                                                                           |
| **Server address**                                     | The IP address or hostname of your Hydrolix instance                                                                                                                          |
| **Server port**                                        | The port on which your Hydrolix instance is running                                                                                                                           |
| **Use default**                                        | Toggle to use the default port instead of specifying a custom one                                                                                                             |
| **Protocol**                                           | The communication protocol used: Native or HTTP                                                                                                                               |
| **Secure connection**                                  | Toggle to enable a secure connection                                                                                                                                          |
| **HTTP URL path** (optional)                           | Additional URL path for HTTP requests                                                                                                                                         |
| **Skip TLS verify**                                    | Toggle to bypass TLS certificate verification. Not recommended, unless absolutely necessary for testing                                                                       |
| **Username**, **Password**                             | Credentials for connecting to your Hydrolix instance                                                                                                                          |
| **Default database** (optional)                        | Used when no database is explicitly included in the query                                                                                                                     |
| **Default round** (optional)                           | Used when a query does not specify a round value. Aligns `$from` and `$to` to the nearest multiple of this value. For more details, see [Round timestamps](#round-timestamps) |
| **Ad-hoc filter table variable name** (optional)       | Variable indicating which table to use for ad hoc filter keys                                                                                                                 |
| **Ad-hoc filter time column variable name** (optional) | Variable indicating which column to use for time filtering in value queries                                                                                                   |
| **Ad-hoc filter keys query** (optional)                | Query template to retrieve possible keys for ad hoc filters                                                                                                                   |
| **Ad-hoc filter values query** (optional)              | Query template to retrieve possible values for ad hoc filter keys                                                                                                             |
| **Ad-hoc filter default time range** (optional)        | Default time range for filtering values for ad hoc filter keys when dashboard time range is not available                                                                     |
| **Dial timeout** (optional)                            | Connection timeout in seconds                                                                                                                                                 |
| **Query timeout** (optional)                           | Read timeout in seconds                                                                                                                                                       |

### Provision the data source

To provision the Hydrolix data source using Grafana’s provisioning system, define it in a YAML configuration file.

Below are some provisioning examples.

#### Using HTTPS protocol

```yaml
apiVersion: 1
datasources:
  - name: "Hydrolix"
    type: "hydrolix-hydrolix-datasource"
    jsonData:
      host: localhost
      port: 443
      protocol: http
      secure: true
      username: username
      path: /query
    secureJsonData:
      password: password
```

#### Using native protocol

```yaml
apiVersion: 1
datasources:
  - name: "Hydrolix"
    type: "hydrolix-hydrolix-datasource"
    jsonData:
      host: localhost
      port: 9440
      protocol: native
      secure: true
      username: username
    secureJsonData:
      password: password
```

#### Using HTTPS protocol with defaults and ad hoc filters

```yaml
apiVersion: 1
datasources:
  - name: "Hydrolix"
    type: "hydrolix-hydrolix-datasource"
    jsonData:
      host: localhost
      port: 443
      protocol: http
      secure: true
      username: username
      path: /query
      defaultDatabase: database
      defaultRound: 60s
      adHocTableVariable: table
      adHocTimeColumnVariable: timeColumn
      adHocKeysQuery: DESCRIBE $${table}
      adHocValuesQuery: >
        SELECT $${column}, COUNT(*) as count FROM $${table}
        WHERE $$__timeFilter($${timeColumn}) AND $$__adHocFilter()
        GROUP BY $${column}
        ORDER BY count DESC
        LIMIT 100
    secureJsonData:
      password: password
```

> For more details about provisioning, see
> Grafana’s [Provisioning documentation](https://grafana.com/docs/grafana/latest/administration/provisioning/#data-sources).

## Querying the data source

The query editor in Grafana enables powerful SQL querying with convenient syntax enhancements through macros and
templates.

### SQL query editor

The editor provides extensive SQL capabilities, featuring:

- Intelligent autocompletion for databases, tables, columns, and SQL syntax.
- Template variable and macro support.
- Code formatting.

#### Keyboard shortcuts

- `Cmd/Ctrl + Return` – Run the query.

### Macros

To simplify syntax and to allow for dynamic parts, like date range filters, the query can contain macros.

| Macro                                        | Description                                                                                                           | Output example                                                                                        |
|----------------------------------------------|-----------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------|
| `$__dateFilter(column)`                      | Generates a condition to filter data (using the provided column) based on the panel's date range                      | `date >= toDate('2022-10-21') AND date <= toDate('2022-10-23')`                                       |
| `$__timeFilter(column)`                      | Generates a condition to filter data (using the provided column) based on the panel's time range in seconds           | `time >= toDateTime(1415792726) AND time <= toDateTime(1447328726)`                                   |
| `$__timeFilter_ms(column)`                   | Generates a condition to filter data (using the provided column) based on the panel's time range in milliseconds      | `time >= fromUnixTimestamp64Milli(1415792726123) AND time <= fromUnixTimestamp64Milli(1447328726456)` |
| `$__dateTimeFilter(dateColumn, timeColumn)`  | Combines `$__dateFilter()` and `$__timeFilter()` for filtering with separate date and time columns                    | `$__dateFilter(dateColumn) AND $__timeFilter(timeColumn)`                                             |
| `$__adHocFilter`                             | Replaced with a condition to filter data based on the ad hoc query                                                    | `statusCode = '200'`                                                                                  |
| `$__fromTime`                                | Replaced with the panel's start time, cast as `DateTime`                                                              | `toDateTime(1415792726)`                                                                              |
| `$__toTime`                                  | Replaced with the panel's end time, cast as `DateTime`                                                                | `toDateTime(1447328726)`                                                                              |
| `$__fromTime_ms`                             | Replaced with the panel's start time, cast as `DateTime64(3)` (millisecond precision)                                 | `fromUnixTimestamp64Milli(1415792726123)`                                                             |
| `$__toTime_ms`                               | Replaced with the panel's end time, cast as `DateTime64(3)` (millisecond precision)                                   | `fromUnixTimestamp64Milli(1447328726456)`                                                             |
| `$__interval_s`                              | Replaced with the interval in seconds                                                                                 | `20`                                                                                                  |
| `$__timeInterval(column)`                    | Calculates intervals based on panel width, useful for grouping data in seconds                                        | `toStartOfInterval(toDateTime(column), INTERVAL 20 second)`                                           |
| `$__timeInterval_ms(column)`                 | Calculates intervals based on panel width, useful for grouping data in milliseconds                                   | `toStartOfInterval(toDateTime64(column, 3), INTERVAL 20 millisecond)`                                 |
| `$__conditionalAll(condition, $templateVar)` | Includes the provided condition only if the template variable does not select all values, defaults to `1=1` otherwise | `condition` or `1=1`                                                                                  |                                                                                                       |

Below is an example of a query with the `$__timeFilter` macro:

```sql
SELECT $__timeInterval(log_time) AS time, avg(cpu_usage) AS value
FROM logs
WHERE $__timeFilter(log_time)
GROUP BY time
ORDER BY time
```

### Ad hoc filters

Ad hoc filters allow flexible, key-value filtering dynamically applied across queries. These filters are injected into
queries via the `$__adHocFilter` macro, which must be explicitly included in the `WHERE` clause:

```sql
SELECT $__timeInterval(log_time) AS time, avg(cpu_usage) AS value
FROM logs
WHERE $__timeFilter(log_time) AND $__adHocFilter()
GROUP BY time
ORDER BY time
```

The plugin ensures filters are applied only when valid for the selected table.

#### Configure ad hoc filters

To enable ad hoc filters, both the data source and the dashboard must be configured properly:

1. In the data source settings (under *Advanced Settings*):
    - **Ad-hoc filter table variable name**: the name of a dashboard variable that defines the table used for retrieving
      filter keys and values.
    - **Ad-hoc filter time column variable name**: the name of a dashboard variable that defines the time column used
      for time filtering in value queries.
    - **Ad-hoc filter keys query**: a query template for listing possible keys.
    - **Ad-hoc filter values query**: a query template for listing possible values.
    - **Ad-hoc filter default time range**: a default time range to use when the dashboard’s time range is unavailable.

2. In the target dashboard, create two variables using the exact names defined in the data source settings:
    - A variable for the table name.
    - A variable for the time column.

> **Note:** Ad hoc filters will not work unless both the data source and the dashboard are configured correctly. Be sure
> to match variable names precisely.

#### Query templates

These templates are used to dynamically generate the list of available keys and values for filtering.

**Keys query:**

```sql
DESCRIBE ${table}
```

**Values query:**

```sql
SELECT ${column}, COUNT(${column}) AS count
FROM ${table}
WHERE $__timeFilter(${timeColumn}) AND $__adHocFilter()
GROUP BY ${column}
ORDER BY count DESC
LIMIT 100
```

In the query templates, certain placeholders are dynamically replaced with real values at runtime. Here is what each
variable represents:

| Name            | Description                                                                                                                        |
|-----------------|------------------------------------------------------------------------------------------------------------------------------------|
| `${table}`      | Table from which the ad hoc keys and values are pulled                                                                             |
| `${column}`     | Specific column selected by the user as a filter key. This value comes from the list of available fields returned by the key query |
| `${timeColumn}` | Column used for applying the time filter                                                                                           |

> For more details about ad hoc filters, see
> Grafana’s [Ad hoc filters documentation](https://grafana.com/docs/grafana/latest/dashboards/variables/add-template-variables/#add-ad-hoc-filters).

### Round timestamps

To control how time ranges are aligned, `$from` and `$to` timestamps can be rounded to the nearest multiple of the round
value, set in the query editor or in the data source settings.

When a round value is set in the query editor, it takes precedence and is always used. If no round is set in the query editor,
the data source falls back to the default round, if it is configured and non-zero. If neither is set, or if the round value
in the query editor is explicitly set to `0`, no rounding is applied and the original timestamps are used as-is.

The supported time units for rounding are: `ms` (milliseconds), `s` (seconds), `m` (minutes), and `h` (hours).

#### Examples

| Default round | Query round | Effective round | Input timestamp | Rounded timestamp |
|---------------|-------------|-----------------|-----------------|-------------------|
| `5m`          | _not set_   | `5m`            | `10:07:20`      | `10:05:00`        |
| `5m`          | `1m`        | `1m`            | `09:02:30`      | `09:03:00`        |
| _not set_     | _not set_   | _not applied_   | `08:01:23`      | `08:01:23`        |
| `5m`          | `0`         | _not applied_   | `07:45:50`      | `07:45:50`        |

### Template variables

Hydrolix queries fully support Grafana's template variables, allowing the creation of dynamic and reusable dashboards.

> For more details about template variables, see
> Grafana’s [Template variables documentation](https://grafana.com/docs/grafana/latest/dashboards/variables/add-template-variables/).
