/**
 * Tests for report-nightly-failure.js. Plain node — jest.config.js is scoped
 * to src/, and the injected { github, context, core } stub trivially.
 *
 *   node .github/scripts/report-nightly-failure.test.js
 */

const assert = require('assert');

const LABEL_NAME = 'nightly-compat-failure';
const report = require('./report-nightly-failure.js');

let failures = 0;

function stubs({ jobs = [], openIssues = [], throwOn = {}, createdLabels = [LABEL_NAME] } = {}) {
  const calls = { created: [], commented: [], labels: [], listed: [], notices: [], warnings: [], failed: [] };
  const maybeThrow = (name) => {
    if (throwOn[name]) {
      const e = new Error(throwOn[name].message || 'boom');
      e.status = throwOn[name].status;
      throw e;
    }
  };
  const github = {
    paginate: async () => { maybeThrow('paginate'); return jobs; },
    rest: {
      actions: { listJobsForWorkflowRun: 'listJobsForWorkflowRun' },
      issues: {
        listForRepo: async (a) => { maybeThrow('listForRepo'); calls.listed.push(a); return { data: openIssues }; },
        createComment: async (a) => { maybeThrow('createComment'); calls.commented.push(a); },
        createLabel: async (a) => { maybeThrow('createLabel'); calls.labels.push(a); },
        create: async (a) => {
          maybeThrow('create');
          calls.created.push(a);
          return { data: { number: 42, labels: createdLabels.map((name) => ({ name })) } };
        },
      },
    },
  };
  const core = {
    notice: (m) => calls.notices.push(m),
    warning: (m) => calls.warnings.push(m),
    setFailed: (m) => calls.failed.push(m),
  };
  const context = {
    repo: { owner: 'hydrolix', repo: 'grafana-datasource-plugin' },
    runId: 999, serverUrl: 'https://github.com', ref: 'refs/heads/develop',
  };
  return { github, core, context, calls };
}

const job = (name, conclusion, steps = []) => ({ name, conclusion, steps });
// A nightly rung whose e2e step failed but was tolerated: the JOB reports
// success, which is the only shape the real API can now produce.
const tolerated = (name) => job(name, 'success', [{ name: 'Run E2E tests', conclusion: 'failure' }]);
const RELEASED = 'Nightly E2E Tests / E2E - Grafana 13.2.1 (luxon=false)';
const NIGHTLY = 'Nightly E2E Tests / E2E - Grafana nightly (luxon=true)';

async function t(desc, fn) {
  try {
    await fn();
    console.log(`ok   - ${desc}`);
  } catch (err) {
    console.log(`FAIL - ${desc}\n       ${err.message}`);
    failures++;
  }
}

