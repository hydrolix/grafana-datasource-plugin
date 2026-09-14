/**
 * Report a failed nightly compatibility run as a GitHub issue, reusing one
 * open labelled issue so a persistent break accumulates comments instead of
 * duplicates. Called from nightly-compat.yml via actions/github-script.
 *
 * Runs only when the nightly is already broken, so enrichment (naming the
 * failing cells) must never take down the notification.
 */

const LABEL = 'nightly-compat-failure';

// A deny-list: an unlisted conclusion must not silently drop a job from the
// report. `null` means still running, which this job itself is.
const OK_CONCLUSIONS = ['success', 'skipped', null];

// The only step tolerated by continue-on-error (nightly-e2e.yml). A job that
// succeeded *because* of it is advisory; failing anywhere else is blocking.
const TOLERATED_STEP = 'Run E2E tests';

module.exports = async ({ github, context, core }) => {
  const { owner, repo } = context.repo;
  const runUrl = `${context.serverUrl}/${owner}/${repo}/actions/runs/${context.runId}`;

  // --- enrichment (best effort) --------------------------------------------
  let blocking = [];
  let advisory = [];
  let attributionError = null;

  try {
    const jobs = await github.paginate(
      github.rest.actions.listJobsForWorkflowRun,
      { owner, repo, run_id: context.runId, per_page: 100 }
    );

    // Tolerance is step-level, so a tolerated e2e failure leaves the JOB
    // reporting success and never appears in a conclusion-based filter.
    // Classifying by step outcome is what keeps the advisory bucket
    // reachable without excusing infrastructure failures.
    blocking = jobs
      .filter((j) => !OK_CONCLUSIONS.includes(j.conclusion))
      .map((j) => `${j.name} (${j.conclusion})`)
      .sort();

    advisory = jobs
      .filter((j) => OK_CONCLUSIONS.includes(j.conclusion))
      .filter((j) => (j.steps || []).some(
        (st) => st.name === TOLERATED_STEP && st.conclusion === 'failure'))
      .map((j) => `${j.name} (tolerated ${TOLERATED_STEP} failure)`)
      .sort();
  } catch (err) {
    attributionError = err.message;
    core.warning(`Could not list jobs for run ${context.runId}: ${err.message}`);
  }

  // The caller now runs this on every completed night, so "nothing found" is
  // the normal green case and must file nothing. (Under the previous
  // `failure()` gate this return was wrong — being invoked implied a failure.)
  if (!blocking.length && !advisory.length && !attributionError) {
    core.notice('Nightly run is clean; nothing to report.');
    return null;
  }

  // --- body ----------------------------------------------------------------
  const lines = [
    `Nightly compatibility run failed on \`${context.ref.replace('refs/heads/', '')}\`.`,
    '',
    `**Run:** ${runUrl}`,
    '',
  ];

  if (blocking.length) {
    lines.push(`**Blocking failures (${blocking.length}):**`, '');
    lines.push(...blocking.map((n) => `- \`${n}\``));
    lines.push('');
  }

  if (advisory.length) {
    lines.push(
      `**Advisory — unreleased Grafana (${advisory.length}):**`,
      '',
      ...advisory.map((n) => `- \`${n}\``),
      '',
      'These rungs track Grafana `main` and are tolerated: a break here is an',
      'early warning about an unreleased Grafana, not a plugin regression.',
      ''
    );
  }

  if (attributionError) {
    lines.push(
      '**Job attribution unavailable**',
      '',
      'The failing jobs could not be listed, so this report cannot name the',
      'broken combinations. Open the run directly.',
      '',
      '```',
      attributionError,
      '```',
      ''
    );
  }

  lines.push(
    'Each E2E job name carries the Grafana version and the `datetime.useLuxon`',
    'state. Download the matching `e2e-results-<version>-luxon-<bool>` artifact',
    'for the Playwright report, Grafana log, and the resolved image digest.'
  );
  const body = lines.join('\n');

  // --- notification (load-bearing) -----------------------------------------
  const summary = [...blocking, ...advisory].join(', ') || '(unknown)';
  try {
    const existing = await github.rest.issues.listForRepo({
      owner, repo, state: 'open', labels: LABEL, per_page: 20,
    });
    // listForRepo returns PRs too; a labelled PR would swallow every comment.
    const open = existing.data.filter((i) => !i.pull_request);

    if (open.length > 0) {
      const issue_number = open[0].number;
      await github.rest.issues.createComment({ owner, repo, issue_number, body });
      core.notice(`Commented on existing issue #${issue_number}`);
      return issue_number;
    }

    try {
      await github.rest.issues.createLabel({
        owner, repo, name: LABEL, color: 'B60205',
        description: 'Nightly Grafana version/feature-toggle compatibility failure',
      });
    } catch (err) {
      // 422 already_exists is the steady state; anything else matters because...
      if (err.status !== 422) {
        core.warning(
          `createLabel(${LABEL}) failed with ${err.status}: ${err.message}. ` +
          'The issue may be created unlabelled, which breaks nightly de-duplication.'
        );
      }
    }

    const created = await github.rest.issues.create({
      owner, repo,
      title: `Nightly compatibility failure (${new Date().toISOString().slice(0, 10)})`,
      labels: [LABEL],
      body,
    });

    // ...GitHub silently drops `labels` for a token without push access, and
    // an unlabelled issue is invisible to tomorrow's lookup.
    const got = (created.data.labels || []).map((l) => (typeof l === 'string' ? l : l.name));
    if (!got.includes(LABEL)) {
      core.warning(
        `Issue #${created.data.number} was created WITHOUT the '${LABEL}' label — ` +
        "de-duplication is broken and tomorrow's run will file another issue."
      );
    }

    core.notice(`Opened issue #${created.data.number}`);
    return created.data.number;
  } catch (err) {
    // Notification failed: get the cell list into the job log before rethrowing.
    core.setFailed(
      `Nightly run failed AND the failure report could not be filed ` +
      `(${err.status ?? ''} ${err.message}). Failing jobs: ${summary}. Run: ${runUrl}`
    );
    throw err;
  }
};

// Exposed so the test can assert it still matches the workflow step name.
module.exports.TOLERATED_STEP = TOLERATED_STEP;
