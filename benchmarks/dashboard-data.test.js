'use strict';
const {test} = require('node:test');
const assert = require('node:assert/strict');
const {alternatives, environmentKey, selectRecords, aggregate, buildSeries} = require('./dashboard-data.js');

function record(runner = 'linux', run = 1, image = 'image-1') {
  return {runner, run_id: run, run_attempt: 1, go_selector: 'stable', harness: 'harness',
    environment: {go: {GOVERSION: 'go1.27.1'}, os: runner, cpu: 'cpu', image},
    summary: {NowInstant: {ns: 2, speedup: 5.1}, Since: {ns: 4, speedup: 2},
      Now: {ns: 6, speedup: 1.7}, UnixNano: {ns: 20, speedup: 1.5},
      TimeNow: {ns: 10}, TimeSince: {ns: 8}, TimeUnixNano: {ns: 30}}};
}
const filters = {runners: ['linux', 'windows'], go: 'go1.27.1', harness: 'harness'};

test('multiple platforms in one run survive deduplication', () => {
  const linux = record(), windows = record('windows');
  const duplicate = {...linux, go_selector: '1.27.x'};
  const selected = selectRecords([linux, windows, duplicate], filters);
  assert.equal(selected.length, 2);
  assert.equal(selected.find(r => r.runner === 'linux').go_selector, '1.27.x');
  assert.equal(buildSeries(selected, Object.keys(alternatives), 'individual').length, 8);
});

test('latest environment is selected independently for each platform', () => {
  const records = [record('linux', 1, 'old'), record('windows', 1, 'windows-image'), record('linux', 2, 'new')];
  assert.deepEqual(selectRecords(records, filters).map(r => r.environment.image), ['new', 'windows-image']);
  const all = selectRecords(records, {...filters, environment: 'all'});
  assert.equal(all.length, 3);
  assert.equal(buildSeries(all, ['Now'], 'individual').length, 3);
  assert.equal(selectRecords(records, {...filters, environment: environmentKey(records[0])}).length, 1);
});

test('different toolchains and harnesses never get silently substituted', () => {
  const wrongGo = record('windows'); wrongGo.environment.go.GOVERSION = 'go1.23.12';
  const wrongHarness = {...record('windows'), harness: 'old'};
  assert.equal(selectRecords([record(), wrongGo, wrongHarness], filters).length, 1);
  assert.deepEqual(selectRecords([record()], {...filters, go: 'go1.99.0'}), []);
});

test('run attempts remain separate and latest measurements come first', () => {
  const records = [record(), record('linux', 2), {...record('linux', 2), run_attempt: 2}];
  const points = buildSeries(selectRecords(records, filters), ['Now'], 'individual')[0].points;
  assert.deepEqual(points.map(p => [p.record.run_id, p.record.run_attempt]), [[2, 2], [2, 1], [1, 1]]);
});

test('sum includes each selected task and its corresponding stdlib cost', () => {
  const point = buildSeries([record()], Object.keys(alternatives), 'sum')[0].points[0];
  assert.equal(point.coarse, 32);
  assert.equal(point.stdlib, 58); // TimeNow twice, TimeSince, TimeUnixNano.
  assert.equal(point.speedup, 58 / 32);
});

test('mean and median aggregate timings, not speedups or platform samples', () => {
  const mean = buildSeries([record(), record('windows')], ['NowInstant', 'Now'], 'mean');
  assert.equal(mean.length, 2);
  assert.equal(mean[0].points[0].coarse, 4);
  assert.equal(mean[0].points[0].speedup, 2.5);
  const median = buildSeries([record()], Object.keys(alternatives), 'median')[0].points[0];
  assert.equal(median.coarse, 5);
  assert.equal(median.stdlib, 10);
  assert.equal(median.speedup, 2);
  assert.equal(aggregate([20, 2, 6], 'median'), 6);
});

test('individual mode preserves the recorded paired-ratio speedup', () => {
  const point = buildSeries([record()], ['NowInstant'], 'individual')[0].points[0];
  assert.equal(point.speedup, 5.1);
});

test('empty selections and incomplete aggregate inputs do not produce stale or partial totals', () => {
  assert.deepEqual(selectRecords([record()], {...filters, runners: []}), []);
  for (const mode of ['individual', 'sum', 'mean', 'median']) assert.deepEqual(buildSeries([record()], [], mode), []);
  const missing = record(); delete missing.summary.UnixNano;
  assert.deepEqual(buildSeries([missing], ['Now', 'UnixNano'], 'sum'), []);
  assert.equal(buildSeries([missing], ['Now', 'UnixNano'], 'individual').length, 1);
});

test('selection and aggregation do not mutate the archived data', () => {
  const records = [record(), record('linux', 2)];
  const original = structuredClone(records);
  buildSeries(selectRecords(records, filters), Object.keys(alternatives), 'median');
  assert.deepEqual(records, original);
});