(async () => {
  await t('files a new issue when none is open', async () => {
    const s = stubs({ jobs: [job(RELEASED, 'failure')] });
    const n = await report(s);
    assert.strictEqual(n, 42);
    assert.strictEqual(s.calls.created.length, 1);
    assert.match(s.calls.created[0].body, /Blocking failures \(1\)/);
    assert.match(s.calls.created[0].body, /13\.2\.1 \(luxon=false\)/);
    // The run URL is the most actionable line in the issue.
    assert.match(s.calls.created[0].body, /actions\/runs\/999/);
    // The de-dup label must actually be requested, or tomorrow's lookup
    // misses this issue and files another.
    assert.ok(s.calls.created[0].labels.includes(LABEL_NAME));
    // ...and the lookup must be scoped to it, or it comments on an
    // unrelated open issue every night.
    assert.strictEqual(s.calls.listed[0].labels, LABEL_NAME);
    assert.strictEqual(s.calls.listed[0].state, 'open');
  });

  await t('comments on an existing open issue instead of filing a duplicate', async () => {
    const s = stubs({ jobs: [job(RELEASED, 'failure')], openIssues: [{ number: 9 }] });
    const n = await report(s);
    assert.strictEqual(n, 9);
    assert.strictEqual(s.calls.created.length, 0);
    assert.strictEqual(s.calls.commented.length, 1);
    // The comment body is the common case; an empty one would pass a
    // length-only assertion.
    assert.match(s.calls.commented[0].body, /13\.2\.1 \(luxon=false\)/);
  });

  await t('ignores a pull request carrying the label', async () => {
    const s = stubs({
      jobs: [job(RELEASED, 'failure')],
      openIssues: [{ number: 7, pull_request: { url: 'x' } }],
    });
    const n = await report(s);
    assert.strictEqual(n, 42, 'should have created an issue, not commented on the PR');
    assert.strictEqual(s.calls.commented.length, 0);
  });

  await t('a tolerated e2e step failure is advisory, not blocking', async () => {
    const s = stubs({ jobs: [job(RELEASED, 'failure'), tolerated(NIGHTLY)] });
    await report(s);
    const body = s.calls.created[0].body;
    assert.match(body, /Blocking failures \(1\)/);
    assert.match(body, /Advisory — unreleased Grafana \(1\)/);
    assert.match(body, /tolerated Run E2E tests failure/);
  });

  await t('a nightly job failing OUTSIDE the tolerated step stays blocking', async () => {
    // Compose bring-up, the health gate or the toggle check failing on the
    // nightly rung is infrastructure — excusing it as "unreleased Grafana"
    // would bury the highest-value signal the canary produces.
    const s = stubs({ jobs: [job(NIGHTLY, 'failure')] });
    await report(s);
    const body = s.calls.created[0].body;
    assert.match(body, /Blocking failures \(1\)/);
    assert.doesNotMatch(body, /Advisory/);
  });

  await t('files an unattributable issue rather than returning silently', async () => {
    // The caller gates on a failed run, so "nothing found" still means
    // something broke. Returning null here filed no notification at all.
    const s = stubs({ jobs: [job(RELEASED, 'success'), job('Package Plugin', 'skipped')] });
    const n = await report(s);
    assert.strictEqual(n, 42);
    assert.match(s.calls.created[0].body, /No job could be attributed/);
  });

  await t('an unlisted conclusion is still reported (deny-list, not allow-list)', async () => {
    const s = stubs({ jobs: [job(RELEASED, 'neutral'), job(NIGHTLY, 'stale')] });
    await report(s);
    const body = s.calls.created[0].body;
    assert.match(body, /neutral/);
    assert.match(body, /stale/);
    assert.doesNotMatch(body, /No job could be attributed/);
  });

  await t('counts timed_out and cancelled, not just failure', async () => {
    const s = stubs({ jobs: [job(RELEASED, 'timed_out'), job('Package Plugin', 'cancelled')] });
    await report(s);
    const body = s.calls.created[0].body;
    assert.match(body, /timed_out/);
    assert.match(body, /cancelled/);
    assert.doesNotMatch(body, /No job could be attributed/);
  });

  await t('survives a jobs-API failure and says so in the body', async () => {
    const s = stubs({ jobs: [], throwOn: { paginate: { status: 502, message: 'Bad Gateway' } } });
    const n = await report(s);
    assert.strictEqual(n, 42);
    assert.match(s.calls.created[0].body, /Job attribution unavailable/);
    assert.match(s.calls.created[0].body, /Bad Gateway/);
    assert.ok(s.calls.warnings.length > 0);
  });

  await t('warns when the label was silently dropped', async () => {
    const s = stubs({ jobs: [job(RELEASED, 'failure')], createdLabels: [] });
    await report(s);
    assert.ok(
      s.calls.warnings.some((w) => /without the 'nightly-compat-failure' label/i.test(w)),
      `expected a de-duplication warning, got: ${JSON.stringify(s.calls.warnings)}`
    );
  });

  await t('tolerates createLabel 422 without warning', async () => {
    const s = stubs({ jobs: [job(RELEASED, 'failure')], throwOn: { createLabel: { status: 422 } } });
    await report(s);
    assert.strictEqual(s.calls.warnings.length, 0, 'already_exists is the steady state');
    assert.strictEqual(s.calls.created.length, 1);
  });

  await t('warns on a non-422 createLabel error but still files', async () => {
    const s = stubs({
      jobs: [job(RELEASED, 'failure')],
      throwOn: { createLabel: { status: 403, message: 'Forbidden' } },
    });
    await report(s);
    assert.ok(s.calls.warnings.some((w) => /createLabel/.test(w)));
    assert.strictEqual(s.calls.created.length, 1);
  });

  await t('setFailed carries the cell list when the issue cannot be filed', async () => {
    const s = stubs({
      jobs: [job(RELEASED, 'failure')],
      throwOn: { create: { status: 410, message: 'Issues are disabled' } },
    });
    await assert.rejects(() => report(s));
    assert.strictEqual(s.calls.failed.length, 1);
    assert.match(s.calls.failed[0], /13\.2\.1 \(luxon=false\)/);
    assert.match(s.calls.failed[0], /Issues are disabled/);
  });

  if (failures > 0) {
    console.log(`${failures} test(s) failed`);
    process.exit(1);
  }
  console.log('all report-nightly-failure tests passed');
})();
