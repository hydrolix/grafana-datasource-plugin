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
      const suggestion = suggestions.find(o => o.name == error.template)
      if (suggestion) {
        let solution = suggestion.template;
        for (const group in error.groups) {
          solution = solution.replaceAll(`{${group}}`, error.groups[group]);
        }
        error.solution = solution.split("\n")
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