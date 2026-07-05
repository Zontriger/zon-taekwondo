/* ============================================================================
   SIGAT — Sistema de Gestión de Atletas de Taekwondo (Vanilla JS).
   ============================================================================ */
'use strict';

const $  = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => [...r.querySelectorAll(s)];

const state = {
  me: null,
  cat: { cinturones: [], escuelas: [], maestros: [], campos_filtro: [] },
  geo: { estados: [], ciudades: [], municipios: [], parroquias: [] },
  filtro: { q: '', match: 'all', cond: [] },
  pageSize: 25,
  offset: 0,
  total: 0,
  editId: null,
  route: 'atletas',
  maestroTipo: 'estados',
  maestroData: [],
};

const BELT_COLORS = {
  'Blanco': '#ffffff', 'Amarillo': '#E5A93C', 'Naranja': '#E8792B',
  'Verde': '#1E5631', 'Azul': '#0077B6', 'Rojo': '#8B0000', 'Negro': '#141414',
};
const MESES = ['Enero','Febrero','Marzo','Abril','Mayo','Junio','Julio','Agosto','Septiembre','Octubre','Noviembre','Diciembre'];
const PARENTESCOS_COMUNES = ['Madre','Padre','Tutor legal','Abuelo/a','Tío/a','Hermano/a'];
const ADMIN_ROUTES = ['escuelas','entrenadores','usuarios','datos','respaldo'];
const TIPOS_SANGRE = ['O+','O-','A+','A-','B+','B-','AB+','AB-'];
const NAME_ALLOW = /[\p{L} .'\-]/u;
const PLACE_ALLOW = /[\p{L}0-9 .,'\-/()]/u;
const USER_ALLOW = /[A-Za-z0-9._\-]/;

// Nombres legibles (display_name) para el respaldo por tabla.
const TABLA_LABEL = {
  atleta: 'Atletas', representante: 'Representantes',
  atleta_telefono_contacto: 'Teléfonos de contacto de atletas',
  historial_cinturon: 'Historial de cinturones', periodo_actividad: 'Periodos de actividad',
  escuela: 'Escuelas', estado: 'Estados', ciudad: 'Ciudades', municipio: 'Municipios',
  parroquia: 'Parroquias', cinturon: 'Cinturones', entrenador: 'Usuarios del sistema',
};
const TABLAS_RESPALDO = Object.keys(TABLA_LABEL);
// Etiqueta singular para el título del modal de datos maestros.
const MAESTRO_SINGULAR = { estados: 'estado', ciudades: 'ciudad', municipios: 'municipio', parroquias: 'parroquia', cinturones: 'cinturón' };

const svg = (p) => `<svg class="ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${p}</svg>`;
const ICON = {
  edit:      svg('<path d="M12 20h9"/><path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4Z"/>'),
  belt:      svg('<circle cx="12" cy="8" r="6"/><path d="M15.5 13.5 17 22l-5-3-5 3 1.5-8.5"/>'),
  retire:    svg('<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><line x1="17" y1="11" x2="23" y2="11"/>'),
  reactivate:svg('<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><line x1="20" y1="8" x2="20" y2="14"/><line x1="23" y1="11" x2="17" y2="11"/>'),
  trash:     svg('<polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>'),
  chevron:   svg('<polyline points="9 18 15 12 9 6"/>'),
  chevronL:  svg('<polyline points="15 18 9 12 15 6"/>'),
  arrow:     svg('<line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/>'),
  x:         svg('<line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>'),
  pdf:       svg('<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/>'),
  download:  svg('<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/>'),
};
const STATE_DOT = '<svg class="ico-dot" viewBox="0 0 8 8"><circle cx="4" cy="4" r="4" fill="currentColor"/></svg>';

/* ---------------------- API ---------------------- */
async function api(path, opts = {}) {
  const res = await fetch(path, { headers: { 'Content-Type': 'application/json' }, ...opts });
  if (res.status === 401) { mostrarLogin(); throw new Error('no autenticado'); }
  let data = null;
  const txt = await res.text();
  if (txt) { try { data = JSON.parse(txt); } catch { data = txt; } }
  if (!res.ok) {
    const err = new Error((data && data.error) || res.statusText);
    if (data && data.errors) err.fields = data.errors;
    throw err;
  }
  return data;
}

/* ---------------------- Toast ---------------------- */
let toastTimer, toastHideT;
function toast(msg, kind = 'ok') {
  const t = $('#toast');
  clearTimeout(toastTimer); clearTimeout(toastHideT);
  t.className = 'toast ' + kind;
  t.innerHTML = `<span class="toast-msg"></span><button type="button" class="toast-x" aria-label="Cerrar">${ICON.x}</button>`;
  t.querySelector('.toast-msg').textContent = msg;
  t.querySelector('.toast-x').onclick = ocultarToast;
  t.classList.remove('hidden');
  void t.offsetWidth;            // reflow: anima desde el estado oculto
  t.classList.add('show');
  toastTimer = setTimeout(ocultarToast, 3400);
}
function ocultarToast() {
  const t = $('#toast');
  clearTimeout(toastTimer);
  t.classList.remove('show');
  toastHideT = setTimeout(() => t.classList.add('hidden'), 220);
}

/* ---------------------- Modales base ---------------------- */
// abrir/cerrar animan la opacidad: al cerrar se espera la transición antes de
// ocultar (display:none) para evitar el "flashazo" al encadenar modales.
function abrir(m) {
  clearTimeout(m._closeT);
  m.classList.remove('hidden');
  void m.offsetWidth;           // reflow para animar desde opacity:0
  m.classList.add('show');
}
function cerrar(m) {
  clearTimeout(m._closeT);
  m.classList.remove('show');
  m._closeT = setTimeout(() => m.classList.add('hidden'), 180);
}

// confirmar: modal de confirmación propio (reemplaza confirm()).
let confirmResolve = null;
function confirmar(mensaje, opt = {}) {
  return new Promise(res => {
    confirmResolve = res;
    $('#cf-title').textContent = opt.titulo || 'Confirmar';
    $('#cf-msg').textContent = mensaje;
    $('#cf-ok').className = 'btn ' + (opt.peligro ? 'btn-danger' : 'btn-primary');
    $('#cf-ok').textContent = opt.ok || 'Aceptar';
    abrir($('#modal-confirm'));
  });
}
function resolverConfirm(v) { cerrar($('#modal-confirm')); if (confirmResolve) { confirmResolve(v); confirmResolve = null; } }
$('#cf-ok').addEventListener('click', () => resolverConfirm(true));
$('#cf-cancel').addEventListener('click', () => resolverConfirm(false));
$('#modal-confirm').addEventListener('mousedown', e => { if (e.target === $('#modal-confirm')) resolverConfirm(false); });

// pedirDatos: modal genérico de entrada (reemplaza prompt()).
let promptResolve = null, promptCampos = [];
function pedirDatos(titulo, campos) {
  return new Promise(res => {
    promptResolve = res; promptCampos = campos;
    $('#pr-title').textContent = titulo;
    $('#pr-error').textContent = '';
    $('#pr-body').innerHTML = campos.map(c => {
      const id = 'pr-f-' + c.key;
      let inner;
      if (c.type === 'select') inner = `<select id="${id}">${c.options.map(o => `<option value="${esc(o.value)}" ${String(o.value) === String(c.value ?? '') ? 'selected' : ''}>${esc(o.label)}</option>`).join('')}</select>`;
      else if (c.type === 'date') inner = `<input id="${id}" type="date" value="${esc(c.value || '')}">`;
      else if (c.type === 'number') inner = `<input id="${id}" inputmode="numeric" maxlength="${c.maxLen || 3}" value="${esc(c.value || '')}">`;
      else inner = `<input id="${id}" maxlength="${c.maxLen || 60}" value="${esc(c.value || '')}">`;
      return `<label class="pr-field" id="pr-wrap-${c.key}">${esc(c.label)}${inner}</label>`;
    }).join('');
    evalPromptCond();
    abrir($('#modal-prompt'));
  });
}
// Muestra/oculta campos del prompt según su showIf(valores).
function evalPromptCond() {
  const vals = {};
  promptCampos.forEach(c => { const el = $('#pr-f-' + c.key); if (el) vals[c.key] = el.value; });
  promptCampos.forEach(c => { if (c.showIf) { const w = $('#pr-wrap-' + c.key); if (w) w.classList.toggle('hidden', !c.showIf(vals)); } });
}
$('#pr-body').addEventListener('change', evalPromptCond);
$('#pr-body').addEventListener('input', evalPromptCond);
function resolverPrompt(v) { cerrar($('#modal-prompt')); if (promptResolve) { promptResolve(v); promptResolve = null; } }
$('#pr-cancel').addEventListener('click', () => resolverPrompt(null));
$('#pr-x').addEventListener('click', () => resolverPrompt(null));
$('#modal-prompt').addEventListener('mousedown', e => { if (e.target === $('#modal-prompt')) resolverPrompt(null); });
$('#form-prompt').addEventListener('submit', e => {
  e.preventDefault();
  const out = {};
  promptCampos.forEach(c => { out[c.key] = $('#pr-f-' + c.key).value.trim(); });
  resolverPrompt(out);
});

/* ---------------------- Auth / navegación ---------------------- */
function mostrarLogin() { $('#view-app').classList.add('hidden'); $('#view-login').classList.remove('hidden'); }
function mostrarApp()   { $('#view-login').classList.add('hidden'); $('#view-app').classList.remove('hidden'); }
function isAdmin() { return !!(state.me && state.me.es_admin); }

function renderBadge() {
  $('#user-badge').innerHTML =
    `<b class="badge-name">${esc(state.me.nombres)} ${esc(state.me.apellidos)}</b>` +
    `<span class="badge-rol ${state.me.es_admin ? 'badge-admin' : ''}">${state.me.es_admin ? 'ADMIN' : 'CONSULTOR'}</span>`;
}
function aplicarPermisos() { document.body.classList.toggle('rol-consultor', !isAdmin()); }

async function iniciar() {
  try { state.me = await api('/api/me'); await trasLogin(); }
  catch { mostrarLogin(); }
}
async function trasLogin() {
  renderBadge(); aplicarPermisos(); mostrarApp();
  await cargarCatalogos(); await cargarGeo();
  renderAdvRows(); await cargarLista();
  routerInit(); conectarWS();
}

$('#login-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  $('#login-error').textContent = '';
  try {
    state.me = await api('/api/login', { method: 'POST', body: JSON.stringify({ username: $('#login-user').value, password: $('#login-pass').value }) });
    $('#login-pass').value = '';
    await trasLogin();
  } catch (err) { $('#login-error').textContent = err.message; }
});
$('#btn-logout').addEventListener('click', async () => {
  try { await api('/api/logout', { method: 'POST' }); } catch {}
  if (ws) ws.close();
  mostrarLogin();
});

/* ---------------------- Router ---------------------- */
function routerInit() {
  $$('#sidebar .nav-item[data-route]').forEach(a => a.addEventListener('click', () => irARuta(a.dataset.route)));
  $('#btn-menu').addEventListener('click', () => toggleSidebar());
  $('#sidebar-backdrop').addEventListener('click', () => toggleSidebar(false));
  window.addEventListener('hashchange', () => mostrarRuta(rutaDeHash()));
  mostrarRuta(rutaDeHash());
}
function rutaDeHash() { const r = (location.hash || '').replace(/^#\/?/, ''); return r || 'atletas'; }
function irARuta(route) { location.hash = '#/' + route; }
function toggleSidebar(force) {
  const open = force === undefined ? !$('#sidebar').classList.contains('open') : force;
  $('#sidebar').classList.toggle('open', open);
  $('#sidebar-backdrop').classList.toggle('hidden', !open);
}
function mostrarRuta(route) {
  if (ADMIN_ROUTES.includes(route) && !isAdmin()) route = 'atletas';
  if (!$('#sec-' + route)) route = 'atletas';
  state.route = route;
  $$('.route-sec').forEach(s => s.classList.add('hidden'));
  $('#sec-' + route).classList.remove('hidden');
  $$('#sidebar .nav-item[data-route]').forEach(a => a.classList.toggle('active', a.dataset.route === route));
  toggleSidebar(false);
  if (route === 'escuelas') cargarEscuelas();
  else if (route === 'entrenadores') cargarEntrenadores();
  else if (route === 'usuarios') cargarUsuarios();
  else if (route === 'datos') cargarMaestro(state.maestroTipo);
}

/* ---------------------- Catálogos + Geografía ---------------------- */
async function cargarCatalogos() {
  state.cat = await api('/api/catalogos');
  const { cinturones, escuelas } = state.cat;
  llenarSelect($('#f-escuela'), escuelas, 'Todas las escuelas', e => e.nombre);
  llenarSelect($('#f-cinturon'), cinturones, 'Todos los cinturones', c => c.color);
  llenarSelect($('#a-escuela'), escuelas, 'No aplica', e => `${e.nombre} (${e.municipio_nombre})`);
  llenarSelect($('#a-cinturon'), cinturones, 'No aplica', c => c.color);
  llenarSelect($('#a-maestro'), state.cat.maestros, 'No aplica', m => `${m.nombres} ${m.apellidos}`);
  llenarSelect($('#ent-escuela'), escuelas, 'No aplica', e => `${e.nombre} (${e.municipio_nombre})`);
  llenarSelect($('#ent-cinturon'), cinturones, 'No aplica', c => c.color);
}
async function cargarGeo() {
  state.geo = await api('/api/geo');
  llenarSelect($('#f-municipio'), state.geo.municipios, 'Todos los municipios', m => m.nombre);
}
function llenarSelect(sel, items, placeholder, label) {
  sel.innerHTML = `<option value="">${placeholder}</option>` + (items || []).map(i => `<option value="${i.id}">${esc(label(i))}</option>`).join('');
}
function cinturonPorId(id) { return state.cat.cinturones.find(c => c.id == id); }
// ID del cinturón Blanco (por defecto en atletas nuevos); si no existe, el de menor orden.
function cinturonBlancoId() {
  const cs = state.cat.cinturones || [];
  const blanco = cs.find(c => /blanco/i.test(c.color));
  const first = [...cs].sort((a, b) => (a.orden ?? 0) - (b.orden ?? 0))[0];
  return String((blanco || first || {}).id || '');
}
function porId(arr, id) { return (arr || []).find(x => String(x.id) === String(id)); }
function nombreGeo(kind, id) { const o = porId(state.geo[kind], id); return o ? o.nombre : '—'; }

const CAT_OPTS = {
  escuelas:   () => state.cat.escuelas.map(e => ({ id: e.id, label: e.nombre })),
  maestros:   () => (state.cat.maestros || []).map(m => ({ id: m.id, label: `${m.nombres} ${m.apellidos}` })),
  cinturones: () => state.cat.cinturones.map(c => ({ id: c.id, label: c.color })),
  estados:    () => state.geo.estados.map(e => ({ id: e.id, label: e.nombre })),
  ciudades:   () => state.geo.ciudades.map(c => ({ id: c.id, label: c.nombre })),
  municipios: () => state.geo.municipios.map(m => ({ id: m.id, label: m.nombre })),
  parroquias: () => state.geo.parroquias.map(p => ({ id: p.id, label: p.nombre })),
};

/* ============================================================================
   Cascada geográfica reutilizable
   ============================================================================ */
function geoWire(g, onChange) {
  Object.entries(g).forEach(([nivel, el]) => { if (el) el.addEventListener('change', () => { geoRefresh(g, nivel); if (onChange) onChange(); }); });
}
function geoRefresh(g, changed) {
  const v = { estado: g.estado ? g.estado.value : '', ciudad: g.ciudad ? g.ciudad.value : '', municipio: g.municipio ? g.municipio.value : '', parroquia: g.parroquia ? g.parroquia.value : '' };
  if (changed === 'parroquia' && v.parroquia) { const p = porId(state.geo.parroquias, v.parroquia); if (p) v.municipio = String(p.municipio_id); }
  if ((changed === 'parroquia' || changed === 'municipio') && v.municipio) { const m = porId(state.geo.municipios, v.municipio); if (m) { v.estado = String(m.estado_id); v.ciudad = m.ciudad_id ? String(m.ciudad_id) : ''; } }
  if (changed === 'ciudad') { if (v.ciudad) { const c = porId(state.geo.ciudades, v.ciudad); if (c) v.estado = String(c.estado_id); } v.municipio = ''; v.parroquia = ''; }
  if (changed === 'estado') { v.ciudad = ''; v.municipio = ''; v.parroquia = ''; }
  if (changed === 'municipio') v.parroquia = '';
  geoPopulate(g, v);
}
function geoPopulate(g, v) {
  if (g.estado) fillSel(g.estado, state.geo.estados, e => e.nombre, v.estado);
  if (g.ciudad) { const items = v.estado ? state.geo.ciudades.filter(c => String(c.estado_id) === v.estado) : state.geo.ciudades; fillSel(g.ciudad, items, c => c.nombre, v.ciudad); }
  if (g.municipio) { let items = state.geo.municipios; if (v.ciudad) items = items.filter(m => String(m.ciudad_id) === v.ciudad); else if (v.estado) items = items.filter(m => String(m.estado_id) === v.estado); fillSel(g.municipio, items, m => m.nombre, v.municipio); }
  if (g.parroquia) { let items = state.geo.parroquias; if (v.municipio) items = items.filter(p => String(p.municipio_id) === v.municipio); else if (v.estado) items = items.filter(p => String(p.estado_id) === v.estado); fillSel(g.parroquia, items, p => p.nombre, v.parroquia); }
}
function fillSel(sel, items, labelFn, selected) {
  sel.innerHTML = `<option value="">No aplica</option>` + items.map(i => `<option value="${i.id}">${esc(labelFn(i))}</option>`).join('');
  sel.value = selected != null ? String(selected) : '';
}
function geoSet(g, loc) {
  geoPopulate(g, { estado: loc.estado_id ? String(loc.estado_id) : '', ciudad: loc.ciudad_id ? String(loc.ciudad_id) : '', municipio: loc.municipio_id ? String(loc.municipio_id) : '', parroquia: loc.parroquia_id ? String(loc.parroquia_id) : '' });
}
function geoRead(g) { const num = (el) => (el && el.value ? parseInt(el.value, 10) : null); return { estado_id: num(g.estado), ciudad_id: num(g.ciudad), municipio_id: num(g.municipio), parroquia_id: num(g.parroquia) }; }

/* ============================================================================
   Componente DataTable (búsqueda + filtro por columna + paginación + vacíos)
   ============================================================================ */
function DataTable(opts) {
  const mount = typeof opts.mount === 'string' ? $(opts.mount) : opts.mount;
  const cols = opts.columns;
  const st = { data: [], search: '', colf: {}, offset: 0, pageSize: opts.pageSize || 15, showFilters: false };
  mount.className = 'dt';
  const filtrables = cols.map((c, i) => ({ c, i })).filter(x => x.c.filter !== false);
  mount.innerHTML = `
    <div class="dt-bar">
      <input class="dt-search grow" type="search" maxlength="40" placeholder="Buscar…">
      <button type="button" class="btn btn-ghost btn-sm dt-ftoggle">Filtros por columna</button>
    </div>
    <div class="dt-colfilters hidden">${filtrables.map(x => {
      if (x.c.type === 'enum' || x.c.type === 'bool') {
        const os = x.c.type === 'bool' ? ['Sí', 'No'] : (x.c.options || []);
        return `<label class="dt-colf">${esc(x.c.label)}<select data-cf="${x.i}"><option value="">Todos</option>${os.map(o => `<option value="${esc(o)}">${esc(o)}</option>`).join('')}</select></label>`;
      }
      return `<label class="dt-colf">${esc(x.c.label)}<input data-cf="${x.i}" maxlength="30"></label>`;
    }).join('')}</div>
    <div class="table-wrap"><table class="tabla">
      <thead><tr>${cols.map(c => `<th>${esc(c.label)}</th>`).join('')}${opts.actions ? '<th></th>' : ''}</tr></thead>
      <tbody class="dt-tbody"></tbody></table>
      <div class="empty dt-empty hidden"></div>
    </div>
    <div class="pager">
      <button class="btn btn-ghost btn-sm dt-prev" title="Anterior">${ICON.chevronL}</button>
      <span class="pg-range"><input class="pg-input dt-desde" inputmode="numeric"><span>–</span><input class="pg-input dt-hasta" inputmode="numeric"><span>de <b class="dt-total">0</b></span></span>
      <button class="btn btn-ghost btn-sm dt-next" title="Siguiente">${ICON.chevron}</button>
    </div>`;
  const q = (s) => mount.querySelector(s);

  q('.dt-search').addEventListener('input', () => { st.search = q('.dt-search').value.trim().toLowerCase(); st.offset = 0; render(); });
  q('.dt-ftoggle').addEventListener('click', () => { st.showFilters = !st.showFilters; q('.dt-colfilters').classList.toggle('hidden', !st.showFilters); });
  $$('[data-cf]', mount).forEach(el => { const ev = el.tagName === 'SELECT' ? 'change' : 'input'; el.addEventListener(ev, () => { st.colf[el.dataset.cf] = el.value; st.offset = 0; render(); }); });
  q('.dt-prev').addEventListener('click', () => { if (st.offset > 0) { st.offset = Math.max(0, st.offset - st.pageSize); render(); } });
  q('.dt-next').addEventListener('click', () => { const t = filtered().length; if (st.offset + st.pageSize < t) { st.offset += st.pageSize; render(); } });
  const commit = () => {
    const total = filtered().length;
    const d = parseInt(q('.dt-desde').value, 10), h = parseInt(q('.dt-hasta').value, 10);
    const inval = !Number.isInteger(d) || !Number.isInteger(h) || d < 1 || h < d || (total > 0 && d > total);
    q('.dt-desde').classList.toggle('invalid', !(d >= 1 && (total === 0 || d <= total)));
    q('.dt-hasta').classList.toggle('invalid', !(h >= d));
    if (inval) return;
    st.offset = d - 1; st.pageSize = Math.max(1, h - d + 1); render();
  };
  ['.dt-desde', '.dt-hasta'].forEach(s => { q(s).addEventListener('change', commit); q(s).addEventListener('keydown', e => { if (e.key === 'Enter') { e.preventDefault(); commit(); } }); limitInput(q(s), { allow: /[0-9]/, maxLen: 7 }); });
  if (opts.actions) q('.dt-tbody').addEventListener('click', e => {
    const ed = e.target.closest('[data-edit]'), de = e.target.closest('[data-del]'), vr = e.target.closest('[data-ver]');
    if (ed && opts.onEdit) opts.onEdit(porId(st.data, ed.dataset.edit));
    if (de && opts.onDelete) opts.onDelete(porId(st.data, de.dataset.del));
    if (vr && opts.onVer) opts.onVer(porId(st.data, vr.dataset.ver));
  });

  function filtered() {
    let rows = st.data;
    if (st.search) rows = rows.filter(r => cols.some(c => String(c.value(r) ?? '').toLowerCase().includes(st.search)));
    Object.entries(st.colf).forEach(([i, val]) => {
      if (!val) return;
      const c = cols[i];
      rows = rows.filter(r => {
        const cell = String(c.value(r) ?? '');
        if (c.type === 'enum' || c.type === 'bool') return cell === val;
        return cell.toLowerCase().includes(val.toLowerCase());
      });
    });
    return rows;
  }
  function render() {
    const rows = filtered();
    const total = rows.length;
    if (st.offset >= total && total > 0) st.offset = Math.max(0, Math.floor((total - 1) / st.pageSize) * st.pageSize);
    const page = rows.slice(st.offset, st.offset + st.pageSize);
    q('.dt-tbody').innerHTML = page.map(r => {
      const tds = cols.map(c => `<td>${c.html ? c.html(r) : esc(String(c.value(r) ?? '—'))}</td>`).join('');
      return `<tr>${tds}${opts.actions ? `<td class="acciones">${opts.actions(r)}</td>` : ''}</tr>`;
    }).join('');
    const emptyEl = q('.dt-empty');
    const filtrando = st.search || Object.values(st.colf).some(v => v);
    if (total === 0) { emptyEl.classList.remove('hidden'); emptyEl.textContent = filtrando ? 'No hay registros que coincidan con el filtro.' : 'No hay ningún registro.'; }
    else emptyEl.classList.add('hidden');
    const desde = total === 0 ? 0 : st.offset + 1, hasta = Math.min(st.offset + st.pageSize, total);
    q('.dt-desde').value = desde; q('.dt-hasta').value = hasta; q('.dt-total').textContent = total;
    q('.dt-desde').classList.remove('invalid'); q('.dt-hasta').classList.remove('invalid');
    q('.dt-prev').disabled = st.offset === 0; q('.dt-next').disabled = hasta >= total;
  }
  return { setData(arr) { st.data = arr || []; st.offset = 0; render(); }, render };
}
function accionesHTML(id) {
  return `<button class="btn btn-sm" data-edit="${id}" title="Editar">${ICON.edit}</button>` +
         `<button class="btn btn-sm btn-danger" data-del="${id}" title="Eliminar">${ICON.trash}</button>`;
}

/* ============================================================================
   FILTROS de atletas
   ============================================================================ */
let debounceTxt, debounceFiltro;
function bindFiltros() {
  $('#f-texto').addEventListener('input', () => { clearTimeout(debounceTxt); debounceTxt = setTimeout(() => { state.filtro.q = $('#f-texto').value.trim(); applyFiltro(); }, 300); });
  const facetMap = { 'f-escuela': 'escuela', 'f-municipio': 'municipio', 'f-cinturon': 'cinturon', 'f-estado': 'estado_actividad' };
  Object.entries(facetMap).forEach(([id, field]) => $('#' + id).addEventListener('change', (e) => setFacet(field, e.target.value)));
  $('#f-fdesde').addEventListener('change', onFechaDesde);
  $('#f-fhasta').addEventListener('change', onFechaHasta);
  $('#btn-avanzado').addEventListener('click', () => { $('#adv-panel').classList.toggle('hidden'); $('#btn-avanzado').classList.toggle('active', !$('#adv-panel').classList.contains('hidden')); });
  $('#adv-match').addEventListener('change', (e) => { state.filtro.match = e.target.value; applyFiltro(); });
  $('#adv-add').addEventListener('click', addCondition);
  $('#adv-clear').addEventListener('click', clearFilters);
  $('#btn-limpiar-filtros').addEventListener('click', clearFilters);
  $('#btn-pdf-filtros').addEventListener('click', () => descargar('/api/reportes/atletas.pdf?' + filtroParams().toString()));
  $('#btn-pdf-todos').addEventListener('click', () => descargar('/api/reportes/atletas.pdf'));
}
function fieldSpecByKey(key) { return state.cat.campos_filtro.find(f => f.key === key); }
function needsValue(op) { return op !== 'set' && op !== 'unset'; }
function filtroActivo() { return state.filtro.q !== '' || state.filtro.cond.length > 0; }
function setFacet(field, value) { state.filtro.cond = state.filtro.cond.filter(c => c.field !== field); if (value !== '') state.filtro.cond.push({ field, op: 'eq', value }); renderAdvRows(); applyFiltro(); }
function setDateCond(field, op, value) { state.filtro.cond = state.filtro.cond.filter(c => !(c.field === field && c.op === op)); if (value) state.filtro.cond.push({ field, op, value }); renderAdvRows(); applyFiltro(); }
// Rango de fechas "Registrado desde/hasta" con restricción mutua: hasta no puede
// ser anterior a desde, ni desde posterior a hasta (mismo día permitido). Se
// ajustan los atributos min/max y, si un valor ya establecido queda fuera de
// rango, se corrige para evitar un rango inválido.
function onFechaDesde() {
  const d = $('#f-fdesde'), h = $('#f-fhasta');
  if (d.value) h.min = d.value; else h.removeAttribute('min');
  if (d.value && h.value && h.value < d.value) { h.value = d.value; setDateCond('fecha_registro', 'lte', h.value); }
  setDateCond('fecha_registro', 'gte', d.value);
}
function onFechaHasta() {
  const d = $('#f-fdesde'), h = $('#f-fhasta');
  if (h.value) d.max = h.value; else d.removeAttribute('max');
  if (h.value && d.value && d.value > h.value) { d.value = h.value; setDateCond('fecha_registro', 'gte', d.value); }
  setDateCond('fecha_registro', 'lte', h.value);
}
function syncFacetsFromCond() {
  const map = { escuela: '#f-escuela', municipio: '#f-municipio', cinturon: '#f-cinturon', estado_actividad: '#f-estado' };
  Object.entries(map).forEach(([field, sel]) => { const c = state.filtro.cond.find(x => x.field === field && x.op === 'eq'); $(sel).value = c ? c.value : ''; });
}
function addCondition() { const campos = state.cat.campos_filtro; if (!campos.length) return; state.filtro.cond.push({ field: campos[0].key, op: campos[0].ops[0].op, value: '' }); renderAdvRows(); }
function clearFilters() { state.filtro.q = ''; $('#f-texto').value = ''; state.filtro.cond = []; $('#f-fdesde').value = ''; $('#f-fhasta').value = ''; $('#f-fdesde').removeAttribute('max'); $('#f-fhasta').removeAttribute('min'); renderAdvRows(); syncFacetsFromCond(); applyFiltro(); }
function actualizarBotonLimpiar() { $('#btn-limpiar-filtros').classList.toggle('hidden', !filtroActivo()); }
function optionList(opts, selected) { return opts.map(o => `<option value="${esc(o.id)}" ${String(o.id) === String(selected) ? 'selected' : ''}>${esc(o.label)}</option>`).join(''); }
function advValHTML(c) {
  const spec = fieldSpecByKey(c.field);
  if (!spec || !needsValue(c.op)) return '<span class="adv-noval">—</span>';
  if (spec.type === 'select') { const opts = CAT_OPTS[spec.options] ? CAT_OPTS[spec.options]() : []; return `<select class="adv-val-input"><option value="">—</option>${optionList(opts, c.value)}</select>`; }
  if (spec.type === 'estado') return `<select class="adv-val-input"><option value="activo" ${c.value === 'activo' ? 'selected' : ''}>activo</option><option value="retirado" ${c.value === 'retirado' ? 'selected' : ''}>retirado</option></select>`;
  if (spec.type === 'date') return `<input class="adv-val-input" type="date" value="${esc(c.value)}">`;
  if (spec.type === 'number') return `<input class="adv-val-input" inputmode="numeric" value="${esc(c.value)}" placeholder="Valor">`;
  return `<input class="adv-val-input" value="${esc(c.value)}" placeholder="Valor">`;
}
function advRowHTML(c, i) {
  const spec = fieldSpecByKey(c.field) || {};
  const fieldOpts = state.cat.campos_filtro.map(f => `<option value="${f.key}" ${f.key === c.field ? 'selected' : ''}>${esc(f.label)}</option>`).join('');
  const ops = (spec.ops || []).map(o => `<option value="${o.op}" ${o.op === c.op ? 'selected' : ''}>${esc(o.label)}</option>`).join('');
  return `<div class="adv-row" data-i="${i}"><select class="adv-field">${fieldOpts}</select><select class="adv-op">${ops}</select><span class="adv-val">${advValHTML(c)}</span><button type="button" class="adv-del" title="Quitar condición">${ICON.x}</button></div>`;
}
function renderAdvRows() {
  const cont = $('#adv-rows');
  if (!state.filtro.cond.length) { cont.innerHTML = '<p class="adv-empty">Sin condiciones. Agregue una o use los filtros rápidos de arriba.</p>'; return; }
  cont.innerHTML = state.filtro.cond.map((c, i) => advRowHTML(c, i)).join('');
  $$('#adv-rows .adv-row').forEach(row => {
    const i = +row.dataset.i;
    row.querySelector('.adv-field').addEventListener('change', (e) => { const spec = fieldSpecByKey(e.target.value); state.filtro.cond[i] = { field: e.target.value, op: spec.ops[0].op, value: '' }; renderAdvRows(); syncFacetsFromCond(); applyFiltro(); });
    row.querySelector('.adv-op').addEventListener('change', (e) => { state.filtro.cond[i].op = e.target.value; if (!needsValue(e.target.value)) state.filtro.cond[i].value = ''; renderAdvRows(); applyFiltro(); });
    const val = row.querySelector('.adv-val-input');
    if (val) { const ev = val.tagName === 'SELECT' ? 'change' : 'input'; val.addEventListener(ev, (e) => { state.filtro.cond[i].value = e.target.value; syncFacetsFromCond(); applyFiltroDebounced(); }); }
    row.querySelector('.adv-del').addEventListener('click', () => { state.filtro.cond.splice(i, 1); renderAdvRows(); syncFacetsFromCond(); applyFiltro(); });
  });
}
function applyFiltro() { state.offset = 0; cargarLista(); }
function applyFiltroDebounced() { clearTimeout(debounceFiltro); debounceFiltro = setTimeout(applyFiltro, 350); }
function domainParam() {
  const conds = state.filtro.cond.filter(c => c.field && c.op && (!needsValue(c.op) || String(c.value) !== ''));
  if (!conds.length) return null;
  return JSON.stringify({ match: state.filtro.match, conditions: conds.map(c => ({ field: c.field, op: c.op, value: String(c.value) })) });
}
function filtroParams() { const p = new URLSearchParams(); if (state.filtro.q) p.set('q', state.filtro.q); const d = domainParam(); if (d) p.set('domain', d); return p; }

/* ---------------------- Listado de atletas ---------------------- */
async function cargarLista() {
  actualizarBotonLimpiar();
  const p = filtroParams();
  p.set('limit', state.pageSize); p.set('offset', state.offset);
  let data;
  try { data = await api('/api/atletas?' + p.toString()); } catch (e) { toast(e.message, 'err'); return; }
  state.total = data.total;
  if (state.offset >= state.total && state.total > 0) { state.offset = Math.max(0, Math.floor((state.total - 1) / state.pageSize) * state.pageSize); return cargarLista(); }
  renderTabla(data.items); renderPager();
}
function renderTabla(items) {
  const empty = $('#empty');
  if (items.length === 0) { empty.classList.remove('hidden'); empty.textContent = filtroActivo() ? 'No hay registros que coincidan con el filtro.' : 'No hay ningún registro.'; }
  else empty.classList.add('hidden');
  $('#tabla-body').innerHTML = items.map(a => `
    <tr data-id="${a.id}">
      <td class="nombre-cell"><b>${esc(a.apellidos)}, ${esc(a.nombres)}</b>${a.es_menor ? '<small>menor de edad</small>' : ''}</td>
      <td>${esc(cedulaFmt(a.cedula_tipo, a.cedula_numero))}</td>
      <td>${a.edad}</td>
      <td>${esc(a.escuela_nombre || '—')}</td>
      <td>${chipCinturon(a.cinturon_color, a.cinturon_dan)}</td>
      <td class="state-${a.estado}">${STATE_DOT} ${a.estado}</td>
      <td class="chev">${ICON.chevron}</td>
    </tr>`).join('');
  $$('#tabla-body tr').forEach(tr => tr.addEventListener('click', () => verDetalle(tr.dataset.id)));
}
function bindPager() {
  $('#pg-prev').addEventListener('click', () => { if (state.offset > 0) { state.offset = Math.max(0, state.offset - state.pageSize); cargarLista(); } });
  $('#pg-next').addEventListener('click', () => { if (state.offset + state.pageSize < state.total) { state.offset += state.pageSize; cargarLista(); } });
  const commit = () => aplicarRango();
  ['#pg-desde', '#pg-hasta'].forEach(sel => { $(sel).addEventListener('change', commit); $(sel).addEventListener('keydown', (e) => { if (e.key === 'Enter') { e.preventDefault(); commit(); } }); $(sel).addEventListener('input', () => $(sel).classList.remove('invalid')); });
}
function aplicarRango() {
  const desde = parseInt($('#pg-desde').value, 10), hasta = parseInt($('#pg-hasta').value, 10);
  const invalido = !Number.isInteger(desde) || !Number.isInteger(hasta) || desde < 1 || hasta < desde || (state.total > 0 && desde > state.total);
  if (invalido) { $('#pg-desde').classList.toggle('invalid', !(desde >= 1 && (state.total === 0 || desde <= state.total))); $('#pg-hasta').classList.toggle('invalid', !(hasta >= desde)); return; }
  $('#pg-desde').classList.remove('invalid'); $('#pg-hasta').classList.remove('invalid');
  state.offset = desde - 1; state.pageSize = Math.max(1, hasta - desde + 1); cargarLista();
}
function renderPager() {
  const desde = state.total === 0 ? 0 : state.offset + 1, hasta = Math.min(state.offset + state.pageSize, state.total);
  $('#list-count').textContent = `${state.total} atleta(s)`;
  $('#pg-desde').value = desde; $('#pg-hasta').value = hasta; $('#pg-total').textContent = state.total;
  $('#pg-desde').classList.remove('invalid'); $('#pg-hasta').classList.remove('invalid');
  $('#pg-prev').disabled = state.offset === 0; $('#pg-next').disabled = hasta >= state.total;
}
function chipCinturon(color, dan) {
  if (!color) return '<span class="chip">—</span>';
  const dot = BELT_COLORS[color] || '#888';
  return `<span class="chip chip-belt"><span class="dot" style="background:${dot}"></span>${esc(color)}${dan ? ` ${dan}° DAN` : ''}</span>`;
}

/* ============================================================================
   MODAL: FORM ATLETA
   ============================================================================ */
const modalAtleta = $('#modal-atleta');
const GA = () => ({ estado: $('#a-estado'), ciudad: $('#a-ciudad'), municipio: $('#a-municipio'), parroquia: $('#a-parroquia') });
let retornoFicha = null; // id del atleta cuya ficha reabrir al cerrar el formulario
$('#btn-nuevo').addEventListener('click', () => { retornoFicha = null; abrirForm(null); });

// Al cerrar el formulario (Cancelar/X/fondo) sin guardar, reabrir la ficha si venía de ella.
function alCerrarFormAtleta() { const rid = retornoFicha; retornoFicha = null; if (rid) setTimeout(() => verDetalle(rid), 0); }
modalAtleta.querySelectorAll('[data-close]').forEach(b => b.addEventListener('click', alCerrarFormAtleta));
modalAtleta.addEventListener('mousedown', e => { if (e.target === modalAtleta) alCerrarFormAtleta(); });

function abrirForm(atleta) {
  if (!isAdmin()) return;
  state.editId = atleta ? atleta.id : null;
  limpiarErrores();
  $('#form-error').textContent = '';
  $('#modal-title').textContent = atleta ? 'Editar atleta' : 'Nuevo atleta';
  const f = $('#form-atleta');
  f.reset();
  $('#a-cedula-tipo').value = 'V'; $('#r-cedula-tipo').value = 'V';
  $('#fs-cinturon').classList.toggle('hidden', !!atleta);
  $('#a-contactos').innerHTML = '';
  if (atleta) {
    $('#a-id').value = atleta.id;
    $('#a-nombres').value = atleta.nombres;
    $('#a-apellidos').value = atleta.apellidos;
    $('#a-cedula-tipo').value = atleta.cedula_tipo || 'V';
    $('#a-cedula-numero').value = atleta.cedula_numero || '';
    escribirFecha('a-nac', atleta.fecha_nacimiento, false);
    $('#a-telefono').value = atleta.telefono || '';
    (atleta.telefonos_contacto || []).forEach(t => agregarContacto(t));
    geoSet(GA(), atleta);
    $('#a-escuela').value = atleta.escuela_id || '';
    $('#a-maestro').value = atleta.maestro_id || '';
    $('#a-tipo-sangre').value = atleta.tipo_sangre || '';
    $('#a-direccion').value = atleta.direccion_detalle || '';
    escribirFecha('a-ins', atleta.fecha_inscripcion, !atleta.inscripcion_dia_exacto);
    $('#a-ins-nodia').checked = !atleta.inscripcion_dia_exacto;
    const r = atleta.representante || {};
    $('#r-nombres').value = r.nombres || '';
    $('#r-apellidos').value = r.apellidos || '';
    $('#r-cedula-tipo').value = r.cedula_tipo || 'V';
    $('#r-cedula-numero').value = r.cedula_numero || '';
    $('#r-telefono').value = r.telefono || '';
    setParentesco(r.parentesco || '');
  } else {
    geoSet(GA(), {});
    escribirFecha('a-ins', hoy(), false);
    $('#a-ins-nodia').checked = false;
    setParentesco('');
    // Cinturón inicial: Blanco y fecha de asignación de hoy por defecto.
    $('#a-cinturon').value = cinturonBlancoId();
    $('#a-cinturon-fecha').value = hoy();
  }
  toggleDiaInscripcion(); actualizarContactoAdd(); actualizarEdadHint(); actualizarDanWrap(); validarForm();
  abrir(modalAtleta);
}

function llenarMeses(sel) { sel.innerHTML = '<option value="">Mes</option>' + MESES.map((m, i) => `<option value="${i + 1}">${m}</option>`).join(''); }
function tripletVals(pfx) { return { dia: $(`#${pfx}-dia`).value.trim(), mes: $(`#${pfx}-mes`).value, anio: $(`#${pfx}-anio`).value.trim() }; }
function fechaISO(dia, mes, anio) {
  const y = parseInt(anio, 10), m = parseInt(mes, 10), d = parseInt(dia, 10);
  if (!y || !m || !d) return null;
  if (y < 1900 || y > new Date().getFullYear()) return null;
  if (m < 1 || m > 12) return null;
  if (d < 1 || d > new Date(y, m, 0).getDate()) return null;
  const iso = `${y}-${pad2(m)}-${pad2(d)}`;
  if (new Date(iso) > new Date(hoy())) return null;
  return iso;
}
function escribirFecha(pfx, iso, sinDia) {
  if (!iso) { $(`#${pfx}-dia`).value = ''; $(`#${pfx}-mes`).value = ''; $(`#${pfx}-anio`).value = ''; return; }
  const [y, m, d] = iso.split('-');
  $(`#${pfx}-dia`).value = sinDia ? '' : String(parseInt(d, 10));
  $(`#${pfx}-mes`).value = String(parseInt(m, 10));
  $(`#${pfx}-anio`).value = y;
}
function readNac() { const v = tripletVals('a-nac'); return fechaISO(v.dia, v.mes, v.anio); }
function readIns() { const v = tripletVals('a-ins'); const dia = $('#a-ins-nodia').checked ? '1' : v.dia; return fechaISO(dia, v.mes, v.anio); }
// Solo oculta el input Día (readIns/motivoFecha ya lo ignoran cuando está
// marcado). No se borra el valor para que al desmarcar reaparezca el mismo día.
function toggleDiaInscripcion() { $('#a-ins-dia').classList.toggle('hidden', $('#a-ins-nodia').checked); }

function agregarContacto(value = '') {
  if ($$('#a-contactos .contacto-row').length >= 3) return;
  const row = document.createElement('div');
  row.className = 'contacto-row';
  row.innerHTML = `<input class="contacto-inp" inputmode="tel" maxlength="20" placeholder="Teléfono de contacto" /><button type="button" class="contacto-del" title="Quitar">${ICON.x}</button>`;
  const inp = row.querySelector('.contacto-inp');
  inp.value = value;
  limitInput(inp, { allow: /[0-9+\-\s()]/ });
  inp.addEventListener('input', validarForm);
  row.querySelector('.contacto-del').addEventListener('click', () => { row.remove(); actualizarContactoAdd(); validarForm(); });
  $('#a-contactos').appendChild(row);
  actualizarContactoAdd();
}
function actualizarContactoAdd() { $('#a-contacto-add').disabled = $$('#a-contactos .contacto-row').length >= 3; }
function contactosVals() { return $$('#a-contactos .contacto-inp').map(i => i.value.trim()).filter(Boolean); }
$('#a-contacto-add').addEventListener('click', () => { agregarContacto(); validarForm(); });

function setParentesco(valor) {
  if (valor && PARENTESCOS_COMUNES.includes(valor)) { $('#r-parentesco-sel').value = valor; $('#r-parentesco-otro').value = ''; }
  else if (valor) { $('#r-parentesco-sel').value = 'Otro'; $('#r-parentesco-otro').value = valor; }
  else { $('#r-parentesco-sel').value = ''; $('#r-parentesco-otro').value = ''; }
  toggleParentescoOtro();
}
function toggleParentescoOtro() { $('#r-parentesco-otro-wrap').classList.toggle('hidden', $('#r-parentesco-sel').value !== 'Otro'); }
function parentescoVal() { const s = $('#r-parentesco-sel').value; return s === 'Otro' ? valOrNull('#r-parentesco-otro') : (s || null); }
$('#r-parentesco-sel').addEventListener('change', () => { toggleParentescoOtro(); validarForm(); });

$('#a-ins-nodia').addEventListener('change', () => { toggleDiaInscripcion(); validarForm(); });
$('#a-cinturon').addEventListener('change', () => { actualizarDanWrap(); validarForm(); });
geoWire(GA());

function edadDe(iso) {
  if (!iso) return null;
  const n = new Date(iso), h = new Date();
  let e = h.getFullYear() - n.getFullYear();
  const m = h.getMonth() - n.getMonth();
  if (m < 0 || (m === 0 && h.getDate() < n.getDate())) e--;
  return e;
}
function esMenorForm() { const e = edadDe(readNac()); return e !== null && e < 18; }
function actualizarEdadHint() {
  const e = edadDe(readNac());
  const hint = $('#edad-hint');
  if (e === null) { hint.textContent = 'Edad: Ingrese la fecha de nacimiento'; hint.style.color = 'var(--muted)'; $('.req-menor').style.display = 'none'; return; }
  const menor = e < 18;
  hint.textContent = `Edad: ${e} años` + (menor ? ' — menor de edad: representante obligatorio.' : '');
  hint.style.color = menor ? 'var(--orange)' : 'var(--cyan)';
  $('.req-menor').style.display = menor ? 'inline' : 'none';
}
function actualizarDanWrap() {
  const c = cinturonPorId($('#a-cinturon').value); const negro = c && c.es_negro;
  $('#dan-wrap').classList.toggle('hidden', !negro);
  if (!negro) $('#a-dan').value = '';
  else if (!$('#a-dan').value) $('#a-dan').value = '1';   // DAN 1 por defecto
}

function markErr(sel) { const el = $(sel); if (el) el.classList.add('field-err'); }
function limpiarErrores() { $$('#form-atleta .field-err').forEach(el => el.classList.remove('field-err')); }
function telValido(s) { const d = (s.match(/\d/g) || []).length; return d >= 7 && d <= 15; }
// motivoFecha devuelve null si la fecha (día/mes/año) es válida, o un mensaje concreto.
function motivoFecha(pfx, label, diaOpcionalSinDia) {
  const v = tripletVals(pfx);
  const dia = diaOpcionalSinDia && $('#a-ins-nodia').checked ? '1' : v.dia;
  if (!v.anio || !v.mes || !dia) return `Complete la ${label}`;
  const y = +v.anio, m = +v.mes, d = +dia;
  if (y < 1900 || y > new Date().getFullYear()) return `El año de la ${label} es inválido`;
  if (m < 1 || m > 12) return `El mes de la ${label} es inválido`;
  if (d < 1 || d > new Date(y, m, 0).getDate()) return `El día de la ${label} es inválido`;
  if (new Date(`${y}-${pad2(m)}-${pad2(d)}`) > new Date(hoy())) return `La ${label} no puede ser futura`;
  return null;
}
function renderAyuda(motivos) {
  const box = $('#form-ayuda');
  if (!motivos.length) { box.classList.add('hidden'); box.innerHTML = ''; return; }
  box.classList.remove('hidden');
  box.innerHTML = `<div class="ayuda-title">Para guardar, falta corregir:</div><ul>${motivos.map(m => `<li>${esc(m)}</li>`).join('')}</ul>`;
}
function validarForm() {
  limpiarErrores();
  const motivos = [];
  if (!val('#a-nombres')) { motivos.push('El nombre es requerido'); markErr('#a-nombres'); }
  if (!val('#a-apellidos')) { motivos.push('El apellido es requerido'); markErr('#a-apellidos'); }
  const tel = val('#a-telefono');
  if (!tel) { motivos.push('El teléfono principal es requerido'); markErr('#a-telefono'); }
  else if (!telValido(tel)) { motivos.push('El teléfono principal es inválido (ej. 0412123456)'); markErr('#a-telefono'); }
  const mNac = motivoFecha('a-nac', 'fecha de nacimiento', false);
  if (mNac) { motivos.push(mNac); markErr('#dt-nac'); }
  const mIns = motivoFecha('a-ins', 'fecha de inscripción', true);
  if (mIns) { motivos.push(mIns); markErr('#dt-ins'); }
  const cn = $('#a-cedula-numero').value.trim();
  if (cn && !/^[0-9]+$/.test(cn)) { motivos.push('La cédula solo admite dígitos'); markErr('#a-cedula-numero'); }
  if (!$('#dan-wrap').classList.contains('hidden')) { const d = parseInt(val('#a-dan'), 10); if (!(d >= 1 && d <= 9)) { motivos.push('El DAN debe ser un número del 1 al 9'); markErr('#a-dan'); } }
  if (esMenorForm()) {
    if (!val('#r-nombres')) { motivos.push('Falta el nombre del representante (menor de edad)'); markErr('#r-nombres'); }
    if (!val('#r-apellidos')) { motivos.push('Falta el apellido del representante'); markErr('#r-apellidos'); }
    const rt = val('#r-telefono');
    if (!rt) { motivos.push('Falta el teléfono del representante'); markErr('#r-telefono'); }
    else if (!telValido(rt)) { motivos.push('El teléfono del representante es inválido (ej. 0412123456)'); markErr('#r-telefono'); }
  }
  const rcn = $('#r-cedula-numero').value.trim();
  if (rcn && !/^[0-9]+$/.test(rcn)) { motivos.push('La cédula del representante solo admite dígitos'); markErr('#r-cedula-numero'); }
  renderAyuda(motivos);
  $('#a-submit').disabled = motivos.length > 0;
  actualizarEdadHint();
  return motivos.length === 0;
}
const FIELD_SEL = {
  nombres: '#a-nombres', apellidos: '#a-apellidos', cedula_numero: '#a-cedula-numero', cedula_tipo: '#a-cedula-tipo',
  fecha_nacimiento: '#dt-nac', fecha_inscripcion: '#dt-ins', telefono: '#a-telefono', dan: '#a-dan',
  rep_nombres: '#r-nombres', rep_apellidos: '#r-apellidos', rep_telefono: '#r-telefono', rep_cedula_numero: '#r-cedula-numero',
};
function pintarErrores(fields) { Object.keys(fields).forEach(k => { const sel = FIELD_SEL[k]; if (sel) $$(sel).forEach(el => el.classList.add('field-err')); }); }
$('#form-atleta').addEventListener('input', validarForm);
$('#form-atleta').addEventListener('change', validarForm);

function representantePayload() {
  const r = { cedula_tipo: valOrNull('#r-cedula-tipo'), cedula_numero: valOrNull('#r-cedula-numero'), nombres: valOrNull('#r-nombres'), apellidos: valOrNull('#r-apellidos'), telefono: valOrNull('#r-telefono'), parentesco: parentescoVal() };
  if (!r.cedula_numero) r.cedula_tipo = null;
  return (r.nombres || r.apellidos || r.telefono || r.cedula_numero || r.parentesco) ? r : null;
}
$('#form-atleta').addEventListener('submit', async (e) => {
  e.preventDefault();
  $('#form-error').textContent = '';
  if (!validarForm()) { $('#form-error').textContent = 'Complete los campos requeridos correctamente.'; return; }
  const loc = geoRead(GA());
  const cedNum = valOrNull('#a-cedula-numero');
  const payload = {
    nombres: val('#a-nombres'), apellidos: val('#a-apellidos'),
    cedula_tipo: cedNum ? val('#a-cedula-tipo') : null, cedula_numero: cedNum,
    fecha_nacimiento: readNac(), telefono: valOrNull('#a-telefono'), telefonos_contacto: contactosVals(),
    estado_id: loc.estado_id, ciudad_id: loc.ciudad_id, municipio_id: loc.municipio_id, parroquia_id: loc.parroquia_id,
    direccion_detalle: valOrNull('#a-direccion'), escuela_id: intOrNull('#a-escuela'),
    maestro_id: intOrNull('#a-maestro'), tipo_sangre: valOrNull('#a-tipo-sangre'),
    fecha_inscripcion: readIns(), inscripcion_dia_exacto: !$('#a-ins-nodia').checked,
    representante: representantePayload(),
  };
  if (state.editId === null && !$('#fs-cinturon').classList.contains('hidden')) {
    const cid = intOrNull('#a-cinturon');
    if (cid) {
      payload.cinturon_id = cid;
      const d = $('#a-dan').value; payload.dan = d ? parseInt(d, 10) : null;
      payload.cinturon_fecha = $('#a-cinturon-fecha').value || null;
    }
  }
  try {
    if (state.editId === null) { await api('/api/atletas', { method: 'POST', body: JSON.stringify(payload) }); toast('Atleta registrado'); }
    else { await api('/api/atletas/' + state.editId, { method: 'PUT', body: JSON.stringify(payload) }); toast('Cambios guardados'); }
    cerrar(modalAtleta);
    const rid = retornoFicha; retornoFicha = null;
    await cargarLista();
    if (rid) verDetalle(rid);
  } catch (err) {
    if (err.fields) { pintarErrores(err.fields); $('#form-error').textContent = 'Corrija los campos marcados en rojo.'; }
    else $('#form-error').textContent = err.message;
  }
});

/* ---------------------- Detalle atleta ---------------------- */
const modalDetalle = $('#modal-detalle');
async function verDetalle(id) {
  let a;
  try { a = await api('/api/atletas/' + id); } catch (e) { return toast(e.message, 'err'); }
  const insc = a.inscripcion_dia_exacto ? a.fecha_inscripcion : soloMesAnio(a.fecha_inscripcion);
  const rep = a.representante;
  const contactos = (a.telefonos_contacto || []).length ? a.telefonos_contacto.join(', ') : '—';
  const ubic = [a.parroquia_nombre, a.municipio_nombre, a.ciudad_nombre, a.estado_nombre].filter(Boolean).join(', ') || '—';
  $('#det-title').textContent = `${a.nombres} ${a.apellidos}`;
  $('#det-body').innerHTML = `
    <div class="det-head">
      <div class="det-avatar"><img src="/logo.png" width="42" height="42" alt=""></div>
      <div class="det-head-info"><h3 style="margin:0">${esc(a.apellidos)}, ${esc(a.nombres)}</h3>
        <div>${chipCinturon(a.cinturon_color, a.cinturon_dan)} <span class="chip state-${a.estado}">${STATE_DOT} ${a.estado}</span></div></div>
      <button class="btn btn-sm det-ficha-btn" id="d-ficha">${ICON.download}<span>Ficha del atleta (PDF)</span></button>
    </div>
    <div class="det-grid">
      ${kv('Cédula', cedulaFmt(a.cedula_tipo, a.cedula_numero))}
      ${kv('Edad', a.edad + ' años' + (a.es_menor ? ' (menor)' : ''))}
      ${kv('Nacimiento', a.fecha_nacimiento)}
      ${kv('Tipo de sangre', a.tipo_sangre || '—')}
      ${kv('Inscripción', insc)}
      ${kv('Escuela', a.escuela_nombre || '—')}
      ${kv('Entrenador', a.maestro_nombre || '—')}
      ${kv('Ubicación', ubic)}
      ${kv('Teléfono principal', a.telefono || '—')}
      ${kv('Teléfonos de contacto', contactos)}
      ${kv('Dirección', a.direccion_detalle || '—')}
    </div>
    ${rep ? `<fieldset><legend>Representante</legend><div class="det-grid">
      ${kv('Nombre', `${rep.nombres || ''} ${rep.apellidos || ''}`.trim() || '—')}
      ${kv('Cédula', cedulaFmt(rep.cedula_tipo, rep.cedula_numero))}
      ${kv('Teléfono', rep.telefono || '—')}
      ${kv('Parentesco', rep.parentesco || '—')}
    </div></fieldset>` : ''}
    <fieldset><legend>Historial de cinturón</legend>
      <ul class="timeline">${(a.cinturones || []).map(h => `<li><b>${chipCinturon(h.color, h.dan)}</b> <small>— ${h.fecha_cambio}</small></li>`).join('') || '<li><small>Sin registros</small></li>'}</ul>
    </fieldset>
    <fieldset><legend>Periodos de actividad</legend>
      <ul class="timeline">${(a.periodos || []).map(p => `<li><b>${p.fecha_inicio}</b> <span class="arrow-sep">${ICON.arrow}</span> ${p.fecha_fin ? esc(p.fecha_fin) : '<span class="state-activo">activo</span>'} ${p.motivo_retiro ? `<small>(${esc(p.motivo_retiro)})</small>` : ''}</li>`).join('')}</ul>
    </fieldset>
    ${isAdmin() ? `<div class="det-actions">
      <button class="btn btn-sm" id="d-editar">${ICON.edit}<span>Editar</span></button>
      <button class="btn btn-sm" id="d-cinturon">${ICON.belt}<span>Cambiar cinturón</span></button>
      ${a.estado === 'activo' ? `<button class="btn btn-sm" id="d-retirar">${ICON.retire}<span>Retirar</span></button>` : `<button class="btn btn-sm" id="d-reactivar">${ICON.reactivate}<span>Reactivar</span></button>`}
      <button class="btn btn-sm btn-danger" id="d-eliminar">${ICON.trash}<span>Eliminar</span></button>
    </div>` : ''}`;
  $('#d-ficha').onclick = () => descargar('/api/atletas/' + a.id + '/ficha.pdf');
  if (isAdmin()) {
    $('#d-editar').onclick = () => { retornoFicha = a.id; cerrar(modalDetalle); abrirForm(a); };
    $('#d-cinturon').onclick = () => cambiarCinturon(a);
    if ($('#d-retirar')) $('#d-retirar').onclick = () => retirar(a);
    if ($('#d-reactivar')) $('#d-reactivar').onclick = () => reactivar(a);
    $('#d-eliminar').onclick = () => eliminar(a);
  }
  abrir(modalDetalle);
}
async function cambiarCinturon(a) {
  const r = await pedirDatos('Cambiar cinturón', [
    { key: 'cinturon_id', label: 'Cinturón', type: 'select', options: state.cat.cinturones.map(c => ({ value: c.id, label: c.color })) },
    { key: 'dan', label: 'DAN (1–9)', type: 'number', maxLen: 1, showIf: (v) => { const c = cinturonPorId(v.cinturon_id); return !!(c && c.es_negro); } },
    { key: 'fecha', label: 'Fecha del cambio', type: 'date', value: hoy() },
  ]);
  if (!r) return;
  try {
    await api(`/api/atletas/${a.id}/cinturon`, { method: 'POST', body: JSON.stringify({ cinturon_id: parseInt(r.cinturon_id, 10), dan: r.dan ? parseInt(r.dan, 10) : null, fecha_cambio: r.fecha || hoy() }) });
    toast('Cinturón actualizado'); await cargarLista(); verDetalle(a.id);
  } catch (e) { toast(e.message, 'err'); }
}
async function retirar(a) {
  const r = await pedirDatos('Retirar atleta', [{ key: 'fecha', label: 'Fecha de retiro', type: 'date', value: hoy() }, { key: 'motivo', label: 'Motivo (opcional)', type: 'text', maxLen: 80 }]);
  if (!r) return;
  try { await api(`/api/atletas/${a.id}/retirar`, { method: 'POST', body: JSON.stringify({ fecha: r.fecha || hoy(), motivo: r.motivo || null }) }); toast('Atleta retirado'); await cargarLista(); verDetalle(a.id); }
  catch (e) { toast(e.message, 'err'); }
}
async function reactivar(a) {
  const r = await pedirDatos('Reactivar atleta', [{ key: 'fecha', label: 'Fecha de reactivación', type: 'date', value: hoy() }]);
  if (!r) return;
  try { await api(`/api/atletas/${a.id}/reactivar`, { method: 'POST', body: JSON.stringify({ fecha: r.fecha || hoy() }) }); toast('Atleta reactivado'); await cargarLista(); verDetalle(a.id); }
  catch (e) { toast(e.message, 'err'); }
}
async function eliminar(a) {
  if (!await confirmar(`¿Eliminar definitivamente a ${a.nombres} ${a.apellidos}? Esta acción no se puede deshacer.`, { peligro: true, ok: 'Eliminar' })) return;
  try { await api('/api/atletas/' + a.id, { method: 'DELETE' }); toast('Atleta eliminado'); cerrar(modalDetalle); await cargarLista(); }
  catch (e) { toast(e.message, 'err'); }
}

/* ============================================================================
   ESCUELAS
   ============================================================================ */
const modalEscuela = $('#modal-escuela');
const GE = () => ({ estado: $('#esc-estado'), ciudad: $('#esc-ciudad'), municipio: $('#esc-municipio') });
geoWire(GE());
let dtEscuelas;
async function cargarEscuelas() {
  let data;
  try { data = await api('/api/escuelas'); } catch (e) { return toast(e.message, 'err'); }
  if (!dtEscuelas) dtEscuelas = DataTable({
    mount: '#dt-escuelas',
    columns: [
      { label: 'Escuela', value: e => e.nombre, type: 'text' },
      { label: 'Municipio', value: e => e.municipio_nombre || '—', type: 'text' },
      { label: 'Dirección', value: e => e.direccion || '—', type: 'text', filter: false },
      { label: 'Activa', value: e => e.activa ? 'Sí' : 'No', type: 'bool', html: e => e.activa ? '<span class="chip state-activo">Sí</span>' : '<span class="chip state-retirado">No</span>' },
    ],
    actions: e => accionesHTML(e.id), onEdit: e => abrirEscuela(e), onDelete: e => borrarEscuela(e),
  });
  dtEscuelas.setData(data);
}
$('#btn-nueva-escuela').addEventListener('click', () => abrirEscuela(null));
function abrirEscuela(e) {
  $('#form-escuela').reset();
  $('#esc-error').textContent = '';
  $('#esc-title').textContent = e ? 'Editar escuela' : 'Nueva escuela';
  $('#esc-id').value = e ? e.id : '';
  $('#esc-nombre').value = e ? e.nombre : '';
  $('#esc-direccion').value = e ? (e.direccion || '') : '';
  $('#esc-activa').checked = e ? !!e.activa : true;
  geoSet(GE(), e && e.municipio_id ? deriveGeoDeMunicipio(e.municipio_id) : {});
  abrir(modalEscuela);
}
function deriveGeoDeMunicipio(municipioId) { const m = porId(state.geo.municipios, municipioId); return m ? { estado_id: m.estado_id, ciudad_id: m.ciudad_id, municipio_id: m.id } : {}; }
async function borrarEscuela(e) {
  if (!await confirmar(`¿Eliminar la escuela "${e.nombre}"?`, { peligro: true, ok: 'Eliminar' })) return;
  try { await api('/api/escuelas/' + e.id, { method: 'DELETE' }); toast('Escuela eliminada'); await cargarCatalogos(); cargarEscuelas(); }
  catch (err) { toast(err.message, 'err'); }
}
$('#form-escuela').addEventListener('submit', async (e) => {
  e.preventDefault();
  $('#esc-error').textContent = '';
  const muni = intOrNull('#esc-municipio');
  if (!val('#esc-nombre')) { $('#esc-error').textContent = 'El nombre es obligatorio.'; return; }
  if (!muni) { $('#esc-error').textContent = 'Seleccione el municipio.'; return; }
  const body = { nombre: val('#esc-nombre'), municipio_id: muni, direccion: valOrNull('#esc-direccion'), activa: $('#esc-activa').checked };
  const id = $('#esc-id').value;
  try {
    if (id) await api('/api/escuelas/' + id, { method: 'PUT', body: JSON.stringify(body) });
    else await api('/api/escuelas', { method: 'POST', body: JSON.stringify(body) });
    toast('Escuela guardada'); cerrar(modalEscuela); await cargarCatalogos(); cargarEscuelas();
  } catch (err) { $('#esc-error').textContent = err.message; }
});

/* ============================================================================
   USUARIOS
   ============================================================================ */
const modalUsuario = $('#modal-usuario');
let dtUsuarios;
async function cargarUsuarios() {
  let data;
  try { data = await api('/api/usuarios'); } catch (e) { return toast(e.message, 'err'); }
  if (!dtUsuarios) dtUsuarios = DataTable({
    mount: '#dt-usuarios',
    columns: [
      { label: 'Usuario', value: u => u.username, type: 'text' },
      { label: 'Nombre', value: u => `${u.nombres} ${u.apellidos}`.trim(), type: 'text' },
      { label: 'Rol', value: u => u.es_admin ? 'Administrador' : 'Consultor', type: 'enum', options: ['Administrador', 'Consultor'], html: u => u.es_admin ? '<span class="chip chip-belt">Administrador</span>' : '<span class="chip">Consultor</span>' },
    ],
    actions: u => accionesHTML(u.id), onEdit: u => abrirUsuario(u), onDelete: u => borrarUsuario(u),
  });
  dtUsuarios.setData(data);
}
$('#btn-nuevo-usuario').addEventListener('click', () => abrirUsuario(null));
function abrirUsuario(u) {
  $('#form-usuario').reset();
  $('#usr-error').textContent = '';
  $$('#form-usuario .field-err').forEach(el => el.classList.remove('field-err'));
  $('#usr-title').textContent = u ? 'Editar usuario' : 'Nuevo usuario';
  $('#usr-id').value = u ? u.id : '';
  $('#usr-nombre').value = u ? `${u.nombres} ${u.apellidos}`.trim() : '';
  $('#usr-username').value = u ? u.username : '';
  $('#usr-rol').value = u ? (u.es_admin ? '1' : '0') : '0';
  $('#usr-pass-label').textContent = u ? 'Nueva contraseña (dejar en blanco para no cambiar)' : 'Contraseña *';
  validarUsuarioForm();
  abrir(modalUsuario);
}
async function borrarUsuario(u) {
  if (!await confirmar(`¿Eliminar al usuario "${u.username}"?`, { peligro: true, ok: 'Eliminar' })) return;
  try { await api('/api/usuarios/' + u.id, { method: 'DELETE' }); toast('Usuario eliminado'); cargarUsuarios(); }
  catch (e) { toast(e.message, 'err'); }
}
function validarUsuarioForm() {
  const editando = !!$('#usr-id').value;
  const p = $('#usr-pass').value;
  const cambiaPass = !editando || p !== '';
  const checks = { len: p.length >= 8, upper: /[A-Z]/.test(p), lower: /[a-z]/.test(p), digit: /[0-9]/.test(p) };
  $$('#usr-pw-checks li').forEach(li => { li.classList.toggle('ok', !!checks[li.dataset.check]); li.classList.toggle('idle', !cambiaPass); });
  const okBase = val('#usr-nombre') && val('#usr-username').length >= 3;
  const okPass = !cambiaPass || (checks.len && checks.upper && checks.lower && checks.digit);
  $('#usr-submit').disabled = !(okBase && okPass);
}
$('#form-usuario').addEventListener('input', validarUsuarioForm);
$('#form-usuario').addEventListener('submit', async (e) => {
  e.preventDefault();
  $('#usr-error').textContent = '';
  const id = $('#usr-id').value;
  const body = { username: val('#usr-username'), nombres: val('#usr-nombre'), apellidos: '', es_admin: $('#usr-rol').value === '1' };
  const p = $('#usr-pass').value;
  if (p !== '') body.password = p;
  try {
    if (id) await api('/api/usuarios/' + id, { method: 'PUT', body: JSON.stringify(body) });
    else await api('/api/usuarios', { method: 'POST', body: JSON.stringify(body) });
    toast('Usuario guardado'); cerrar(modalUsuario); cargarUsuarios();
  } catch (err) {
    if (err.fields) { const map = { username: '#usr-username', password: '#usr-pass', nombres: '#usr-nombre' }; Object.keys(err.fields).forEach(k => { if (map[k]) $(map[k]).classList.add('field-err'); }); $('#usr-error').textContent = Object.values(err.fields)[0]; }
    else $('#usr-error').textContent = err.message;
  }
});

/* ============================================================================
   ENTRENADORES (maestros)
   ============================================================================ */
const modalEntrenador = $('#modal-entrenador');
let dtEntrenadores;
async function cargarEntrenadores() {
  let data;
  try { data = await api('/api/entrenadores'); } catch (e) { return toast(e.message, 'err'); }
  if (!dtEntrenadores) dtEntrenadores = DataTable({
    mount: '#dt-entrenadores',
    columns: [
      { label: 'Nombre', value: m => `${m.nombres} ${m.apellidos}`.trim(), type: 'text' },
      { label: 'Cédula', value: m => cedulaFmt(m.cedula_tipo, m.cedula_numero), type: 'text' },
      { label: 'Teléfono', value: m => m.telefono || '—', type: 'text', filter: false },
      { label: 'Escuela', value: m => m.escuela_nombre || '—', type: 'text' },
      { label: 'Cinturón', value: m => m.cinturon_color ? (m.cinturon_color + (m.dan ? ` ${m.dan}° DAN` : '')) : '—', type: 'text', html: m => chipCinturon(m.cinturon_color, m.dan) },
      { label: 'Atletas', value: m => m.num_atletas, type: 'number', filter: false },
      { label: 'Activo', value: m => m.activo ? 'Sí' : 'No', type: 'bool', html: m => m.activo ? '<span class="chip state-activo">Sí</span>' : '<span class="chip state-retirado">No</span>' },
    ],
    actions: m => `<button class="btn btn-sm" data-ver="${m.id}" title="Ver sus atletas">${svg('<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/>')}</button>` + accionesHTML(m.id),
    onVer: m => verAtletasDe(m), onEdit: m => abrirEntrenador(m), onDelete: m => borrarEntrenador(m),
  });
  dtEntrenadores.setData(data);
}
function verAtletasDe(m) {
  state.filtro.q = ''; $('#f-texto').value = '';
  state.filtro.match = 'all';
  state.filtro.cond = [{ field: 'maestro', op: 'eq', value: String(m.id) }];
  $('#f-fdesde').value = ''; $('#f-fhasta').value = '';
  renderAdvRows(); syncFacetsFromCond();
  irARuta('atletas'); applyFiltro();
  toast(`Mostrando atletas de ${m.nombres} ${m.apellidos}`);
}
$('#btn-nuevo-entrenador').addEventListener('click', () => abrirEntrenador(null));
$('#ent-cinturon').addEventListener('change', actualizarEntDan);
function actualizarEntDan() { const c = cinturonPorId($('#ent-cinturon').value); const negro = c && c.es_negro; $('#ent-dan-wrap').classList.toggle('hidden', !negro); if (!negro) $('#ent-dan').value = ''; }
function abrirEntrenador(m) {
  $('#form-entrenador').reset();
  $('#ent-error').textContent = '';
  $$('#form-entrenador .field-err').forEach(el => el.classList.remove('field-err'));
  $('#ent-title').textContent = m ? 'Editar entrenador' : 'Nuevo entrenador';
  $('#ent-id').value = m ? m.id : '';
  $('#ent-cedula-tipo').value = (m && m.cedula_tipo) || 'V';
  if (m) {
    $('#ent-nombres').value = m.nombres; $('#ent-apellidos').value = m.apellidos;
    $('#ent-cedula-numero').value = m.cedula_numero || '';
    $('#ent-telefono').value = m.telefono || '';
    $('#ent-escuela').value = m.escuela_id || '';
    $('#ent-cinturon').value = m.cinturon_id || '';
    $('#ent-dan').value = m.dan || '';
    $('#ent-activo').checked = !!m.activo;
  } else { $('#ent-activo').checked = true; }
  actualizarEntDan();
  abrir(modalEntrenador);
}
async function borrarEntrenador(m) {
  if (!await confirmar(`¿Eliminar al entrenador "${m.nombres} ${m.apellidos}"?`, { peligro: true, ok: 'Eliminar' })) return;
  try { await api('/api/entrenadores/' + m.id, { method: 'DELETE' }); toast('Entrenador eliminado'); await cargarCatalogos(); cargarEntrenadores(); }
  catch (e) { toast(e.message, 'err'); }
}
$('#form-entrenador').addEventListener('submit', async (e) => {
  e.preventDefault();
  $('#ent-error').textContent = '';
  $$('#form-entrenador .field-err').forEach(el => el.classList.remove('field-err'));
  const cedNum = valOrNull('#ent-cedula-numero');
  const danVisible = !$('#ent-dan-wrap').classList.contains('hidden');
  const body = {
    nombres: val('#ent-nombres'), apellidos: val('#ent-apellidos'),
    cedula_tipo: cedNum ? val('#ent-cedula-tipo') : null, cedula_numero: cedNum,
    telefono: valOrNull('#ent-telefono'),
    escuela_id: intOrNull('#ent-escuela'), cinturon_id: intOrNull('#ent-cinturon'),
    dan: (danVisible && $('#ent-dan').value) ? parseInt($('#ent-dan').value, 10) : null,
    activo: $('#ent-activo').checked,
  };
  const id = $('#ent-id').value;
  try {
    if (id) await api('/api/entrenadores/' + id, { method: 'PUT', body: JSON.stringify(body) });
    else await api('/api/entrenadores', { method: 'POST', body: JSON.stringify(body) });
    toast('Entrenador guardado'); cerrar(modalEntrenador); await cargarCatalogos(); cargarEntrenadores();
  } catch (err) {
    if (err.fields) { const map = { nombres: '#ent-nombres', apellidos: '#ent-apellidos', cedula_numero: '#ent-cedula-numero', dan: '#ent-dan' }; Object.keys(err.fields).forEach(k => { if (map[k]) $(map[k]).classList.add('field-err'); }); $('#ent-error').textContent = Object.values(err.fields)[0]; }
    else $('#ent-error').textContent = err.message;
  }
});

/* ============================================================================
   DATOS MAESTROS
   ============================================================================ */
const modalMaestro = $('#modal-maestro');
const GM = () => ({ estado: $('#mst-estado'), ciudad: $('#mst-ciudad'), municipio: $('#mst-municipio') });
geoWire(GM());
let dtMaestro = null, dtMaestroTipo = null;
function maestroColumns(tipo) {
  if (tipo === 'estados') return [{ label: 'Nombre', value: x => x.nombre, type: 'text' }];
  if (tipo === 'ciudades') return [{ label: 'Nombre', value: x => x.nombre, type: 'text' }, { label: 'Estado', value: x => nombreGeo('estados', x.estado_id), type: 'text' }];
  if (tipo === 'municipios') return [{ label: 'Nombre', value: x => x.nombre, type: 'text' }, { label: 'Ciudad', value: x => nombreGeo('ciudades', x.ciudad_id), type: 'text' }, { label: 'Estado', value: x => nombreGeo('estados', x.estado_id), type: 'text' }];
  if (tipo === 'parroquias') return [{ label: 'Nombre', value: x => x.nombre, type: 'text' }, { label: 'Municipio', value: x => nombreGeo('municipios', x.municipio_id), type: 'text' }, { label: 'Estado', value: x => nombreGeo('estados', x.estado_id), type: 'text' }];
  return [{ label: 'Color', value: x => x.color, type: 'text' }, { label: 'Orden', value: x => x.orden, type: 'number' }, { label: 'Habilita DAN', value: x => x.es_negro ? 'Sí' : 'No', type: 'bool' }];
}
$$('#maestro-tabs .subtab').forEach(b => b.addEventListener('click', () => {
  $$('#maestro-tabs .subtab').forEach(x => x.classList.remove('active'));
  b.classList.add('active'); cargarMaestro(b.dataset.tipo);
}));
async function cargarMaestro(tipo) {
  state.maestroTipo = tipo;
  let data;
  try { data = await api('/api/maestros/' + tipo); } catch (e) { return toast(e.message, 'err'); }
  state.maestroData = data;
  // Recrear la tabla si cambió el tipo (columnas distintas).
  if (dtMaestroTipo !== tipo) {
    dtMaestro = DataTable({ mount: '#dt-maestro', columns: maestroColumns(tipo), actions: x => accionesHTML(x.id), onEdit: x => abrirMaestro(x), onDelete: x => borrarMaestro(x) });
    dtMaestroTipo = tipo;
  }
  dtMaestro.setData(data);
}
$('#btn-nuevo-maestro').addEventListener('click', () => abrirMaestro(null));
function abrirMaestro(x) {
  const tipo = state.maestroTipo;
  $('#form-maestro').reset();
  $('#mst-error').textContent = '';
  $('#mst-id').value = x ? x.id : '';
  $('#mst-title').textContent = (x ? 'Editar ' : 'Nuevo ') + (MAESTRO_SINGULAR[tipo] || tipo);
  const show = (sel, on) => $(sel).classList.toggle('hidden', !on);
  show('#mst-estado-wrap', ['ciudades', 'municipios', 'parroquias'].includes(tipo));
  show('#mst-ciudad-wrap', ['municipios', 'parroquias'].includes(tipo));
  show('#mst-municipio-wrap', tipo === 'parroquias');
  show('#mst-cinturon-extra', tipo === 'cinturones');
  $('#mst-nombre').placeholder = tipo === 'cinturones' ? 'Color' : 'Nombre';
  if (tipo === 'cinturones') {
    $('#mst-nombre').value = x ? x.color : '';
    $('#mst-orden').value = x ? x.orden : (state.maestroData.length + 1);
    $('#mst-esnegro').checked = x ? !!x.es_negro : false;
  } else {
    $('#mst-nombre').value = x ? x.nombre : '';
    geoSet(GM(), x ? { estado_id: x.estado_id, ciudad_id: x.ciudad_id, municipio_id: x.municipio_id } : {});
  }
  abrir(modalMaestro);
}
async function borrarMaestro(x) {
  if (!await confirmar('¿Eliminar este registro?', { peligro: true, ok: 'Eliminar' })) return;
  try { await api('/api/maestros/' + state.maestroTipo + '/' + x.id, { method: 'DELETE' }); toast('Eliminado'); await cargarGeo(); await cargarCatalogos(); cargarMaestro(state.maestroTipo); }
  catch (e) { toast(e.message, 'err'); }
}
$('#form-maestro').addEventListener('submit', async (e) => {
  e.preventDefault();
  $('#mst-error').textContent = '';
  const tipo = state.maestroTipo, id = $('#mst-id').value;
  const body = {};
  if (tipo === 'cinturones') {
    body.color = val('#mst-nombre'); body.orden = parseInt($('#mst-orden').value, 10) || 0; body.es_negro = $('#mst-esnegro').checked;
    if (!body.color) { $('#mst-error').textContent = 'El color es obligatorio.'; return; }
  } else {
    body.nombre = val('#mst-nombre');
    if (!body.nombre) { $('#mst-error').textContent = 'El nombre es obligatorio.'; return; }
    if (['ciudades', 'municipios', 'parroquias'].includes(tipo)) body.estado_id = intOrNull('#mst-estado');
    if (['municipios', 'parroquias'].includes(tipo)) body.ciudad_id = intOrNull('#mst-ciudad');
    if (tipo === 'parroquias') body.municipio_id = intOrNull('#mst-municipio');
    if (tipo === 'ciudades' && !body.estado_id) { $('#mst-error').textContent = 'Seleccione el estado.'; return; }
    if (tipo === 'municipios' && !body.estado_id && !body.ciudad_id) { $('#mst-error').textContent = 'Seleccione estado o ciudad.'; return; }
    if (tipo === 'parroquias' && !body.municipio_id) { $('#mst-error').textContent = 'Seleccione el municipio.'; return; }
  }
  try {
    if (id) await api('/api/maestros/' + tipo + '/' + id, { method: 'PUT', body: JSON.stringify(body) });
    else await api('/api/maestros/' + tipo, { method: 'POST', body: JSON.stringify(body) });
    toast('Guardado'); cerrar(modalMaestro); await cargarGeo(); await cargarCatalogos(); cargarMaestro(tipo);
  } catch (err) { $('#mst-error').textContent = err.message; }
});

/* ============================================================================
   RESPALDO
   ============================================================================ */
function initRespaldo() {
  const opts = TABLAS_RESPALDO.map(t => `<option value="${t}">${esc(TABLA_LABEL[t] || t)}</option>`).join('');
  $('#exp-tabla').innerHTML = opts; $('#imp-tabla').innerHTML = opts;
}
$('#btn-dl-db').addEventListener('click', () => descargar('/api/backup/db'));
$('#btn-exp').addEventListener('click', () => descargar('/api/backup/tabla/' + $('#exp-tabla').value + '.csv'));
$('#btn-imp').addEventListener('click', async () => {
  const tabla = $('#imp-tabla').value;
  const file = $('#imp-file').files[0];
  if (!file) { toast('Seleccione un archivo CSV', 'err'); return; }
  if (!await confirmar(`Esto REEMPLAZARÁ por completo la tabla "${TABLA_LABEL[tabla] || tabla}". ¿Continuar?`, { peligro: true, ok: 'Reemplazar' })) return;
  $('#imp-msg').textContent = 'Importando…';
  try {
    const txt = await file.text();
    const res = await fetch('/api/backup/tabla/' + tabla, { method: 'POST', headers: { 'Content-Type': 'text/csv' }, body: txt });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'error');
    $('#imp-msg').textContent = `Importadas ${data.insertadas} fila(s).`;
    toast('Importación completa'); await cargarGeo(); await cargarCatalogos(); await cargarLista();
  } catch (e) { $('#imp-msg').textContent = ''; toast(e.message, 'err'); }
});

/* ============================================================================
   MI CUENTA
   ============================================================================ */
$('#btn-cuenta').addEventListener('click', () => {
  $('#acc-error').textContent = '';
  $('#form-cuenta').reset();
  $$('#form-cuenta .field-err').forEach(el => el.classList.remove('field-err'));
  $('#acc-user').value = state.me.username;
  validarCuenta(); abrir($('#modal-cuenta'));
});
$$('[data-eye]').forEach(btn => btn.addEventListener('click', () => { const inp = $('#' + btn.dataset.eye); inp.type = inp.type === 'password' ? 'text' : 'password'; btn.classList.toggle('on', inp.type === 'text'); }));
function validarCuenta() {
  const user = $('#acc-user').value.trim();
  const p1 = $('#acc-pass').value, p2 = $('#acc-pass2').value;
  const cambia = p1 !== '' || p2 !== '';
  const checks = { len: p1.length >= 8, upper: /[A-Z]/.test(p1), lower: /[a-z]/.test(p1), digit: /[0-9]/.test(p1), match: p1 !== '' && p1 === p2 };
  $$('#pw-checks li').forEach(li => { li.classList.toggle('ok', !!checks[li.dataset.check]); li.classList.toggle('idle', !cambia); });
  const passOk = !cambia || (checks.len && checks.upper && checks.lower && checks.digit && checks.match);
  $('#acc-submit').disabled = !(user.length >= 3 && passOk);
}
$('#form-cuenta').addEventListener('input', validarCuenta);
$('#form-cuenta').addEventListener('submit', async (e) => {
  e.preventDefault();
  $('#acc-error').textContent = '';
  $$('#form-cuenta .field-err').forEach(el => el.classList.remove('field-err'));
  const body = {};
  const user = $('#acc-user').value.trim(), p1 = $('#acc-pass').value;
  if (user && user !== state.me.username) body.username = user;
  if (p1 !== '') body.password = p1;
  if (Object.keys(body).length === 0) { toast('No hay cambios que guardar'); cerrar($('#modal-cuenta')); return; }
  try { state.me = await api('/api/profile', { method: 'POST', body: JSON.stringify(body) }); renderBadge(); toast('Cuenta actualizada'); cerrar($('#modal-cuenta')); }
  catch (err) {
    if (err.fields) { if (err.fields.username) $('#acc-user').classList.add('field-err'); if (err.fields.password) $('#acc-pass').classList.add('field-err'); $('#acc-error').textContent = err.fields.username || err.fields.password || 'Revise los datos.'; }
    else $('#acc-error').textContent = err.message;
  }
});

/* ---------------------- WebSocket ---------------------- */
let ws, wsRetry;
function conectarWS() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  ws = new WebSocket(`${proto}://${location.host}/ws`);
  ws.onopen = () => $('#ws-dot').classList.add('on');
  ws.onclose = () => { $('#ws-dot').classList.remove('on'); clearTimeout(wsRetry); if (!$('#view-app').classList.contains('hidden')) wsRetry = setTimeout(conectarWS, 3000); };
  ws.onmessage = (ev) => {
    let e; try { e = JSON.parse(ev.data); } catch { return; }
    const mio = e.por && e.por === state.me.username;
    if (e.recurso === 'atleta') { if (state.route === 'atletas') cargarLista(); }
    else if (['estado', 'ciudad', 'municipio', 'parroquia', 'cinturon', 'escuela'].includes(e.recurso)) {
      cargarGeo(); cargarCatalogos();
      if (state.route === 'datos') cargarMaestro(state.maestroTipo);
      if (state.route === 'escuelas') cargarEscuelas();
    } else if (e.recurso === 'maestro') { cargarCatalogos(); if (state.route === 'entrenadores') cargarEntrenadores(); }
    else if (e.recurso === 'usuario' && state.route === 'usuarios') cargarUsuarios();
    if (!mio && e.por) toast(`${e.por} actualizó datos`, 'ok');
  };
}

/* ---------------------- Utilidades ---------------------- */
$$('[data-close]').forEach(b => b.addEventListener('click', () => cerrar(b.closest('.modal-backdrop'))));
$$('.modal-backdrop').forEach(m => m.addEventListener('mousedown', (e) => { if (e.target === m && m.id !== 'modal-confirm' && m.id !== 'modal-prompt') cerrar(m); }));

function limitInput(el, { allow, max, maxLen } = {}) {
  el.addEventListener('input', () => {
    let v = el.value;
    if (allow) v = [...v].filter(ch => allow.test(ch)).join('');
    if (maxLen) v = v.slice(0, maxLen);
    if (max != null && v !== '' && parseInt(v, 10) > max) v = String(max);
    if (v !== el.value) el.value = v;
  });
}
function val(s) { return $(s).value.trim(); }
function valOrNull(s) { const v = $(s).value.trim(); return v === '' ? null : v; }
function intOrNull(s) { const v = $(s).value; return v ? parseInt(v, 10) : null; }
function hoy() { return new Date().toISOString().slice(0, 10); }
// descargar: baja un archivo (PDF/DB/CSV) en la misma pestaña, sin abrir otra.
// El servidor envía Content-Disposition: attachment, así que el navegador lo
// guarda sin navegar fuera de la app.
function descargar(url) {
  const a = document.createElement('a');
  a.href = url; a.download = '';
  document.body.appendChild(a); a.click(); a.remove();
}
function soloMesAnio(f) { return f ? f.slice(0, 7) : f; }
function pad2(n) { return String(n).padStart(2, '0'); }
function kv(k, v) { return `<div><div class="k">${k}</div>${esc(String(v))}</div>`; }
function cedulaFmt(tipo, numero) { if (tipo && numero) return `${tipo}-${numero}`; return numero || '—'; }
function esc(s) { return String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c])); }

