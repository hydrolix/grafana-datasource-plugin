# Error Panel Templates

This directory contains the templates used to generate the Grafana error panel configuration.

## Files

### `error_panel.html`

**Type:** Handlebars HTML Template
**Purpose:** Defines the visual structure of the error panel

**What it does:**
- Creates an error table with columns: Timestamp, Error Message, Details
- Uses Handlebars syntax for dynamic content rendering
- Implements expandable/collapsible error details
- Shows solution suggestions when available
- Displays "No errors" message when data is empty

**Available Variables:**
- `{{data}}` - Array of error objects
- `{{timestamp}}` - Error timestamp
- `{{message}}` - Error message text
- `{{details}}` - Detailed error information
- `{{solution}}` - Suggested solution (optional)
- `{{@index}}` - Current error index in loop

**Example Customization:**
```html
<!-- Add a new severity column -->
<th>Severity</th>
...
<td class="severity-{{severity}}">{{severity}}</td>
```

---

### `before_render.js`

**Type:** JavaScript (Pre-rendering Hook)
**Purpose:** Processes data before rendering in Grafana

**What it does:**
- Filters errors by TTL (Time To Live) - only shows errors from last 5 minutes
- Registers Handlebars helper functions
- Modifies the context object before template rendering

**Key Settings:**
- `TTL = 300` - Time window in seconds (5 minutes)

**Example Customization:**
```javascript
// Change to 10 minutes
const TTL = 600;

// Add custom Handlebars helper
Handlebars.registerHelper('formatDate', function(timestamp) {
  return new Date(timestamp).toLocaleDateString();
});
```

---

### `after_render.js`

**Type:** JavaScript (Post-rendering Hook)
**Purpose:** Adds interactivity after the panel is rendered

**What it does:**
- Attaches click handlers to "Show more" links
- Manages expansion/collapse of error details
- Handles DOM interactions for error display

**Example Customization:**
```javascript
// Add auto-expand for critical errors
document.querySelectorAll('.error-critical').forEach(error => {
  error.querySelector('.error-details').style.display = 'block';
});

// Add keyboard shortcuts
document.addEventListener('keydown', (e) => {
  if (e.key === 'e') {
    // Expand all errors
  }
});
```

---

## How Templates Are Used

1. **Loading:** The `create_error_panel_template()` function reads these files at runtime
2. **Integration:** Content is inserted into the Grafana panel configuration
3. **Rendering:** Grafana's Business Text panel renders the HTML with data
4. **Execution:** JavaScript hooks execute at appropriate times

## Workflow

```
Python CLI
    ↓
Read template files
    ↓
Generate panel config
    ↓
Send to Grafana API
    ↓
Grafana renders panel
    ↓ (before_render.js)
Filter & prepare data
    ↓
Render HTML template
    ↓ (after_render.js)
Add interactivity
```

## Testing Your Changes

After modifying templates, test the configuration:

```bash
# View generated config
python grafana_error_panel_cli.py example-config

# Apply to dashboard
python grafana_error_panel_cli.py wizard
```

## Best Practices

### HTML Template
- ✅ Keep semantic HTML structure
- ✅ Use CSS classes for styling (don't inline styles)
- ✅ Maintain accessibility attributes
- ✅ Test with empty data (`{{else}}` block)

### JavaScript
- ✅ Check for undefined/null before accessing properties
- ✅ Use `const` and `let` instead of `var`
- ✅ Add comments for complex logic
- ✅ Handle edge cases (empty arrays, missing elements)

### General
- ✅ Test changes before deploying
- ✅ Keep backups of working templates
- ✅ Document custom modifications
- ✅ Use version control for templates

## Common Customizations

### Change Time Window
```javascript
// before_render.js
const TTL = 1800; // 30 minutes
```

### Add Custom CSS Classes
```html
<!-- error_panel.html -->
<tr class="error-row severity-{{severity}}">
```

### Add Search/Filter
```javascript
// after_render.js
const searchBox = document.createElement('input');
searchBox.placeholder = 'Search errors...';
searchBox.addEventListener('input', (e) => {
  const query = e.target.value.toLowerCase();
  document.querySelectorAll('.error-row').forEach(row => {
    const text = row.textContent.toLowerCase();
    row.style.display = text.includes(query) ? '' : 'none';
  });
});
```

### Format Timestamps
```javascript
// before_render.js
Handlebars.registerHelper('timeAgo', function(timestamp) {
  const seconds = Math.floor((Date.now() - new Date(timestamp)) / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  return `${Math.floor(seconds / 3600)}h ago`;
});
```

## Troubleshooting

### Template not loading
- Check file exists in `templates/` directory
- Verify file permissions (readable)
- Check for syntax errors in templates

### JavaScript not executing
- Check browser console for errors
- Verify Handlebars syntax is correct
- Ensure element IDs match between HTML and JavaScript

### Styles not applying
- Check CSS class names match
- Verify Grafana theme compatibility
- Use browser DevTools to inspect rendered HTML

## References

- [Handlebars Documentation](https://handlebarsjs.com/)
- [Grafana Business Text Panel](https://grafana.com/grafana/plugins/marcusolsson-dynamictext-panel/)
- [JavaScript Event Listeners](https://developer.mozilla.org/en-US/docs/Web/API/EventTarget/addEventListener)
