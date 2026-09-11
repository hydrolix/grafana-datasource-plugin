/**
 * Report a failed nightly compatibility run as a GitHub issue.
 *
 *   const report = require('./.github/scripts/report-nightly-failure.js');
 *   await report({ github, context, core });
 *
 * Reuses one open labelled issue so a persistently broken combination
 * accumulates comments rather than duplicate issues.
 *
 * This runs only when the nightly is already broken, so enrichment (naming
 * the failing cells) must never take down the notification.
 */

const LABEL = 'nightly-compat-failure';

// `failure` alone misses timeouts, cancellations and startup failures, which
// produced issues naming zero jobs.
const BAD_CONCLUSIONS = ['failure', 'timed_out', 'cancelled', 'startup_failure', 'action_required'];

// Reusable-workflow jobs carry the caller's name as a prefix, e.g.
// "Nightly E2E Tests / E2E - Grafana nightly (luxon=true)". These rungs track
// Grafana main, so a break is advisory rather than a plugin regression.
const ADVISORY_RE = /Grafana nightly \(luxon=/;

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

    // A job tolerated by continue-on-error still reports "failure" here —
    // only the *run* conclusion is tolerated. So advisory rungs must be
    // separated by name, not by conclusion.
    const bad = jobs
      .filter((j) => BAD_CONCLUSIONS.includes(j.conclusion))
      .map((j) => `${j.name} (${j.conclusion})`)
      .sort();

    blocking = bad.filter((n) => !ADVISORY_RE.test(n));
    advisory = bad.filter((n) => ADVISORY_RE.test(n));
  } catch (err) {
    attributionError = err.message;
    core.warning(`Could not list jobs for run ${context.runId}: ${err.message}`);
  }

  // Reached when this job runs on a green run.
  if (!blocking.length && !advisory.length && !attributionError) {
    core.notice('Nightly run reported no failing jobs; nothing to report.');
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
  } else if (!blocking.length && !advisory.length) {
    lines.push(
      '- :warning: **No job could be attributed.** The run failed but every job',
      '  reported success or was skipped. Likely a cancelled run, a timeout, or',
      '  a failure tolerated by `continue-on-error`. Open the run directly.',
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
    // an unlabelled issue is invisible to tomorrow's lookup — so the script
    // would file a fresh issue every night.
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
