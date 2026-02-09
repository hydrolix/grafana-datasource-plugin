# Error Panel Setup Guide

## Prerequisites

### Required Plugins

Ensure the following Grafana plugins are installed:
- **Infinity** datasource plugin
- **Business Text** panel plugin
- **Hydrolix** datasource plugin (v0.10.0)

## Setup Instructions

### 1. Create Static Datasource

Create a new datasource with the following configuration:
- **Type**: Infinity
- **Name**: static
- **Other options**: Leave as default

### 2. Create Dashboard Variables

Create two dashboard variables with the following configuration:

#### Variable 1: `hdx_query_errors`
- **Type**: Custom
- **Show on dashboard**: Nothing

#### Variable 2: `hdx_query_errors_selected`
- **Type**: Custom
- **Show on dashboard**: Nothing

### 3. Create Error Panel

Create a new panel with the following configuration:

#### Panel Configuration
- **Panel type**: Business Text

#### Repeat Options
- **Repeat by variable**: hdx_selected_error
- **Repeat direction**: Vertical

#### Business Text Settings

**Render template**: All data

**Editors**: Enable the following editors:
- JavaScript code before content rendering
- JavaScript code after content ready

#### Editor Settings
- **Primary Content Language**: HTML
- **Formatting**: Disabled

#### Content Settings
- **Wrap automatically in paragraphs**: Disabled

**HTML Content**: Paste the following HTML template into the content editor:

```html
{{#if (errors)}}
<div>
    <table style="width:100%; border-collapse:collapse;">
        <thead>
        <tr style="text-align:left; border-bottom:1px solid rgba(255,255,255,.15);">
            <th style="padding:6px 4px;">Time</th>
            <th style="padding:6px 4px;">Message</th>
        </tr>
        </thead>
        <tbody>
        {{#each (errors)}}
        <tr style="border-bottom:1px solid rgba(255,255,255,.08);">
            <td style="padding:6px 4px; white-space:nowrap; opacity:.75;">{{time}}</td>
            <td style="padding:6px 4px;">
                {{#if (isShort message solution)}}
                {{message}}
                {{else}}
                {{#if (isSelected time)}}
                {{message}}
                {{#if solution}}
                <br/><br/>
                <h5>How to fix</h5>
                {{#each solution}}
                {{{this}}}<br/>
                {{/each}}
                {{/if}}
                <br>
                <a id="hdx-unselect-error-{{id}}" href="#" style="color: #797676;">show less</a>
                {{else}}
                {{short message}}
                <a id="hdx-select-error-{{id}}" href="#" style="color: #797676;">show more</a>
                {{/if}}
                {{/if}}
            </td>
        </tr>
        {{/each}}
        </tbody>
    </table>
</div>
{{else}}
<div style="font-size: clamp(14px, 6vw, 72px); font-weight: 600; text-align: center; height: 150px;">
    <span style="color: green;">No errors</span>
</div>
{{/if}}

```

#### JavaScript 

**Before Content Rendering**: Paste the following javascript template into the Before Content Rendering editor:
```javascript
const shortMessageLength = 180
const errorMaxTTL = 300
const errorVariable = "hdx_query_errors"

const raw = context.grafana.replaceVariables(`\${${errorVariable}}`);
const selected = context.grafana.replaceVariables(`\${${errorVariable}_selected}`);
let errors = [];
let suggestions;
try {
    suggestions = context.data['data'][0]
} catch (e) {
    suggestions = []
}
try {

    errors = JSON.parse(raw);
    errors = errors.filter(
        (e) =>
            new Date().getTime() - new Date(e.time).getTime() < errorMaxTTL * 1000
    );
    if (!Array.isArray(errors)) errors = [];
    errors.forEach((error, index) => {
        error['id'] = index
        if (error.template) {
            const suggestion = suggestions.find(o => o.error_name == error.template)
            if (suggestion) {
                let solution = suggestion.solution;
                for (const group in error.groups) {
                    solution = solution.replaceAll(`{${group}}`, error.groups[group]);
                }
                error.solution = solution.split("\n")
                error.solution2 = solution.replaceAll("\n", "<br/>")
            }
        }
    })
} catch (e) {
    errors = [];
}
if (!window.hdxErrors) {
    window.hdxErrors = {};
}
window.hdxErrors[errorVariable] = errors;

context.handlebars.registerHelper("errors", () => errors);


context.handlebars.registerHelper("short", (s) =>
    (s && s.length > shortMessageLength ? s.slice(0, shortMessageLength) + "…" : (s || ""))
);

context.handlebars.registerHelper("isShort", (str, s) =>
    !s && str && str.length <= shortMessageLength
);

context.handlebars.registerHelper("isSelected", (time) =>
    selected == `${time}`
);

```

**After Content Ready**: Paste the following javascript template into the After Content Ready editor:
```javascript
const errorVariable = "hdx_query_errors"
const selectedQueryParam = `var-${errorVariable}_selected`

function updateSelected(time) {
    const queryParams = {}
    queryParams[selectedQueryParam] = time;
    context.grafana.locationService.partial(queryParams, true)
}

window.hdxErrors[errorVariable].forEach(error => {
    const showMoreEl = document.getElementById('hdx-select-error-' + error.id);
    if (showMoreEl) {
        if (showMoreEl._handler) {
            showMoreEl.removeEventListener("click", showMoreEl._handler)
        }
        const selectFn = (e) => {
            e.preventDefault()
            updateSelected(error.time)
        }
        showMoreEl._handler = selectFn;
        showMoreEl.addEventListener("click", selectFn);
    }

    const showLessEl = document.getElementById('hdx-unselect-error-' + error.id);

    if (showLessEl) {
        if (showLessEl._handler) {
            showLessEl.removeEventListener("click", showLessEl._handler)
        }
        const unselectFn = (e) => {
            e.preventDefault()
            updateSelected('')
        };
        showLessEl._handler = unselectFn;
        showLessEl.addEventListener("click", unselectFn);
    }
})
```
#### Data source 

Select datasource **static**
- Type **JSON**
- Parser **Frontend**
- Source **Inline**
- Import data - upload `erroryze.json`

### 4. Create Row 

Create a new row with the following configuration:
- **Repeat for**: hdx_query_errors

Insert error panel into the row

