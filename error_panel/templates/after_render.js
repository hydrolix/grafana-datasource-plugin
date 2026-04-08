const shortMessageLength = {{message_length}};
const errorMaxTTL = {{error_ttl}};
const errorVariable = "{{dashboard_var_name}}";
const updateInterval = 1000;

const isShort = (str, s) => !s && str && str.length <= shortMessageLength
const short = (s) =>
    s && s.length > shortMessageLength
        ? s.slice(0, shortMessageLength) + "…"
        : s || ""

const getErrors = () => {
  const raw = context.grafana.replaceVariables(`\${${errorVariable}}`);
  let errors = [];
  let suggestions;
  try {
    suggestions = context.data[0];
  } catch (e) {
    suggestions = [];
  }
  try {
    errors = JSON.parse(raw);
    errors = errors.filter(
        (e) =>
            new Date().getTime() - new Date(e.time).getTime() < errorMaxTTL * 1000
    );
    if (!Array.isArray(errors)) {
      errors = [];
    }
    errors.forEach((error) => {
      if (error.template) {
        const suggestion = suggestions.find((o) => o.name == error.template);
        if (suggestion) {
          let solution = suggestion.template;
          for (const group in error.groups) {
            solution = solution.replaceAll(`{${group}}`, error.groups[group]);
          }
          error.solution = solution
        }
      }
    });
  } catch (e) {
    errors = [];
  }
  return errors;
}

const renderTableRows = (errors) => {

  const rows = errors.map((error) => {
    let row = document.createElement("tr")
    row.className = 'hdx-error-table-tr'
    let timeCell = document.createElement("td");
    timeCell.className = 'hdx-error-table-time hdx-error-table-td'
    timeCell.textContent = error.time;
    row.appendChild(timeCell);
    let errorCell = document.createElement("td");
    errorCell.className = 'hdx-error-table-td'
    row.appendChild(errorCell);
    if (isShort(error.message, error.solution)) {
      errorCell.innerHTML = error.message;
    } else {
      let shortMessage = document.createElement("div")
      let showMoreButton = document.createElement("a")
      shortMessage.innerHTML = short(error.message) + "\n "
      showMoreButton.href = '#';
      showMoreButton.innerText = 'show more'
      showMoreButton.className = 'hdx-error-table-button'
      shortMessage.appendChild(showMoreButton);
      errorCell.appendChild(shortMessage)

      let longMessage = document.createElement("div")
      longMessage.style.display = 'none'
      let showLessButton = document.createElement("a")
      showLessButton.href = '#';
      showLessButton.innerText = 'show less'
      showLessButton.className = 'hdx-error-table-button'

      let solutionHtml = error.solution ? `<br><h5>How to fix</h5>${window.markdownIt.render(error.solution)}<br>`: ''
      longMessage.innerHTML = `<div>${error.message}<br>
          ${solutionHtml}
        </div>`;
      longMessage.appendChild(showLessButton)
      showLessButton.onclick = () => {
        shortMessage.style.display = 'block'
        longMessage.style.display = 'none'
        window.hdxErrorSelected = false
      }
      showMoreButton.onclick = () => {
        shortMessage.style.display = 'none'
        longMessage.style.display = 'block'
        window.hdxErrorSelected = error.time
      }
      if (window.hdxErrorSelected === error.time) {
        showMoreButton.onclick();
      }

      errorCell.appendChild(longMessage)

    }
    return row;
  });
  if (rows.length == 0) {
    document.getElementById("hdx-error-table-no-content").style.display = 'block'
    document.getElementById("hdx-error-table").style.display = 'none'
  } else {
    document.getElementById("hdx-error-table-no-content").style.display = 'none'
    document.getElementById("hdx-error-table").style.display = 'block'
    const tBody = document.getElementById("hdx-error-table-body");
    if (tBody) {
      tBody.innerHTML = ''
      rows.forEach(row => tBody.appendChild(row))
    }

  }
};


const refresh = (force) => {
  console.debug('update', new Date().toISOString())
  //make sure error panel still exists, if not clear refresh interfal.
  if (!document.getElementById('hdx-error-table')) {
    if (window.hdxErrorIntervalId) {
      console.log("clear interval")
      clearInterval(window.hdxErrorIntervalId)
    }
    return;
  }
  const errors = getErrors();

  const lastErrorTs = errors.map((error) => error.time).sort((a, b) => a.time - b.time)[0];
  if (force || window.hdxErrorLast != lastErrorTs) {
    window.hdxErrorLast = lastErrorTs;
    renderTableRows(errors);
  }
}

if (window.hdxErrorIntervalId) {
  clearInterval(window.hdxErrorIntervalId)
}

window.hdxErrorIntervalId = setInterval(() => refresh(false), updateInterval)
refresh(true)
