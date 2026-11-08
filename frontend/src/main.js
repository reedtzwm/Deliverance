import './style.css';
import {
  GetState, Start, Stop, AutoSelect, CheckUpdate, SelectBat, SetConfig, CancelOperation,
  QuitApp, CheckSelfUpdate, ApplySelfUpdate, AppVersion, GetPresets, SetDomains,
  SetTgProxy, OpenTgLink, SetTheme, RunDiagnostics,
} from '../wailsjs/go/main/App';
import {EventsOn, WindowMinimise, WindowToggleMaximise, WindowHide} from '../wailsjs/runtime/runtime';

const $ = (id) => document.getElementById(id);
let state = null;

// ---- навигация ----
document.querySelectorAll('.nav[data-page]').forEach((b) => {
  b.onclick = () => {
    document.querySelectorAll('.nav[data-page]').forEach((x) => x.classList.toggle('active', x === b));
    document.querySelectorAll('.page').forEach((p) => (p.hidden = p.id !== 'page-' + b.dataset.page));
  };
});
$('btnQuit').onclick = () => QuitApp();

// ---- кнопки собственной панели окна ----
$('wMin').onclick = () => WindowMinimise();
$('wMax').onclick = () => WindowToggleMaximise();
$('wClose').onclick = () => WindowHide(); // как крестик: скрыть в трей

// ---- состояние ----
function render(s) {
  state = s;
  $('ver').textContent = s.version || 'не установлена';
  $('curBat').textContent = s.selected || 'не выбрана';
  $('status').classList.toggle('on', s.running);
  $('statusText').textContent = s.running ? 'Работает' : 'Остановлено';
  $('btnToggle').textContent = s.running ? 'Остановить' : 'Запустить';
  for (const id of ['btnToggle', 'btnAuto', 'btnUpdate']) $(id).disabled = s.busy;

  const sel = $('batSelect');
  sel.innerHTML = '';
  (s.bats || []).forEach((b) => {
    const o = document.createElement('option');
    o.value = b; o.textContent = b; sel.appendChild(o);
  });
  if (s.selected) sel.value = s.selected;

  $('cbUpdate').checked = s.config.autoUpdate;
  $('cbStart').checked = s.config.autoStart;
  $('cbAutorun').checked = s.config.autoRun;
  $('cbWatchdog').checked = s.config.watchdog;
  $('cbTg').checked = s.tgRunning;
  $('btnTgOpen').disabled = !s.tgRunning;
  $('btnTgCopy').disabled = !s.tgRunning;
  tgLink = s.tgLink;
  // источник истины — настройки программы (по ним же задаётся цвет окна при запуске)
  if (s.config.theme && s.config.theme !== currentTheme()) applyTheme(s.config.theme);

  // не затираем текст, который пользователь сейчас редактирует
  if (!domainsLoaded) {
    $('domainsText').value = s.config.customDomains || '';
    $('gameFilter').value = s.config.gameFilter || 'off';
    domainsLoaded = true;
  }
}
let domainsLoaded = false;
let tgLink = '';

// ---- диагностика ----
const REASONS = {
  dns: 'не найден адрес (DNS)', timeout: 'нет ответа (таймаут)', reset: 'соединение сброшено',
  refused: 'отказано в соединении', tls: 'ошибка TLS', other: 'ошибка',
};
const groups = {}; // название -> {card, rows, sum, ok, total}
let diagDone = 0, diagTotal = 0;

function groupFor(name) {
  if (groups[name]) return groups[name];
  const card = document.createElement('div');
  card.className = 'card dgroup';
  card.innerHTML = `<div class="dhead"><b></b><span class="dsum"></span></div><div class="drows"></div>`;
  card.querySelector('b').textContent = name;
  $('diagGroups').appendChild(card);
  return (groups[name] = {card, rows: card.querySelector('.drows'), sum: card.querySelector('.dsum'), ok: 0, total: 0});
}
function addRow(g, it) {
  g.total++;
  if (it.ok) g.ok++;
  const row = document.createElement('div');
  row.className = 'drow ' + (it.ok ? 'ok' : 'fail');
  row.innerHTML = `<span class="dmark"></span><span class="dname"></span><span class="dres"></span>`;
  row.querySelector('.dname').textContent = it.name;
  row.querySelector('.dres').textContent = it.ok ? `${it.ms} мс` : (REASONS[it.reason] || 'ошибка');
  g.rows.appendChild(row);
  g.sum.textContent = `${g.ok}/${g.total}`;
  g.sum.classList.toggle('good', g.ok === g.total);
}

EventsOn('diag', (e) => {
  if (e.type === 'start') {
    $('diagGroups').innerHTML = '';
    for (const k of Object.keys(groups)) delete groups[k];
    $('diagHints').hidden = true;
    $('btnDiag').disabled = true;
    diagDone = 0; diagTotal = e.total;
    $('diagProgress').hidden = false;
    $('diagFill').style.width = '0';
    $('diagStatus').innerHTML = '';
    const s = document.createElement('div');
    s.innerHTML = `<b>Обход (zapret):</b> <span></span> &nbsp;·&nbsp; <b>Прокси Telegram:</b> <span></span>`;
    const sp = s.querySelectorAll('span');
    sp[0].textContent = e.bypass ? 'работает' : 'выключен';
    sp[1].textContent = e.tg ? 'включён' : 'выключен';
    $('diagStatus').appendChild(s);
  } else if (e.type === 'item') {
    addRow(groupFor(e.item.group), e.item);
    diagDone++;
    $('diagFill').style.width = (diagDone / Math.max(1, diagTotal) * 100) + '%';
  } else if (e.type === 'done') {
    $('btnDiag').disabled = false;
    $('diagProgress').hidden = true;
    const box = $('diagHints');
    box.innerHTML = '';
    (e.hints || []).forEach((h) => {
      const p = document.createElement('p');
      p.textContent = h;
      box.appendChild(p);
    });
    box.hidden = false;
  }
});
$('btnDiag').onclick = () => RunDiagnostics();