/* ---------------------- Arranque ---------------------- */
function initLimiters() {
  llenarMeses($('#a-nac-mes')); llenarMeses($('#a-ins-mes'));
  ['#a-nombres', '#a-apellidos', '#r-nombres', '#r-apellidos'].forEach(s => limitInput($(s), { allow: NAME_ALLOW, maxLen: 60 }));
  ['#esc-nombre', '#mst-nombre'].forEach(s => limitInput($(s), { allow: PLACE_ALLOW, maxLen: 80 }));
  limitInput($('#usr-nombre'), { allow: NAME_ALLOW, maxLen: 80 });
  limitInput($('#usr-username'), { allow: USER_ALLOW, maxLen: 40 });
  ['#ent-nombres', '#ent-apellidos'].forEach(s => limitInput($(s), { allow: NAME_ALLOW, maxLen: 60 }));
  limitInput($('#ent-cedula-numero'), { allow: /[0-9]/, maxLen: 9 });
  limitInput($('#ent-telefono'), { allow: /[0-9+\-\s()]/ });
  limitInput($('#ent-dan'), { allow: /[1-9]/, maxLen: 1 });
  limitInput($('#a-nac-dia'), { allow: /[0-9]/, maxLen: 2, max: 31 });
  limitInput($('#a-nac-anio'), { allow: /[0-9]/, maxLen: 4 });
  limitInput($('#a-ins-dia'), { allow: /[0-9]/, maxLen: 2, max: 31 });
  limitInput($('#a-ins-anio'), { allow: /[0-9]/, maxLen: 4 });
  limitInput($('#a-cedula-numero'), { allow: /[0-9]/, maxLen: 9 });
  limitInput($('#r-cedula-numero'), { allow: /[0-9]/, maxLen: 9 });
  limitInput($('#a-telefono'), { allow: /[0-9+\-\s()]/ });
  limitInput($('#r-telefono'), { allow: /[0-9+\-\s()]/ });
  limitInput($('#a-dan'), { allow: /[1-9]/, maxLen: 1 });
  limitInput($('#mst-orden'), { allow: /[0-9]/, maxLen: 3 });
  limitInput($('#pg-desde'), { allow: /[0-9]/, maxLen: 7 });
  limitInput($('#pg-hasta'), { allow: /[0-9]/, maxLen: 7 });
}
initLimiters();
bindFiltros();
bindPager();
initRespaldo();
iniciar();
