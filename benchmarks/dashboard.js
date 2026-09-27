'use strict';
const alternatives = {NowInstant: 'TimeNow', Since: 'TimeSince', Now: 'TimeNow', UnixNano: 'TimeUnixNano'};
const el = id => document.getElementById(id);
let records = [];
const envKey = r => JSON.stringify([r.environment.os, r.environment.cpu, r.environment.image]);
function options(id, values, label = x => x) {
  const select = el(id), previous = select.value;
  select.replaceChildren(...[...new Set(values)].map(value => {
    const option = document.createElement('option'); option.value = value; option.textContent = label(value); return option;
  }));
  if ([...select.options].some(o => o.value === previous)) select.value = previous;
}
function cell(row, value) { const td = document.createElement('td'); td.textContent = value; row.append(td); return td; }
function link(parent, text, url) {
  const a = document.createElement('a'); a.textContent = text; a.href = url; parent.append(a);
}
function render() {
  let selected = records.filter(r => r.runner === el('runner').value);
  options('go', selected.map(r => r.environment.go.GOVERSION));
  selected = selected.filter(r => r.environment.go.GOVERSION === el('go').value);
  options('harness', selected.map(r => r.harness), h => h.slice(0, 12));
  selected = selected.filter(r => r.harness === el('harness').value);
  options('environment', selected.map(envKey), key => { const [os, , image] = JSON.parse(key); return `${os} / image ${image}`; });
  selected = selected.filter(r => envKey(r) === el('environment').value);
  // Same exact toolchain may appear in both the stable and numbered lanes.
  selected = selected.filter((r, i, all) => all.findIndex(x => x.run_id === r.run_id && x.run_attempt === r.run_attempt) === i);
  const latest = selected[0], name = el('operation').value, std = alternatives[name];
  if (!latest) return;
  el('status').textContent = `${records.length} recorded platform/toolchain runs · ${selected.length} in this selection`;
  el('latest').replaceChildren();
  link(el('latest'), `${latest.timestamp} · ${latest.commits.head.slice(0, 12)}`, latest.run_url);
  el('measurements').replaceChildren();
  for (const [operation, alternate] of Object.entries(alternatives)) {
    const row = document.createElement('tr'), value = latest.summary[operation];
    [operation, value.ns.toFixed(3), latest.summary[alternate].ns.toFixed(3), `${value.speedup.toFixed(2)}×`, value.bytes, value.allocs].forEach(v => cell(row, v));
    el('measurements').append(row);
  }
  el('history').replaceChildren();
  for (const record of selected) {
    const row = document.createElement('tr');
    cell(row, `${record.timestamp.slice(0, 10)} / ${record.commits.head.slice(0, 8)}`);
    cell(row, record.summary[name].ns.toFixed(3)); cell(row, record.summary[std].ns.toFixed(3));
    cell(row, `${record.summary[name].speedup.toFixed(2)}×`);
    const evidence = cell(row, '');
    link(evidence, 'JSON', `data/${record.file}`); evidence.append(' · ');
    link(evidence, 'Raw', `data/${record.file.replace(/\.json$/, '.txt')}`); evidence.append(' · ');
    link(evidence, 'CI run', record.run_url);
    el('history').append(row);
  }
  el('metadata').textContent = JSON.stringify(latest.environment, null, 2);
  draw([...selected].reverse(), name, std);
}
function draw(rows, name, std) {
  const canvas = el('chart'), scale = devicePixelRatio || 1;
  const width = canvas.clientWidth, height = 280;
  canvas.width = width * scale; canvas.height = height * scale;
  const ctx = canvas.getContext('2d'); ctx.scale(scale, scale);
  const max = Math.max(...rows.flatMap(r => [r.summary[name].ns, r.summary[std].ns])) * 1.15;
  const left = 65, right = width - 20, top = 20, bottom = height - 35;
  ctx.font = '12px system-ui'; ctx.fillStyle = '#888'; ctx.strokeStyle = '#8885';
  for (let i = 0; i <= 4; i++) {
    const y = bottom - i / 4 * (bottom - top);
    ctx.fillText((max * i / 4).toFixed(1), 5, y + 4);
    ctx.beginPath(); ctx.moveTo(left, y); ctx.lineTo(right, y); ctx.stroke();
  }
  for (const [operation, color] of [[name, '#328ad6'], [std, '#de8c35']]) {
    ctx.strokeStyle = color; ctx.fillStyle = color; ctx.lineWidth = 2; ctx.beginPath();
    const points = rows.map((r, i) => [left + i / Math.max(rows.length - 1, 1) * (right - left), bottom - r.summary[operation].ns / max * (bottom - top)]);
    points.forEach(([x, y], i) => i ? ctx.lineTo(x, y) : ctx.moveTo(x, y)); ctx.stroke();
    points.forEach(([x, y]) => { ctx.beginPath(); ctx.arc(x, y, 3, 0, 2 * Math.PI); ctx.fill(); });
  }
  ctx.fillStyle = '#888'; ctx.fillText(rows[0].timestamp.slice(0, 10), left, height - 8);
  ctx.textAlign = 'right'; ctx.fillText(rows.at(-1).timestamp.slice(0, 10), right, height - 8);
}
fetch('data/index.json').then(response => { if (!response.ok) throw Error(response.status); return response.json(); }).then(data => {
  records = data.reverse();
  if (!records.length) { el('status').textContent = 'No measurements have been published yet.'; return; }
  options('runner', records.map(r => r.runner)); options('operation', Object.keys(alternatives));
  for (const id of ['runner', 'go', 'harness', 'environment', 'operation']) el(id).addEventListener('change', render);
  window.addEventListener('resize', render); render();
}).catch(error => { el('status').textContent = `Could not load benchmark history: ${error.message}`; });
