/**
 * Report a failed nightly compatibility run as a GitHub issue.
 *
 * Called from .github/workflows/nightly-compat.yml via actions/github-script,
 * matching the pattern in hydrolix/turbine's send-slack-job-status.js:
 *
 *   const report = require('./.github/scripts/report-nightly-failure.js');
 *   await report({ github, context, core });
 *
 * Reuses a single open labelled issue rather than filing a new one every
 * night: a persistently broken combination should accumulate comments, not
 * inboxes full of duplicates.
 */

const LABEL = 'nightly-compat-failure';

module.exports = async ({ github, context, core }) => {
  const { owner, repo } = context.repo;

  // Jobs from called reusable workflows are included in the parent run, so
  // this sees each `E2E - Grafana <version> (luxon=<bool>)` job by name.
  const jobs = await github.paginate(
    github.rest.actions.listJobsForWorkflowRun,
    { owner, repo, run_id: context.runId, per_page: 100 }
  );

  // A job tolerated by continue-on-error reports conclusion "success", so the
  // known-red nightly rung correctly stays out of this list.
  const failed = jobs
    .filter((j) => j.conclusion === 'failure')
    .map((j) => j.name)
    .sort();

  const runUrl = `${context.serverUrl}/${owner}/${repo}/actions/runs/${context.runId}`;
  const body = [
    `Nightly compatibility run failed on \`${context.ref.replace('refs/heads/', '')}\`.`,
    '',
    `**Run:** ${runUrl}`,
    `**Failing jobs (${failed.length}):**`,
    '',
    ...failed.map((n) => `- \`${n}\``),
    '',
    'Each E2E job name carries the Grafana version and the `datetime.useLuxon`',
    'state. Download the matching `e2e-results-<version>-luxon-<bool>` artifact',
    'for the Playwright report, Grafana log, and the resolved image digest.',
  ].join('\n');

  const existing = await github.rest.issues.listForRepo({
    owner, repo, state: 'open', labels: LABEL, per_page: 1,
  });

  if (existing.data.length > 0) {
    const issue_number = existing.data[0].number;
    await github.rest.issues.createComment({ owner, repo, issue_number, body });
    core.notice(`Commented on existing issue #${issue_number}`);
    return issue_number;
  }

  // Create the label on first use; harmless if it already exists.
  await github.rest.issues.createLabel({
    owner, repo, name: LABEL, color: 'B60205',
    description: 'Nightly Grafana version/feature-toggle compatibility failure',
  }).catch(() => {});

  const created = await github.rest.issues.create({
    owner, repo,
    title: `Nightly compatibility failure (${new Date().toISOString().slice(0, 10)})`,
    labels: [LABEL],
    body,
  });
  core.notice(`Opened issue #${created.data.number}`);
  return created.data.number;
};
