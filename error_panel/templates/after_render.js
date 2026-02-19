const errorVariable = "{{dashboard_var_name}}";
const selectedQueryParam = `var-${errorVariable}_selected`;

function updateSelected(time) {
  const queryParams = {};
  queryParams[selectedQueryParam] = time;
  context.grafana.locationService.partial(queryParams, true);
}

window.hdxErrors[errorVariable].forEach((error) => {
  const showMoreEl = document.getElementById("hdx-select-error-" + error.id);
  if (showMoreEl) {
    if (showMoreEl._handler) {
      showMoreEl.removeEventListener("click", showMoreEl._handler);
    }
    const selectFn = (e) => {
      e.preventDefault();
      updateSelected(error.time);
    };
    showMoreEl._handler = selectFn;
    showMoreEl.addEventListener("click", selectFn);
  }

  const showLessEl = document.getElementById("hdx-unselect-error-" + error.id);

  if (showLessEl) {
    if (showLessEl._handler) {
      showLessEl.removeEventListener("click", showLessEl._handler);
    }
    const unselectFn = (e) => {
      e.preventDefault();
      updateSelected("");
    };
    showLessEl._handler = unselectFn;
    showLessEl.addEventListener("click", unselectFn);
  }
});
