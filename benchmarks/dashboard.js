'use strict';
const {alternatives, environmentKey, runKey, newestFirst, selectGoVersion, selectRecords, buildSeries} = BenchmarkData;
const el = id => document.getElementById(id);
let records = [], visibleSeries = [];
const checked = id => [...el(id).querySelectorAll('input:checked')].map(input => input.value);
const platformLabel = r => `${r.environment.go.GOOS}/${r.environment.go.GOARCH} · ${r.runner}`;

function options(id, entries, fallback) {
  const select = el(id), previous = select.value;
  select.replaceChildren(...entries.map(([value, label]) => {
    const option = document.createElement('option'); option.value = value; option.textContent = label; return option;
  }));
  if (entries.some(([value]) => value === previous)) select.value = previous;
  else if (entries.some(([value]) => value === fallback)) select.value = fallback;
}
function checkboxes(id, entries, defaults) {
  el(id).replaceChildren(...entries.map(([value, text]) => {
    const label = document.createElement('label'), input = document.createElement('input');
    input.type = 'checkbox'; input.value = value; input.checked = defaults.includes(value);
    input.addEventListener('change', render); label.append(input, text); return label;
  }));
}
function cell(row, value) { const td = document.createElement('td'); td.textContent = value; row.append(td); return td; }
function link(parent, text, url) {
  const a = document.createElement('a'); a.textContent = text; a.href = url; parent.append(a);
}
function evidence(parent, record) {
  link(parent, 'JSON', `data/${record.file}`); parent.append(' · ');
  link(parent, 'Raw', `data/${record.file.replace(/\.json$/, '.txt')}`); parent.append(' · ');
  link(parent, 'CI run', record.run_url);
}
function color(index) { return `hsl(${(index * 137.508 + 210) % 360} 65% 45%)`; }
function render() {
  const runners = checked('runners'), operations = checked('operations'), mode = el('aggregation').value;
  let pool = records.filter(r => runners.includes(r.runner));
  options('go', [...new Set(pool.map(r => r.environment.go.GOVERSION))].map(v => [v, v]), selectGoVersion(pool, el('go').value));
  pool = pool.filter(r => r.environment.go.GOVERSION === el('go').value);
  options('harness', [...new Set(pool.map(r => r.harness))].map(v => [v, v.slice(0, 12)]));
  pool = pool.filter(r => r.harness === el('harness').value);
  const environments = [...new Map(pool.map(r => [environmentKey(r), r])).values()];
  options('environment', [['latest', 'Latest environment per platform'], ['all', 'All environments (separate lines)'],
    ...environments.map(r => [environmentKey(r), `${platformLabel(r)} / ${r.environment.os} / ${r.environment.cpu.trim()} / image ${r.environment.image}`])]);
  const selected = selectRecords(records, {runners, go: el('go').value, harness: el('harness').value, environment: el('environment').value});
  visibleSeries = buildSeries(selected, operations, mode);
  const envIDs = new Map([...new Set(visibleSeries.map(s => s.environment))].map((key, i) => [key, i + 1]));
  for (const series of visibleSeries) {
    const multiple = new Set(visibleSeries.filter(s => s.runner === series.runner).map(s => s.environment)).size > 1;
    series.label = `${platformLabel(series.points[0].record)}${multiple ? ` · environment ${envIDs.get(series.environment)}` : ''} · ${series.operation}`;
  }
  el('measurements').replaceChildren(); el('history').replaceChildren(); el('legend').replaceChildren();
  el('metadata').textContent = '';
  const missing = runners.filter(runner => !visibleSeries.some(s => s.runner === runner));
  el('status').textContent = !runners.length ? 'Select at least one platform.'
    : !operations.length ? 'Select at least one operation.'
    : !visibleSeries.length ? 'No measurements match these filters.'
    : `${selected.length} recorded runs · ${visibleSeries.length} comparison series${missing.length ? ` · No matching data: ${missing.join(', ')}` : ''}`;
  el('latest').textContent = visibleSeries.length ? 'Latest available result per series; dates and commits may differ across platforms.' : '';
  el('aggregation-note').textContent = mode === 'individual'
    ? 'Each operation is shown separately using its recorded median ns/op and paired stdlib speedup.'
    : `${{sum: 'Sum', mean: 'Arithmetic mean', median: 'Median'}[mode]} of the selected operations’ median timings, computed separately for each platform and run. Operations have equal weight; stdlib alternatives count once per selected operation. Speedup is the aggregated stdlib time divided by aggregated coarsetime time. This is a summary, not a measured mixed workload.`;
  el('chart-note').textContent = el('metric').value === 'time'
    ? 'Solid lines: coarsetime. Dashed lines: stdlib. Lower time is better. Points share a CI-run axis across platforms.'
    : 'Stdlib/coarsetime speedup; higher is better. 1× means equal cost. Points share a CI-run axis across platforms.';
  const history = [];
  visibleSeries.forEach((series, index) => {
    const item = document.createElement('li'), swatch = document.createElement('span');
    swatch.className = 'swatch'; swatch.style.backgroundColor = color(index); item.append(swatch, series.label); el('legend').append(item);
    const point = series.points[0], row = document.createElement('tr');
    cell(row, series.label); cell(row, point.coarse.toFixed(3)); cell(row, point.stdlib.toFixed(3)); cell(row, `${point.speedup.toFixed(2)}×`);
    cell(row, point.bytes ?? '—'); cell(row, point.allocs ?? '—');
    link(cell(row, ''), `${point.record.timestamp.slice(0, 10)} / ${point.record.commits.head.slice(0, 8)}`, point.record.run_url);
    el('measurements').append(row);
    for (const p of series.points) history.push({series, point: p});
  });
  history.sort((a, b) => newestFirst(a.point.record, b.point.record) || a.series.label.localeCompare(b.series.label));
  for (const {series, point} of history) {
    const row = document.createElement('tr'), record = point.record;
    cell(row, `${record.timestamp.slice(0, 10)} / ${record.commits.head.slice(0, 8)}`); cell(row, series.label);
    cell(row, point.coarse.toFixed(3)); cell(row, point.stdlib.toFixed(3)); cell(row, `${point.speedup.toFixed(2)}×`);
    evidence(cell(row, ''), record); el('history').append(row);
  }
  el('metadata').textContent = JSON.stringify([...envIDs].map(([key, id]) => {
    const record = selected.find(r => environmentKey(r) === key);
    return {id, runner: record.runner, ...record.environment};
  }), null, 2);
  draw();
}
function draw() {
  const canvas = el('chart'), scale = devicePixelRatio || 1, width = canvas.clientWidth, height = 320;
  canvas.width = width * scale; canvas.height = height * scale;
  const ctx = canvas.getContext('2d'); ctx.scale(scale, scale);
  if (!visibleSeries.length) return;
  const speedup = el('metric').value === 'speedup';
  const runs = [...new Map(visibleSeries.flatMap(s => s.points).map(p => [runKey(p.record), p.record])).values()].sort((a, b) => -newestFirst(a, b));
  const positions = new Map(runs.map((r, i) => [runKey(r), i]));
  let max = speedup ? 1 : 0;
  for (const s of visibleSeries) for (const p of s.points) max = Math.max(max, ...(speedup ? [p.speedup] : [p.coarse, p.stdlib]));
  max *= 1.15;
  const left = 65, right = Math.max(left, width - 20), top = 20, bottom = height - 35;
  const x = record => runs.length === 1 ? (left + right) / 2 : left + positions.get(runKey(record)) / (runs.length - 1) * (right - left);
  const y = value => bottom - value / max * (bottom - top);
  ctx.font = '12px system-ui'; ctx.fillStyle = '#888'; ctx.strokeStyle = '#8885';
  for (let i = 0; i <= 4; i++) {
    const value = max * i / 4;
    ctx.fillText(`${value.toFixed(1)}${speedup ? '×' : ''}`, 5, y(value) + 4);
    ctx.beginPath(); ctx.moveTo(left, y(value)); ctx.lineTo(right, y(value)); ctx.stroke();
  }
  if (speedup) {
    ctx.setLineDash([3, 4]); ctx.beginPath(); ctx.moveTo(left, y(1)); ctx.lineTo(right, y(1)); ctx.stroke();
  }
  visibleSeries.forEach((series, index) => {
    for (const field of speedup ? ['speedup'] : ['coarse', 'stdlib']) {
      ctx.strokeStyle = color(index); ctx.fillStyle = color(index); ctx.lineWidth = 2;
      ctx.setLineDash(field === 'stdlib' ? [6, 4] : []); ctx.beginPath();
      const points = [...series.points].reverse();
      points.forEach((p, i) => i ? ctx.lineTo(x(p.record), y(p[field])) : ctx.moveTo(x(p.record), y(p[field]))); ctx.stroke();
      points.forEach(p => { ctx.beginPath(); ctx.arc(x(p.record), y(p[field]), 3, 0, 2 * Math.PI); ctx.fill(); });
    }
  });
  ctx.setLineDash([]); ctx.fillStyle = '#888'; ctx.fillText(runs[0].timestamp.slice(0, 10), left, height - 8);
  ctx.textAlign = 'right'; ctx.fillText(runs.at(-1).timestamp.slice(0, 10), right, height - 8);
}
fetch('data/index.json').then(response => { if (!response.ok) throw Error(response.status); return response.json(); }).then(data => {
  records = data.sort(newestFirst);
  if (!records.length) { el('status').textContent = 'No measurements have been published yet.'; return; }
  const platforms = [...new Map(records.map(r => [r.runner, platformLabel(r)])).entries()].sort(([a], [b]) => a.localeCompare(b));
  checkboxes('runners', platforms, [platforms.some(([value]) => value === 'ubuntu-24.04') ? 'ubuntu-24.04' : platforms[0][0]]);
  checkboxes('operations', Object.keys(alternatives).map(name => [name, name]), Object.keys(alternatives));
  for (const button of document.querySelectorAll('[data-select]')) button.addEventListener('click', () => {
    for (const input of el(button.dataset.target).querySelectorAll('input')) input.checked = button.dataset.select === 'all';
    render();
  });
  for (const id of ['go', 'harness', 'environment', 'aggregation', 'metric']) el(id).addEventListener('change', render);
  window.addEventListener('resize', draw); render();
}).catch(error => { el('status').textContent = `Could not load benchmark history: ${error.message}`; });
