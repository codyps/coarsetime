// Shared by the browser and Node's dependency-free tests.
'use strict';
(function (root) {
  const alternatives = {NowInstant: 'TimeNow', Since: 'TimeSince', Now: 'TimeNow', UnixNano: 'TimeUnixNano'};
  const environmentKey = r => JSON.stringify([r.runner, r.environment.os, r.environment.cpu, r.environment.image]);
  const seriesKey = r => JSON.stringify([environmentKey(r), r.environment.go.GOVERSION, r.harness]);
  const runKey = r => `${r.run_id}/${r.run_attempt}`;
  const newestFirst = (a, b) => b.run_id - a.run_id || b.run_attempt - a.run_attempt
    || Number(a.go_selector === 'stable') - Number(b.go_selector === 'stable');

  function selectRecords(records, {runners, go, harness, environment = 'latest'}) {
    let selected = records.filter(r => runners.includes(r.runner)
      && r.environment.go.GOVERSION === go && r.harness === harness).sort(newestFirst);
    if (environment === 'latest') {
      const latest = new Map();
      for (const record of selected) {
        if (!latest.has(record.runner)) latest.set(record.runner, environmentKey(record));
      }
      selected = selected.filter(r => environmentKey(r) === latest.get(r.runner));
    } else if (environment !== 'all') {
      selected = selected.filter(r => environmentKey(r) === environment);
    }
    // Only deduplicate Go selectors within one platform and environment.
    const seen = new Set();
    return selected.filter(r => {
      const key = JSON.stringify([runKey(r), seriesKey(r)]);
      if (seen.has(key)) return false;
      seen.add(key); return true;
    });
  }

  function aggregate(values, mode) {
    if (!values.length) throw Error('Select at least one operation');
    if (mode === 'sum') return values.reduce((a, b) => a + b, 0);
    if (mode === 'mean') return aggregate(values, 'sum') / values.length;
    if (mode === 'median') {
      const sorted = [...values].sort((a, b) => a - b), middle = Math.floor(sorted.length / 2);
      return sorted.length % 2 ? sorted[middle] : (sorted[middle - 1] + sorted[middle]) / 2;
    }
    if (mode === 'individual' && values.length === 1) return values[0];
    throw Error(`Unknown aggregation: ${mode}`);
  }

  function buildSeries(records, operations, mode) {
    const groups = new Map();
    const cases = mode === 'individual' ? operations.map(name => [name]) : operations.length ? [operations] : [];
    for (const record of [...records].sort(newestFirst)) {
      for (const names of cases) {
        // Never silently aggregate an incomplete set of operations.
        if (!names.every(name => alternatives[name] && record.summary[name] && record.summary[alternatives[name]])) continue;
        const operation = mode === 'individual' ? names[0] : `${{sum: 'Sum', mean: 'Mean', median: 'Median'}[mode]} (${names.length} operations)`;
        const key = JSON.stringify([seriesKey(record), operation]);
        if (!groups.has(key)) groups.set(key, {key, runner: record.runner, environment: environmentKey(record), operation, points: []});
        const coarse = aggregate(names.map(name => record.summary[name].ns), mode);
        // Count stdlib alternatives once per selected task, including TimeNow
        // for both instant capture and calendar time when both are selected.
        const stdlib = aggregate(names.map(name => record.summary[alternatives[name]].ns), mode);
        const speedup = mode === 'individual' ? record.summary[names[0]].speedup : stdlib / coarse;
        const allocation = mode === 'individual' ? record.summary[names[0]] : {};
        groups.get(key).points.push({record, coarse, stdlib, speedup, bytes: allocation.bytes, allocs: allocation.allocs});
      }
    }
    return [...groups.values()];
  }

  const api = {alternatives, environmentKey, runKey, newestFirst, selectRecords, aggregate, buildSeries};
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.BenchmarkData = api;
})(globalThis);