// окно диагностики открывается поверх приложения, а не отдельным окном ОС
function openDiag() { $('diagModal').hidden = false; }
function closeDiag() { $('diagModal').hidden = true; }
$('btnDiagOpen').onclick = openDiag;
$('btnDiagClose').onclick = closeDiag;
$('diagModal').addEventListener('mousedown', (e) => { if (e.target === $('diagModal')) closeDiag(); });
document.addEventListener('keydown', (e) => { if (e.key === 'Escape' && !$('diagModal').hidden) closeDiag(); });

// ---- тема ----
const darkMq = matchMedia('(prefers-color-scheme: dark)');
function applyTheme(t) {
  const dark = t === 'dark' || (t === 'auto' && darkMq.matches);
  document.documentElement.dataset.theme = dark ? 'dark' : 'light';
  try { localStorage.setItem('theme', t); } catch {}
  $('themeSelect').value = t;
}
function currentTheme() {
  try { return localStorage.getItem('theme') || 'auto'; } catch { return 'auto'; }
}
darkMq.addEventListener('change', () => applyTheme(currentTheme()));
$('themeSelect').onchange = (e) => { applyTheme(e.target.value); SetTheme(e.target.value); };
applyTheme(currentTheme());

$('cbTg').onchange = (e) => SetTgProxy(e.target.checked);
$('btnTgOpen').onclick = () => OpenTgLink();
$('btnTgCopy').onclick = async () => {
  try {
    await navigator.clipboard.writeText(tgLink);
    $('tgText').textContent = 'Ссылка скопирована';
  } catch {
    $('tgText').textContent = tgLink; // если буфер недоступен — показываем ссылку
  }
};

function log(line) {
  const el = $('log');
  el.textContent += line + '\n';
  el.scrollTop = el.scrollHeight;
}

EventsOn('state', render);
EventsOn('log', log);
EventsOn('progress', (p) => {
  const box = $('progress');
  if (!p.n) { box.hidden = true; return; }
  box.hidden = false;
  $('barFill').style.width = (p.i / p.n * 100) + '%';
  $('ptext').textContent = `Проверяю ${p.name} (${p.i}/${p.n})`;
});

// ---- действия ----
$('btnToggle').onclick = () => (state && state.running ? Stop() : Start());
$('btnAuto').onclick = () => AutoSelect();
$('btnUpdate').onclick = () => CheckUpdate();
$('btnCancel').onclick = () => CancelOperation();
$('batSelect').onchange = (e) => SelectBat(e.target.value);

function pushConfig() {
  SetConfig({
    selectedBat: '',
    autoUpdate: $('cbUpdate').checked,
    autoStart: $('cbStart').checked,
    autoSelect: true,
    autoRun: $('cbAutorun').checked,
    watchdog: $('cbWatchdog').checked,
  });
}
['cbUpdate', 'cbStart', 'cbAutorun', 'cbWatchdog'].forEach((id) => ($(id).onchange = pushConfig));

// ---- игры и домены ----
let presets = [];
GetPresets().then((list) => {
  presets = list;
  const sel = $('presetSelect');
  list.forEach((p) => {
    const o = document.createElement('option');
    o.value = p.id; o.textContent = p.name; sel.appendChild(o);
  });
});
$('btnPreset').onclick = () => {
  const p = presets.find((x) => x.id === $('presetSelect').value);
  if (!p) return;
  const ta = $('domainsText');
  const have = new Set(ta.value.split(/\s+/).filter(Boolean));
  const add = p.domains.filter((d) => !have.has(d));
  ta.value = (ta.value.trim() ? ta.value.trim() + '\n' : '') + add.join('\n');
  $('domMsg').textContent = `Добавлено: ${p.name} (${add.length}). Нажмите «Сохранить и применить».`;
};
$('btnDomains').onclick = async () => {
  $('domMsg').textContent = await SetDomains($('domainsText').value, $('gameFilter').value);
  domainsLoaded = false; // перечитать нормализованный список
};

// ---- обновление самой программы (заготовка) ----
let selfInfo = null;
async function refreshSelf(apply) {
  const t = $('selfText');
  if (apply && selfInfo && selfInfo.available) {
    t.textContent = await ApplySelfUpdate();
    return;
  }
  selfInfo = await CheckSelfUpdate();
  if (!selfInfo.configured) {
    t.textContent = `Версия ${selfInfo.current} · источник обновлений пока не настроен`;
    $('btnSelf').disabled = true;
  } else if (selfInfo.error) {
    t.textContent = 'Ошибка проверки: ' + selfInfo.error;
  } else if (selfInfo.available) {
    t.textContent = `Доступна версия ${selfInfo.latest} (сейчас ${selfInfo.current})`;
    $('btnSelf').textContent = 'Обновить';
  } else {
    t.textContent = `Версия ${selfInfo.current} · установлена последняя`;
  }
}
$('btnSelf').onclick = () => refreshSelf(true);

GetState().then(render);
AppVersion().then((v) => ($('selfText').textContent = `Версия ${v}`));
