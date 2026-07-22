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
  maxUploadMB: 5,
  // Selección masiva de atletas.
  sel: new Set(),        // ids seleccionados (persisten entre páginas)
  selAll: false,         // "todos los del filtro actual" (aunque no estén en pantalla)
  lastIdx: null,         // última fila marcada (para selección con Shift)
  pageItems: [],         // atletas de la página visible
  // Documentos de la ficha abierta (atleta o entrenador): URL base de la API.
  docBase: null,
  docsData: [],
  docSel: new Set(),
};

const BELT_COLORS = {
  'Blanco': '#ffffff', 'Amarillo': '#E5A93C', 'Naranja': '#E8792B',
  'Verde': '#1E5631', 'Azul': '#0077B6', 'Rojo': '#8B0000', 'Negro': '#141414',
};
const MESES = ['Enero','Febrero','Marzo','Abril','Mayo','Junio','Julio','Agosto','Septiembre','Octubre','Noviembre','Diciembre'];
const PARENTESCOS_COMUNES = ['Madre','Padre','Tutor legal','Abuelo/a','Tío/a','Hermano/a'];
const ADMIN_ROUTES = ['escuelas','entrenadores','usuarios','datos','respaldo','config'];
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
  maestro: 'Entrenadores', documento: 'Documentos de atletas', documento_maestro: 'Documentos de entrenadores',
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
  alert:     svg('<circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>'),
};
const STATE_DOT = '<svg class="ico-dot" viewBox="0 0 8 8"><circle cx="4" cy="4" r="4" fill="currentColor"/></svg>';

/* ---------------------- Tema claro / oscuro ---------------------- */
// Los colores vienen de variables CSS; el tema solo alterna data-theme, con lo
// que cambiar la paleta es cuestión de editar las variables, no el código.
function aplicarTema(t) {
  const modo = t === 'dark' ? 'dark' : 'light';
  document.documentElement.setAttribute('data-theme', modo);
  try { localStorage.setItem('sigat-tema', modo); } catch {}
}
(function initTema() {
  let t = null;
  try { t = localStorage.getItem('sigat-tema'); } catch {}
  if (!t) t = (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) ? 'dark' : 'light';
  aplicarTema(t);
})();
function alternarTema() {
  aplicarTema(document.documentElement.getAttribute('data-theme') === 'dark' ? 'light' : 'dark');
}
$('#btn-tema').addEventListener('click', alternarTema);
$('#btn-tema-login').addEventListener('click', alternarTema);

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

// postForm sube un archivo (multipart/form-data) sin fijar Content-Type (el
// navegador añade el boundary). Devuelve el JSON de respuesta o lanza el error.
async function postForm(path, formData) {
  const res = await fetch(path, { method: 'POST', body: formData });
  if (res.status === 401) { mostrarLogin(); throw new Error('no autenticado'); }
  let data = null;
  const txt = await res.text();
  if (txt) { try { data = JSON.parse(txt); } catch { data = txt; } }
  if (!res.ok) throw new Error((data && data.error) || res.statusText);
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
  await cargarConfigValor();
  await cargarCatalogos(); await cargarGeo();
  renderAdvRows(); await cargarLista();
  routerInit(); conectarWS();
}
// cargarConfigValor lee el límite de subida (disponible para validar en cliente).
async function cargarConfigValor() {
  try { const c = await api('/api/config'); if (c && c.max_upload_mb) state.maxUploadMB = c.max_upload_mb; } catch {}
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
  else if (route === 'config') cargarConfig();
  else if (route === 'respaldo') cargarUltimoRespaldo();
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
  const selectable = opts.selectable !== false;
  const clickable = opts.onOpen || opts.onEdit || opts.onDelete || opts.onVer; // clic en fila → modal detalle
  const canBulk = selectable && opts.delUrl && opts.reload;
  const st = { data: [], search: '', colf: {}, offset: 0, pageSize: opts.pageSize || 15, showFilters: false, sel: new Set(), selAll: false, lastIdx: null, pageRows: [] };
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
    ${selectable ? `<div class="bulk-bar hidden dt-bulk">
      <span class="bulk-count"><b class="dt-seln">0</b> seleccionado(s)</span>
      <button type="button" class="btn btn-ghost btn-sm hidden dt-selall-btn">Seleccionar los <b class="dt-alln">0</b></button>
      <span class="grow"></span>
      ${canBulk ? `<button type="button" class="btn btn-danger btn-sm admin-only dt-bulkdel">${ICON.trash}<span>Eliminar seleccionados</span></button>` : ''}
      <button type="button" class="btn btn-ghost btn-sm dt-bulkclear">Cancelar</button>
    </div>` : ''}
    <div class="table-wrap"><table class="tabla ${clickable ? 'tabla-click' : ''}">
      <thead><tr>${selectable ? '<th class="col-sel"><input type="checkbox" class="row-check dt-selall" title="Seleccionar"></th>' : ''}${cols.map(c => `<th>${esc(c.label)}</th>`).join('')}${opts.actions ? '<th></th>' : ''}</tr></thead>
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

  // Selección de filas (columna de casillas + cabecera tri-estado).
  if (selectable) {
    q('.dt-selall').addEventListener('change', () => {
      const pageIds = st.pageRows.map(r => r.id);
      const allPage = pageIds.length > 0 && pageIds.every(id => st.sel.has(id));
      if (st.selAll) dtClearSel();
      else if (allPage && st.sel.size > 0) { st.selAll = true; dtRefreshSel(); }
      else { pageIds.forEach(id => st.sel.add(id)); st.selAll = false; dtRefreshSel(); }
    });
    q('.dt-bulkclear').addEventListener('click', dtClearSel);
    if (q('.dt-selall-btn')) q('.dt-selall-btn').addEventListener('click', () => { st.selAll = true; dtRefreshSel(); });
    if (canBulk) q('.dt-bulkdel').addEventListener('click', dtBulkDelete);
  }

  // Clic en la fila: casilla → selección; botón de acción → su acción; resto →
  // abrir la modal de detalle del registro.
  q('.dt-tbody').addEventListener('click', (e) => {
    if (e.target.closest('.dt-rowcheck')) return;
    const ed = e.target.closest('[data-edit]'), de = e.target.closest('[data-del]'), vr = e.target.closest('[data-ver]');
    if (ed && opts.onEdit) return opts.onEdit(porId(st.data, ed.dataset.edit));
    if (de && opts.onDelete) return opts.onDelete(porId(st.data, de.dataset.del));
    if (vr && opts.onVer) return opts.onVer(porId(st.data, vr.dataset.ver));
    if (e.target.closest('button')) return;
    const tr = e.target.closest('tr[data-id]');
    if (tr && clickable) {
      const row = porId(st.data, tr.dataset.id);
      // onOpen permite una ficha de detalle propia (p. ej. entrenadores con
      // foto y documentos); si no, se usa la modal genérica de registro.
      if (opts.onOpen) opts.onOpen(row); else abrirRegistro(row, opts);
    }
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
  function dtBindRowChecks() {
    $$('.dt-rowcheck', mount).forEach(chk => chk.addEventListener('click', (e) => {
      e.stopPropagation();
      const idx = +chk.dataset.idx, row = st.pageRows[idx], on = chk.checked;
      st.selAll = false;
      if (e.shiftKey && st.lastIdx != null) {
        const [a, b] = [Math.min(idx, st.lastIdx), Math.max(idx, st.lastIdx)];
        for (let k = a; k <= b; k++) { const it = st.pageRows[k]; if (it) { on ? st.sel.add(it.id) : st.sel.delete(it.id); } }
      } else if (row) { on ? st.sel.add(row.id) : st.sel.delete(row.id); }
      st.lastIdx = idx; dtRefreshSel();
    }));
  }
  function dtRefreshSel() {
    if (!selectable) return;
    const rows = filtered();
    const pageIds = st.pageRows.map(r => r.id);
    const allPage = pageIds.length > 0 && pageIds.every(id => st.sel.has(id));
    const n = st.selAll ? rows.length : st.sel.size;
    q('.dt-bulk').classList.toggle('hidden', n === 0);
    q('.dt-seln').textContent = n;
    const allBtn = q('.dt-selall-btn');
    if (allBtn) { allBtn.classList.toggle('hidden', st.selAll || !allPage || rows.length <= st.pageRows.length); q('.dt-alln').textContent = rows.length; }
    const h = q('.dt-selall');
    if (st.selAll) { h.checked = true; h.indeterminate = false; }
    else if (allPage) { h.checked = false; h.indeterminate = true; }
    else { h.checked = false; h.indeterminate = false; }
    $$('.dt-tbody tr', mount).forEach((tr, i) => {
      const row = st.pageRows[i]; if (!row) return;
      const on = st.selAll || st.sel.has(row.id);
      tr.classList.toggle('row-selected', on);
      const c = tr.querySelector('.dt-rowcheck'); if (c) c.checked = on;
    });
  }
  function dtClearSel() { st.sel.clear(); st.selAll = false; st.lastIdx = null; dtRefreshSel(); }
  async function dtBulkDelete() {
    if (!isAdmin()) return;
    const rows = st.selAll ? filtered() : st.data.filter(r => st.sel.has(r.id));
    if (!rows.length) return;
    if (!await confirmar(`¿Eliminar ${rows.length} registro(s)? Esta acción no se puede deshacer.`, { peligro: true, ok: 'Eliminar' })) return;
    let ok = 0, fail = 0;
    for (const r of rows) { try { await api(opts.delUrl(r), { method: 'DELETE' }); ok++; } catch { fail++; } }
    toast(`Eliminados: ${ok}${fail ? `, con ${fail} error(es)` : ''}`, fail ? 'err' : 'ok');
    dtClearSel(); opts.reload();
  }

  function render() {
    const rows = filtered();
    const total = rows.length;
    if (st.offset >= total && total > 0) st.offset = Math.max(0, Math.floor((total - 1) / st.pageSize) * st.pageSize);
    const page = rows.slice(st.offset, st.offset + st.pageSize);
    st.pageRows = page;
    q('.dt-tbody').innerHTML = page.map((r, i) => {
      const marcado = st.selAll || st.sel.has(r.id);
      const sel = selectable ? `<td class="col-sel"><input type="checkbox" class="row-check dt-rowcheck" data-idx="${i}" ${marcado ? 'checked' : ''}></td>` : '';
      const tds = cols.map(c => `<td>${c.html ? c.html(r) : esc(String(c.value(r) ?? '—'))}</td>`).join('');
      return `<tr data-id="${r.id}" class="${marcado ? 'row-selected' : ''}">${sel}${tds}${opts.actions ? `<td class="acciones">${opts.actions(r)}</td>` : ''}</tr>`;
    }).join('');
    const emptyEl = q('.dt-empty');
    const filtrando = st.search || Object.values(st.colf).some(v => v);
    if (total === 0) { emptyEl.classList.remove('hidden'); emptyEl.textContent = filtrando ? 'No hay registros que coincidan con el filtro.' : 'No hay ningún registro.'; }
    else emptyEl.classList.add('hidden');
    const desde = total === 0 ? 0 : st.offset + 1, hasta = Math.min(st.offset + st.pageSize, total);
    q('.dt-desde').value = desde; q('.dt-hasta').value = hasta; q('.dt-total').textContent = total;
    q('.dt-desde').classList.remove('invalid'); q('.dt-hasta').classList.remove('invalid');
    q('.dt-prev').disabled = st.offset === 0; q('.dt-next').disabled = hasta >= total;
    if (selectable) { dtBindRowChecks(); dtRefreshSel(); }
  }
  return { setData(arr) { st.data = arr || []; st.offset = 0; st.sel.clear(); st.selAll = false; st.lastIdx = null; render(); }, render };
}

// abrirRegistro muestra el detalle de una fila de una DataTable en una modal,
// con acciones de Editar / Eliminar (y opcionalmente Ver) al pie.
const modalRegistro = $('#modal-registro');
function abrirRegistro(row, opts) {
  if (!row) return;
  const cols = opts.columns;
  $('#reg-title').textContent = opts.detailTitle ? opts.detailTitle(row) : String(cols[0].value(row) ?? 'Detalle');
  $('#reg-body').innerHTML = cols.map(c => `<div><div class="k">${esc(c.label)}</div>${c.html ? c.html(row) : esc(String(c.value(row) ?? '—'))}</div>`).join('');
  const acts = [];
  if (opts.onVer) acts.push(`<button class="btn btn-sm" id="reg-ver">${svg('<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/>')}<span>${esc(opts.verLabel || 'Ver')}</span></button>`);
  if (opts.onEdit && isAdmin()) acts.push(`<button class="btn btn-sm" id="reg-editar">${ICON.edit}<span>Editar</span></button>`);
  if (opts.onDelete && isAdmin()) acts.push(`<button class="btn btn-sm btn-danger" id="reg-eliminar">${ICON.trash}<span>Eliminar</span></button>`);
  $('#reg-actions').innerHTML = acts.join('');
  if ($('#reg-ver')) $('#reg-ver').onclick = () => { cerrar(modalRegistro); opts.onVer(row); };
  if ($('#reg-editar')) $('#reg-editar').onclick = () => { cerrar(modalRegistro); opts.onEdit(row); };
  if ($('#reg-eliminar')) $('#reg-eliminar').onclick = () => { cerrar(modalRegistro); opts.onDelete(row); };
  abrir(modalRegistro);
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
  $('#btn-planilla-blanco').addEventListener('click', () => descargar('/api/reportes/planilla-blanco.pdf'));
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
  if (d.value && h.value && h.value < d.value) { h.value = d.value; setDateCond('fecha_inscripcion', 'lte', h.value); }
  setDateCond('fecha_inscripcion', 'gte', d.value);
}
function onFechaHasta() {
  const d = $('#f-fdesde'), h = $('#f-fhasta');
  if (h.value) d.max = h.value; else d.removeAttribute('max');
  if (h.value && d.value && d.value > h.value) { d.value = h.value; setDateCond('fecha_inscripcion', 'gte', d.value); }
  setDateCond('fecha_inscripcion', 'lte', h.value);
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
function applyFiltro() { state.offset = 0; limpiarSeleccion(); cargarLista(); }
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
  state.pageItems = items;
  const empty = $('#empty');
  if (items.length === 0) { empty.classList.remove('hidden'); empty.textContent = filtroActivo() ? 'No hay registros que coincidan con el filtro.' : 'No hay ningún registro.'; }
  else empty.classList.add('hidden');
  $('#tabla-body').innerHTML = items.map((a, i) => {
    const marcado = state.selAll || state.sel.has(a.id);
    return `
    <tr data-id="${a.id}" data-idx="${i}" class="${marcado ? 'row-selected' : ''}">
      <td class="col-sel"><input type="checkbox" class="row-check row-check-item" data-id="${a.id}" data-idx="${i}" ${marcado ? 'checked' : ''}></td>
      <td class="nombre-cell"><b>${esc(a.apellidos)}, ${esc(a.nombres)}</b>${a.es_menor ? '<small>menor de edad</small>' : ''}</td>
      <td>${esc(cedulaFmt(a.cedula_tipo, a.cedula_numero))}</td>
      <td>${a.edad}</td>
      <td>${esc(a.escuela_nombre || '—')}</td>
      <td>${chipCinturon(a.cinturon_color, a.cinturon_dan)}</td>
      <td class="state-${a.estado}">${STATE_DOT} ${a.estado}</td>
      <td class="chev">${ICON.chevron}</td>
    </tr>`;
  }).join('');
  $$('#tabla-body tr').forEach(tr => {
    tr.addEventListener('click', (e) => {
      if (e.target.closest('.col-sel')) return; // clic en la casilla: no abrir la ficha
      verDetalle(tr.dataset.id);
    });
  });
  $$('#tabla-body .row-check-item').forEach(chk => {
    chk.addEventListener('click', (e) => {
      e.stopPropagation();
      const idx = +chk.dataset.idx, id = +chk.dataset.id, on = chk.checked;
      state.selAll = false; // una selección manual cancela "todos los del filtro"
      if (e.shiftKey && state.lastIdx != null) {
        const [a, b] = [Math.min(idx, state.lastIdx), Math.max(idx, state.lastIdx)];
        for (let k = a; k <= b; k++) { const it = state.pageItems[k]; if (it) { on ? state.sel.add(it.id) : state.sel.delete(it.id); } }
      } else {
        on ? state.sel.add(id) : state.sel.delete(id);
      }
      state.lastIdx = idx;
      refreshSelUI();
    });
  });
  refreshSelUI();
}

/* ---------------------- Selección masiva ---------------------- */
function refreshSelUI() {
  const pageIds = state.pageItems.map(a => a.id);
  const allPageSel = pageIds.length > 0 && pageIds.every(id => state.sel.has(id));
  const n = state.selAll ? state.total : state.sel.size;
  $('#bulk-bar').classList.toggle('hidden', n === 0);
  $('#bulk-n').textContent = n;
  // Sugerir "seleccionar todos los del filtro" cuando la página está completa
  // pero hay más resultados fuera de pantalla.
  const showAll = !state.selAll && allPageSel && state.total > state.pageItems.length;
  $('#bulk-all').classList.toggle('hidden', !showAll);
  $('#bulk-all-n').textContent = state.total;
  // Estado del checkbox de cabecera:
  //  · azul con check  → todos los registros del filtro (selAll)
  //  · azul con rayita → todos los visibles seleccionados (pero no selAll)
  //  · blanco          → cualquier otro caso (incluido 1 solo seleccionado)
  const h = $('#sel-all');
  if (state.selAll) { h.checked = true; h.indeterminate = false; }
  else if (allPageSel && pageIds.length > 0) { h.checked = false; h.indeterminate = true; }
  else { h.checked = false; h.indeterminate = false; }
  // Resaltado de filas.
  $$('#tabla-body tr').forEach(tr => {
    const id = +tr.dataset.id, marcado = state.selAll || state.sel.has(id);
    tr.classList.toggle('row-selected', marcado);
    const c = tr.querySelector('.row-check-item'); if (c) c.checked = marcado;
  });
}
function limpiarSeleccion() { state.sel.clear(); state.selAll = false; state.lastIdx = null; refreshSelUI(); }
async function idsDelFiltro() {
  const p = filtroParams(); p.set('limit', 100000); p.set('offset', 0);
  const data = await api('/api/atletas?' + p.toString());
  return (data.items || []).map(a => a.id);
}
function bindSeleccion() {
  // Cabecera tri-estado: vacío → visibles → todos (del filtro) → vacío.
  // Se usa 'change' (no 'click'+preventDefault): al cancelar el clic el navegador
  // restaura el checked/indeterminate previo DESPUÉS del handler, pisando el
  // valor que fijamos. Con 'change' no hay restauración y el estado visual queda.
  $('#sel-all').addEventListener('change', () => {
    const pageIds = state.pageItems.map(a => a.id);
    const allPageSel = pageIds.length > 0 && pageIds.every(id => state.sel.has(id));
    if (state.selAll) { limpiarSeleccion(); }
    else if (allPageSel && state.sel.size > 0) { state.selAll = true; refreshSelUI(); }
    else { pageIds.forEach(id => state.sel.add(id)); state.selAll = false; refreshSelUI(); }
  });
  $('#bulk-all').addEventListener('click', () => { state.selAll = true; refreshSelUI(); });
  $('#bulk-clear').addEventListener('click', limpiarSeleccion);
  $('#bulk-pdf').addEventListener('click', () => {
    if (state.selAll) { descargar('/api/reportes/atletas.pdf?' + filtroParams().toString()); return; }
    if (state.sel.size === 0) return;
    const p = new URLSearchParams(); p.set('ids', [...state.sel].join(','));
    descargar('/api/reportes/atletas.pdf?' + p.toString());
  });
  $('#bulk-del').addEventListener('click', async () => {
    if (!isAdmin()) return;
    let ids;
    try { ids = state.selAll ? await idsDelFiltro() : [...state.sel]; }
    catch (e) { return toast(e.message, 'err'); }
    if (!ids.length) return;
    if (!await confirmar(`¿Eliminar definitivamente ${ids.length} atleta(s)? Esta acción no se puede deshacer.`, { peligro: true, ok: 'Eliminar' })) return;
    let ok = 0, fail = 0;
    for (const id of ids) { try { await api('/api/atletas/' + id, { method: 'DELETE' }); ok++; } catch { fail++; } }
    toast(`Eliminados: ${ok}${fail ? `, con ${fail} error(es)` : ''}`, fail ? 'err' : 'ok');
    limpiarSeleccion(); await cargarLista();
  });
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
  return `<span class="chip chip-belt"><span class="dot" style="background:${dot}"></span>${esc(color)}${dan ? ` ${romano(dan)} DAN` : ''}</span>`;
}
// romano: DAN 1..9 en numeración romana (para mostrar; se guarda como número).
function romano(n) { return ['', 'I', 'II', 'III', 'IV', 'V', 'VI', 'VII', 'VIII', 'IX'][n] || String(n); }

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
  ['#a-estatura', '#a-peso', '#a-imc', '#a-fc'].forEach(s => { $(s).dataset.digits = ''; $(s).value = ''; });
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
    $('#a-sexo').value = atleta.sexo || '';
    $('#a-email').value = atleta.email || '';
    $('#a-direccion').value = atleta.direccion_detalle || '';
    escribirFecha('a-ins', atleta.fecha_inscripcion, !atleta.inscripcion_dia_exacto);
    $('#a-ins-nodia').checked = !atleta.inscripcion_dia_exacto;
    $('#a-horario').value = atleta.horario || '';
    // Datos físicos y estudios.
    smartSet($('#a-estatura'), atleta.estatura);
    smartSet($('#a-peso'), atleta.peso);
    smartSet($('#a-imc'), atleta.imc);
    smartSet($('#a-fc'), atleta.fc);
    $('#a-talla-camisa').value = atleta.talla_camisa || '';
    $('#a-talla-pantalon').value = atleta.talla_pantalon || '';
    $('#a-instituto').value = atleta.instituto || '';
    $('#a-instituto-dir').value = atleta.instituto_direccion || '';
    // Información médica.
    $('#a-med-enfermedad').value = boolToSiNo(atleta.med_enfermedad);
    $('#a-med-enfermedad-det').value = atleta.med_enfermedad_detalle || '';
    $('#a-med-alergia').value = boolToSiNo(atleta.med_alergia);
    $('#a-med-alergia-det').value = atleta.med_alergia_detalle || '';
    $('#a-med-operado').value = boolToSiNo(atleta.med_operado);
    $('#a-med-operado-det').value = atleta.med_operado_detalle || '';
    $('#a-med-emergencia').value = atleta.med_emergencia || '';
    const r = atleta.representante || {};
    $('#r-nombres').value = r.nombres || '';
    $('#r-apellidos').value = r.apellidos || '';
    $('#r-cedula-tipo').value = r.cedula_tipo || 'V';
    $('#r-cedula-numero').value = r.cedula_numero || '';
    $('#r-telefono').value = r.telefono || '';
    $('#r-email').value = r.email || '';
    $('#r-lugar-trabajo').value = r.lugar_trabajo || '';
    $('#r-direccion-trabajo').value = r.direccion_trabajo || '';
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
  toggleDiaInscripcion(); actualizarContactoAdd(); actualizarEdadHint(); actualizarDanWrap(); syncMedica(); validarForm();
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
function toggleParentescoOtro() { $('#r-parentesco-otro').classList.toggle('hidden', $('#r-parentesco-sel').value !== 'Otro'); }
function parentescoVal() { const s = $('#r-parentesco-sel').value; return s === 'Otro' ? valOrNull('#r-parentesco-otro') : (s || null); }
$('#r-parentesco-sel').addEventListener('change', () => { toggleParentescoOtro(); validarForm(); });

$('#a-ins-nodia').addEventListener('change', () => { toggleDiaInscripcion(); validarForm(); });
$('#a-cinturon').addEventListener('change', () => { actualizarDanWrap(); validarForm(); });
['#a-med-enfermedad', '#a-med-alergia', '#a-med-operado'].forEach(s => $(s).addEventListener('change', () => { syncMedica(); validarForm(); }));
geoWire(GA());

// syncMedica: el campo "especifique" solo se habilita cuando la respuesta es
// "Sí"; con "No" o "—" se deshabilita y se limpia.
const MED_CAMPOS = [
  ['a-med-enfermedad', 'a-med-enfermedad-det', 'Especifique la enfermedad que padece'],
  ['a-med-alergia', 'a-med-alergia-det', 'Especifique el medicamento al que es alérgico'],
  ['a-med-operado', 'a-med-operado-det', 'Especifique la operación'],
];
function syncMedica() {
  MED_CAMPOS.forEach(([sel, det]) => {
    const on = $('#' + sel).value === 'si';
    const d = $('#' + det);
    d.disabled = !on;
    if (!on) d.value = '';
  });
}

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
  const cintSel = $('#a-cinturon').value;
  const c = cinturonPorId(cintSel); const negro = c && c.es_negro;
  // Sin cinturón ("No aplica") → ocultar la fecha de asignación.
  $('#cinturon-fecha-wrap').classList.toggle('hidden', !cintSel);
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
  // Información médica: si respondió "Sí", el detalle es obligatorio.
  MED_CAMPOS.forEach(([sel, det, msg]) => {
    if ($('#' + sel).value === 'si' && !val('#' + det)) { motivos.push(msg); markErr('#' + det); }
  });
  renderAyuda(motivos);
  $('#a-submit').disabled = motivos.length > 0;
  actualizarEdadHint();
  return motivos.length === 0;
}
const FIELD_SEL = {
  nombres: '#a-nombres', apellidos: '#a-apellidos', cedula_numero: '#a-cedula-numero', cedula_tipo: '#a-cedula-tipo',
  fecha_nacimiento: '#dt-nac', fecha_inscripcion: '#dt-ins', telefono: '#a-telefono', dan: '#a-dan',
  rep_nombres: '#r-nombres', rep_apellidos: '#r-apellidos', rep_telefono: '#r-telefono', rep_cedula_numero: '#r-cedula-numero',
  med_enfermedad_detalle: '#a-med-enfermedad-det', med_alergia_detalle: '#a-med-alergia-det', med_operado_detalle: '#a-med-operado-det',
};
function pintarErrores(fields) { Object.keys(fields).forEach(k => { const sel = FIELD_SEL[k]; if (sel) $$(sel).forEach(el => el.classList.add('field-err')); }); }
$('#form-atleta').addEventListener('input', validarForm);
$('#form-atleta').addEventListener('change', validarForm);

function representantePayload() {
  const r = {
    cedula_tipo: valOrNull('#r-cedula-tipo'), cedula_numero: valOrNull('#r-cedula-numero'),
    nombres: valOrNull('#r-nombres'), apellidos: valOrNull('#r-apellidos'), telefono: valOrNull('#r-telefono'),
    parentesco: parentescoVal(), email: valOrNull('#r-email'),
    lugar_trabajo: valOrNull('#r-lugar-trabajo'), direccion_trabajo: valOrNull('#r-direccion-trabajo'),
  };
  if (!r.cedula_numero) r.cedula_tipo = null;
  const tieneAlgo = r.nombres || r.apellidos || r.telefono || r.cedula_numero || r.parentesco || r.email || r.lugar_trabajo || r.direccion_trabajo;
  return tieneAlgo ? r : null;
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
    // Campos de la planilla oficial.
    sexo: valOrNull('#a-sexo'), email: valOrNull('#a-email'), horario: valOrNull('#a-horario'),
    estatura: valOrNull('#a-estatura'), peso: valOrNull('#a-peso'), imc: valOrNull('#a-imc'), fc: valOrNull('#a-fc'),
    talla_camisa: valOrNull('#a-talla-camisa'), talla_pantalon: valOrNull('#a-talla-pantalon'),
    instituto: valOrNull('#a-instituto'), instituto_direccion: valOrNull('#a-instituto-dir'),
    med_enfermedad: siNoToBool('#a-med-enfermedad'), med_enfermedad_detalle: valOrNull('#a-med-enfermedad-det'),
    med_alergia: siNoToBool('#a-med-alergia'), med_alergia_detalle: valOrNull('#a-med-alergia-det'),
    med_operado: siNoToBool('#a-med-operado'), med_operado_detalle: valOrNull('#a-med-operado-det'),
    med_emergencia: valOrNull('#a-med-emergencia'),
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
  const insc = a.inscripcion_dia_exacto ? fmtFecha(a.fecha_inscripcion) : soloMesAnio(a.fecha_inscripcion);
  const rep = a.representante;
  const contactos = (a.telefonos_contacto || []).length ? a.telefonos_contacto.join(', ') : '—';
  const ubic = [a.parroquia_nombre, a.municipio_nombre, a.ciudad_nombre, a.estado_nombre].filter(Boolean).join(', ') || '—';
  $('#det-title').textContent = `${a.nombres} ${a.apellidos}`;
  $('#det-body').innerHTML = `
    <div class="det-head">
      <div class="det-head-main">
        ${fotoPanelHTML('Foto del atleta')}
        <div class="det-head-info"><h3 style="margin:0">${esc(a.apellidos)}, ${esc(a.nombres)}</h3>
          <div>${chipCinturon(a.cinturon_color, a.cinturon_dan)} <span class="chip state-${a.estado}">${STATE_DOT} ${a.estado}</span></div></div>
      </div>
      <div class="det-pdf-btns">
        <button class="btn btn-sm det-ficha-btn" id="d-planilla">${ICON.download}<span>Planilla (PDF)</span></button>
      </div>
    </div>

    <div class="det-tabs" id="det-tabs">
      <button type="button" class="det-tab active" data-tab="perfil">Perfil</button>
      <button type="button" class="det-tab" data-tab="salud">Salud</button>
      <button type="button" class="det-tab" data-tab="representante">Representante</button>
      <button type="button" class="det-tab" data-tab="historial">Historial</button>
      <button type="button" class="det-tab" data-tab="docs">Documentos</button>
    </div>

    <div class="det-panel" data-panel="perfil">
      <div class="det-grid">
        ${kv('Cédula', cedulaFmt(a.cedula_tipo, a.cedula_numero))}
        ${kv('Edad', a.edad + ' años' + (a.es_menor ? ' (menor)' : ''))}
        ${kv('Sexo', sexoTexto(a.sexo))}
        ${kv('Nacimiento', fmtFecha(a.fecha_nacimiento))}
        ${kv('Inscripción', insc)}
        ${kv('Horario', a.horario || '—')}
        ${kv('Escuela', a.escuela_nombre || '—')}
        ${kv('Entrenador', a.maestro_nombre || '—')}
        ${kv('Ubicación', ubic)}
        ${kv('Teléfono principal', a.telefono || '—')}
        ${kv('Email', a.email || '—')}
        ${kv('Teléfonos de contacto', contactos)}
        ${kv('Dirección', a.direccion_detalle || '—')}
      </div>
    </div>

    <div class="det-panel hidden" data-panel="salud">
      ${saludHTML(a)}
    </div>

    <div class="det-panel hidden" data-panel="representante">
      ${rep ? `<div class="det-grid">
        ${kv('Nombre', `${rep.nombres || ''} ${rep.apellidos || ''}`.trim() || '—')}
        ${kv('Cédula', cedulaFmt(rep.cedula_tipo, rep.cedula_numero))}
        ${kv('Parentesco', rep.parentesco || '—')}
        ${kv('Teléfono', rep.telefono || '—')}
        ${kv('Email', rep.email || '—')}
        ${kv('Lugar de trabajo', rep.lugar_trabajo || '—')}
        ${kv('Dirección de trabajo', rep.direccion_trabajo || '—')}
      </div>` : '<p class="det-empty-tab">Este atleta no tiene representante registrado.</p>'}
    </div>

    <div class="det-panel hidden" data-panel="historial">
      <fieldset><legend>Historial de cinturón</legend>
        <ul class="timeline">${(a.cinturones || []).map(h => `<li><b>${chipCinturon(h.color, h.dan)}</b> <small>— ${fmtFecha(h.fecha_cambio)}</small></li>`).join('') || '<li><small>Sin registros</small></li>'}</ul>
      </fieldset>
      <fieldset><legend>Periodos de actividad</legend>
        <ul class="timeline">${(a.periodos || []).map(p => `<li><b>${fmtFecha(p.fecha_inicio)}</b> <span class="arrow-sep">${ICON.arrow}</span> ${p.fecha_fin ? esc(fmtFecha(p.fecha_fin)) : '<span class="state-activo">activo</span>'} ${p.motivo_retiro ? `<small>(${esc(p.motivo_retiro)})</small>` : ''}</li>`).join('')}</ul>
      </fieldset>
    </div>

    <div class="det-panel hidden" data-panel="docs">
      ${docsFieldsetHTML()}
    </div>
    ${isAdmin() ? `<div class="det-actions">
      <button class="btn btn-sm" id="d-editar">${ICON.edit}<span>Editar</span></button>
      <button class="btn btn-sm" id="d-cinturon">${ICON.belt}<span>Cambiar cinturón</span></button>
      ${a.estado === 'activo' ? `<button class="btn btn-sm" id="d-retirar">${ICON.retire}<span>Retirar</span></button>` : `<button class="btn btn-sm" id="d-reactivar">${ICON.reactivate}<span>Reactivar</span></button>`}
      <button class="btn btn-sm btn-danger" id="d-eliminar">${ICON.trash}<span>Eliminar</span></button>
    </div>` : ''}`;
  const base = '/api/atletas/' + a.id;
  $('#d-planilla').onclick = () => descargar(base + '/planilla.pdf');
  // Pestañas del detalle.
  $$('#det-tabs .det-tab').forEach(t => t.addEventListener('click', () => {
    $$('#det-tabs .det-tab').forEach(x => x.classList.toggle('active', x === t));
    $$('#det-body .det-panel').forEach(p => p.classList.toggle('hidden', p.dataset.panel !== t.dataset.tab));
  }));
  $('#foto-expand').onclick = () => abrirLightboxFoto(base);
  if (isAdmin()) {
    $('#d-editar').onclick = () => { retornoFicha = a.id; cerrar(modalDetalle); abrirForm(a); };
    $('#d-cinturon').onclick = () => cambiarCinturon(a);
    if ($('#d-retirar')) $('#d-retirar').onclick = () => retirar(a);
    if ($('#d-reactivar')) $('#d-reactivar').onclick = () => reactivar(a);
    $('#d-eliminar').onclick = () => eliminar(a);
    $('#btn-foto-set').onclick = () => pedirFoto(base);
    $('#btn-foto-ajustar').onclick = () => ajustarFotoActual(base);
    $('#btn-foto-del').onclick = () => quitarFoto(base);
    $('#doc-add').onclick = () => abrirSubirDocumento(base);
  }
  cargarFotoDetalle(base);
  initDocsUI(base);
  abrir(modalDetalle);
}
// saludHTML muestra los datos de salud/planilla (tipo de sangre, físicos, médicos).
function saludHTML(a) {
  const med = (estado, det) => siNoTexto(estado) + (det ? ` — ${det}` : '');
  return `<div class="det-grid">
    ${kv('Tipo de sangre', a.tipo_sangre || '—')}
    ${kv('Estatura', a.estatura || '—')}
    ${kv('Peso', a.peso || '—')}
    ${kv('I.M.C.', a.imc || '—')}
    ${kv('F.C.', a.fc || '—')}
    ${kv('Talla de camisa', a.talla_camisa || '—')}
    ${kv('Talla de pantalón', a.talla_pantalon || '—')}
    ${kv('Instituto donde estudia', a.instituto || '—')}
    ${kv('Dirección de la institución', a.instituto_direccion || '—')}
  </div>
  <fieldset style="margin-top:1rem"><legend>Información médica</legend><div class="det-grid">
    ${kv('¿Padece enfermedad?', med(a.med_enfermedad, a.med_enfermedad_detalle))}
    ${kv('¿Alérgico a medicamento?', med(a.med_alergia, a.med_alergia_detalle))}
    ${kv('¿Ha sido operado?', med(a.med_operado, a.med_operado_detalle))}
    ${kv('Emergencia: llamar a', a.med_emergencia || '—')}
  </div></fieldset>`;
}
// fotoPanelHTML: panel de foto reutilizable (mismos ids para atleta/entrenador;
// solo hay una ficha abierta a la vez, así que no colisionan).
function fotoPanelHTML(altText) {
  return `<div class="foto-panel">
    <div class="foto-box" id="foto-box"><img id="foto-img" alt="${esc(altText)}">
      <button type="button" class="foto-expand" id="foto-expand" title="Ver foto en grande" aria-label="Ver foto en grande">
        <svg class="ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 3 21 3 21 9"/><polyline points="9 21 3 21 3 15"/><line x1="21" y1="3" x2="14" y2="10"/><line x1="3" y1="21" x2="10" y2="14"/></svg>
      </button>
    </div>
    ${isAdmin() ? `<div class="foto-actions">
      <button class="btn btn-sm" id="btn-foto-set" title="Subir o renovar la foto">Subir foto</button>
      <button class="btn btn-sm hidden" id="btn-foto-ajustar" title="Reencuadrar la foto actual">Ajustar</button>
      <button class="btn btn-sm hidden" id="btn-foto-del" title="Quitar la foto">Quitar</button>
    </div>` : ''}
  </div>`;
}
// docsFieldsetHTML: repositorio de documentos reutilizable (mismos ids).
function docsFieldsetHTML() {
  return `<div id="fs-docs">
    <div class="docs-toolbar">
      ${isAdmin() ? `<button class="btn btn-sm btn-primary" id="doc-add">${ICON.download}<span>Subir documento</span></button>` : ''}
      <div class="docs-selbar hidden" id="docs-selbar">
        <span class="bulk-count"><b id="docs-seln">0</b> sel.</span>
        <button class="btn btn-sm" id="doc-open" title="Abrir en pestañas nuevas">Abrir</button>
        <button class="btn btn-sm" id="doc-dl" title="Descargar (varios = .zip)">Descargar</button>
        ${isAdmin() ? `<button class="btn btn-sm btn-danger" id="doc-del">Eliminar</button>` : ''}
        <button class="btn btn-sm btn-ghost" id="doc-desel">Quitar selección</button>
      </div>
    </div>
    <div class="docs-grid" id="docs-grid"></div>
    <div class="docs-empty hidden" id="docs-empty">No hay documentos. ${isAdmin() ? 'Use “Subir documento” para agregar archivos PDF o imágenes.' : ''}</div>
  </div>`;
}

/* ---------------------- Detalle del entrenador ---------------------- */
// Ficha del entrenador deportivo: reutiliza el panel de foto y el repositorio
// de documentos del atleta, apuntando a /api/entrenadores/{id}.
function verDetalleEntrenador(m) {
  const base = '/api/entrenadores/' + m.id;
  $('#det-title').textContent = `${m.nombres} ${m.apellidos}`;
  $('#det-body').innerHTML = `
    <div class="det-head">
      <div class="det-head-main">
        ${fotoPanelHTML('Foto del entrenador')}
        <div class="det-head-info"><h3 style="margin:0">${esc(m.apellidos)}, ${esc(m.nombres)}</h3>
          <div>${chipCinturon(m.cinturon_color, m.dan)} ${m.activo ? '<span class="chip state-activo">' + STATE_DOT + ' activo</span>' : '<span class="chip state-retirado">' + STATE_DOT + ' inactivo</span>'}</div></div>
      </div>
    </div>
    <div class="det-grid">
      ${kv('Cédula', cedulaFmt(m.cedula_tipo, m.cedula_numero))}
      ${kv('Teléfono', m.telefono || '—')}
      ${kv('Escuela', m.escuela_nombre || '—')}
      ${kv('Atletas asignados', m.num_atletas)}
    </div>
    <fieldset style="margin-top:.4rem"><legend>Documentos</legend>
      ${docsFieldsetHTML()}
    </fieldset>
    <div class="det-actions">
      <button class="btn btn-sm" id="d-ver-atletas">${svg('<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/>')}<span>Ver sus atletas</span></button>
      ${isAdmin() ? `
        <button class="btn btn-sm" id="d-editar">${ICON.edit}<span>Editar</span></button>
        <button class="btn btn-sm btn-danger" id="d-eliminar">${ICON.trash}<span>Eliminar</span></button>` : ''}
    </div>`;
  $('#foto-expand').onclick = () => abrirLightboxFoto(base);
  $('#d-ver-atletas').onclick = () => { cerrar(modalDetalle); verAtletasDe(m); };
  if (isAdmin()) {
    $('#btn-foto-set').onclick = () => pedirFoto(base);
    $('#btn-foto-ajustar').onclick = () => ajustarFotoActual(base);
    $('#btn-foto-del').onclick = () => quitarFoto(base);
    $('#doc-add').onclick = () => abrirSubirDocumento(base);
    $('#d-editar').onclick = () => { cerrar(modalDetalle); abrirEntrenador(m); };
    $('#d-eliminar').onclick = () => { cerrar(modalDetalle); borrarEntrenador(m); };
  }
  cargarFotoDetalle(base);
  initDocsUI(base);
  abrir(modalDetalle);
}

// abrirLightboxFoto muestra la foto en grande (overlay), con cierre.
function abrirLightboxFoto(base) {
  const box = $('#foto-box');
  if (box && box.classList.contains('is-logo')) { toast('No hay foto para mostrar', 'ok'); return; }
  const lb = $('#lightbox'), img = $('#lightbox-img');
  img.src = `${base}/foto?t=${Date.now()}`;
  abrir(lb);
}

/* ---------------------- Foto (atleta o entrenador) ---------------------- */
// Todas las funciones de foto trabajan sobre una URL base de la entidad
// ('/api/atletas/{id}' o '/api/entrenadores/{id}'), reutilizando la misma UI.
// Carga la foto en la ficha; si no tiene (o no hay acceso), muestra el logo de
// la academia como marcador y ajusta los botones (Subir/Cambiar/Quitar).
function cargarFotoDetalle(base) {
  const img = $('#foto-img'), box = $('#foto-box');
  if (!img) return;
  box.classList.remove('is-logo');
  img.onerror = () => { img.onerror = null; box.classList.add('is-logo'); img.src = '/logo_academia.png'; marcarBotonesFoto(false); };
  img.onload = () => { if (img.src.indexOf('logo_academia') < 0) { box.classList.remove('is-logo'); marcarBotonesFoto(true); } };
  img.src = `${base}/foto?t=${Date.now()}`;
}
// marcarBotonesFoto: "Subir foto" (sin foto) vs "Cambiar foto" (con foto), y
// muestra "Ajustar" y "Quitar" solo cuando hay foto.
function marcarBotonesFoto(tiene) {
  const set = $('#btn-foto-set'), del = $('#btn-foto-del'), aj = $('#btn-foto-ajustar');
  if (set) set.textContent = tiene ? 'Cambiar foto' : 'Subir foto';
  if (del) del.classList.toggle('hidden', !tiene);
  if (aj) aj.classList.toggle('hidden', !tiene);
}
// ajustarFotoActual: carga la foto actual en el recortador para reencuadrarla
// sin volver a subir el archivo.
async function ajustarFotoActual(base) {
  try {
    const res = await fetch(`${base}/foto?t=${Date.now()}`);
    if (!res.ok) return toast('No se pudo cargar la foto actual', 'err');
    const blob = await res.blob();
    fotoBase = base;
    abrirCropper(blob);
  } catch { toast('No se pudo cargar la foto actual', 'err'); }
}
let fotoBase = null; // URL base de la entidad cuya foto se está gestionando
function pedirFoto(base) { fotoBase = base; const f = $('#foto-file'); f.value = ''; f.click(); }
$('#foto-file').addEventListener('change', () => {
  const f = $('#foto-file').files[0];
  if (!f || fotoBase == null) return;
  const okTipo = /\.(jpe?g|png)$/i.test(f.name) || ['image/jpeg', 'image/png'].includes(f.type);
  if (!okTipo) return toast('Formato no admitido: use JPG, JPEG o PNG', 'err');
  if (f.size > state.maxUploadMB * 1024 * 1024) return toast(`La imagen supera el máximo de ${state.maxUploadMB} MB`, 'err');
  abrirCropper(f);
});
async function quitarFoto(base) {
  if (!await confirmar('¿Quitar esta foto?', { peligro: true, ok: 'Quitar' })) return;
  try { await api(`${base}/foto`, { method: 'DELETE' }); toast('Foto eliminada'); cargarFotoDetalle(base); }
  catch (e) { toast(e.message, 'err'); }
}

/* ---------------------- Recorte de foto (cropper) ---------------------- */
// El recorte produce una imagen con la MISMA proporción que el recuadro de la
// planilla (5:6), de modo que la foto se vea completa y sin deformar.
const cropper = { img: null, natW: 0, natH: 0, scale: 1, minScale: 1, tx: 0, ty: 0, vpW: 250, vpH: 300, dragging: false, lastX: 0, lastY: 0, url: null };
function abrirCropper(file) {
  if (cropper.url) URL.revokeObjectURL(cropper.url);
  cropper.url = URL.createObjectURL(file);
  const im = new Image();
  im.onload = () => {
    cropper.img = im; cropper.natW = im.naturalWidth; cropper.natH = im.naturalHeight;
    cropper.minScale = Math.max(cropper.vpW / cropper.natW, cropper.vpH / cropper.natH);
    cropper.scale = cropper.minScale;
    cropper.tx = (cropper.vpW - cropper.natW * cropper.scale) / 2;
    cropper.ty = (cropper.vpH - cropper.natH * cropper.scale) / 2;
    $('#crop-img').src = cropper.url;
    $('#crop-zoom').value = '1';
    $('#crop-error').textContent = '';
    aplicarCropTransform();
    abrir($('#modal-crop'));
  };
  im.onerror = () => toast('No se pudo leer la imagen', 'err');
  im.src = cropper.url;
}
function clampCrop() {
  const w = cropper.natW * cropper.scale, h = cropper.natH * cropper.scale;
  cropper.tx = Math.min(0, Math.max(cropper.vpW - w, cropper.tx));
  cropper.ty = Math.min(0, Math.max(cropper.vpH - h, cropper.ty));
}
function aplicarCropTransform() {
  clampCrop();
  const el = $('#crop-img');
  el.style.width = (cropper.natW * cropper.scale) + 'px';
  el.style.height = (cropper.natH * cropper.scale) + 'px';
  el.style.transform = `translate(${cropper.tx}px, ${cropper.ty}px)`;
}
$('#crop-zoom').addEventListener('input', (e) => {
  const oldScale = cropper.scale, newScale = cropper.minScale * parseFloat(e.target.value);
  const cx = cropper.vpW / 2, cy = cropper.vpH / 2;
  cropper.tx = cx - (cx - cropper.tx) * (newScale / oldScale);
  cropper.ty = cy - (cy - cropper.ty) * (newScale / oldScale);
  cropper.scale = newScale;
  aplicarCropTransform();
});
(function () {
  const vp = $('#crop-vp');
  vp.addEventListener('pointerdown', (e) => { cropper.dragging = true; cropper.lastX = e.clientX; cropper.lastY = e.clientY; vp.setPointerCapture(e.pointerId); });
  vp.addEventListener('pointermove', (e) => { if (!cropper.dragging) return; cropper.tx += e.clientX - cropper.lastX; cropper.ty += e.clientY - cropper.lastY; cropper.lastX = e.clientX; cropper.lastY = e.clientY; aplicarCropTransform(); });
  vp.addEventListener('pointerup', () => { cropper.dragging = false; });
  vp.addEventListener('pointercancel', () => { cropper.dragging = false; });
})();
$('#crop-ok').addEventListener('click', () => {
  const outW = 600, outH = 720;
  const canvas = document.createElement('canvas'); canvas.width = outW; canvas.height = outH;
  const ctx = canvas.getContext('2d');
  const sx = -cropper.tx / cropper.scale, sy = -cropper.ty / cropper.scale;
  const sw = cropper.vpW / cropper.scale, sh = cropper.vpH / cropper.scale;
  ctx.drawImage(cropper.img, sx, sy, sw, sh, 0, 0, outW, outH);
  canvas.toBlob(async (blob) => {
    if (!blob) { $('#crop-error').textContent = 'No se pudo procesar la imagen.'; return; }
    if (blob.size > state.maxUploadMB * 1024 * 1024) { $('#crop-error').textContent = `La imagen supera ${state.maxUploadMB} MB.`; return; }
    try {
      const fd = new FormData(); fd.append('foto', blob, 'foto.jpg');
      await postForm(`${fotoBase}/foto`, fd);
      toast('Foto actualizada'); cerrar($('#modal-crop')); cargarFotoDetalle(fotoBase);
    } catch (e) { $('#crop-error').textContent = e.message; }
  }, 'image/jpeg', 0.9);
});
// Lightbox: cierre por botón o clic en el fondo.
$('#lightbox-x').addEventListener('click', () => cerrar($('#lightbox')));
$('#lightbox').addEventListener('click', (e) => { if (e.target === $('#lightbox') || e.target === $('#lightbox-img')) cerrar($('#lightbox')); });

/* ---------------- Documentos (atleta o entrenador) ---------------- */
// Igual que la foto: todo trabaja sobre la URL base de la entidad, así el mismo
// repositorio de documentos sirve para atletas y entrenadores.
function initDocsUI(base) {
  state.docBase = base;
  state.docSel = new Set();
  $('#doc-open').onclick = () => abrirDocsSeleccionados();
  $('#doc-dl').onclick = () => descargarDocsSeleccionados();
  if ($('#doc-del')) $('#doc-del').onclick = () => eliminarDocsSeleccionados();
  $('#doc-desel').onclick = () => { state.docSel.clear(); renderDocsSel(); };
  cargarDocumentos(base);
}
async function cargarDocumentos(base) {
  let docs;
  try { docs = await api(`${base}/documentos`); } catch (e) { return; }
  state.docsData = docs || [];
  const grid = $('#docs-grid'), empty = $('#docs-empty');
  if (!grid) return;
  empty.classList.toggle('hidden', state.docsData.length > 0);
  grid.innerHTML = state.docsData.map(d => {
    if (!d.existe) {
      // Archivo perdido de /data: aviso claro y solo permitir eliminar el registro.
      return `<div class="doc-card doc-missing" data-id="${d.id}" title="${esc(d.nombre)}">
        <div class="doc-thumb"><div class="doc-fallback doc-notfound">${ICON.alert}<span>No se encontró</span></div></div>
        <div class="doc-name">${esc(d.nombre)}</div>
        ${isAdmin() ? `<button class="btn btn-sm btn-danger doc-del-one" data-id="${d.id}">${ICON.trash}<span>Eliminar</span></button>` : ''}
      </div>`;
    }
    return `<div class="doc-card" data-id="${d.id}" title="${esc(d.nombre)}">
      <input type="checkbox" class="doc-check" data-id="${d.id}">
      <div class="doc-thumb">${thumbDoc(base, d)}</div>
      <div class="doc-name">${esc(d.nombre)}</div>
    </div>`;
  }).join('');
  // Interacción tipo administrador de archivos: un clic selecciona; doble clic
  // abre en pestaña nueva; la casilla marca/desmarca al instante.
  $$('#docs-grid .doc-card:not(.doc-missing)').forEach(card => {
    const did = +card.dataset.id;
    let timer = null;
    card.addEventListener('click', (e) => {
      if (e.target.classList.contains('doc-check')) return; // lo maneja el change
      clearTimeout(timer);
      timer = setTimeout(() => toggleDocSel(did), 200);
    });
    card.addEventListener('dblclick', () => { clearTimeout(timer); abrirDoc(did); });
    card.querySelector('.doc-check').addEventListener('change', (e) => { e.stopPropagation(); toggleDocSel(did, e.target.checked); });
  });
  // Eliminar un documento con archivo perdido.
  $$('#docs-grid .doc-del-one').forEach(btn => btn.addEventListener('click', async (e) => {
    e.stopPropagation();
    const did = +btn.dataset.id;
    if (!await confirmar('¿Eliminar este documento?', { peligro: true, ok: 'Eliminar' })) return;
    try { await api(`${state.docBase}/documentos/${did}`, { method: 'DELETE' }); toast('Documento eliminado'); cargarDocumentos(state.docBase); }
    catch (err) { toast(err.message, 'err'); }
  }));
  renderDocsSel();
}
// thumbDoc: miniatura del documento — imagen directa o primera página del PDF.
function thumbDoc(base, d) {
  const url = `${base}/documentos/${d.id}`;
  if (d.tipo === 'img') return `<img class="doc-img" src="${url}" alt="" loading="lazy">`;
  return `<object data="${url}#page=1&toolbar=0&navpanes=0&scrollbar=0&view=Fit" type="application/pdf"><div class="doc-fallback">${ICON.pdf}<span>PDF</span></div></object>`;
}
function toggleDocSel(id, force) {
  const on = force === undefined ? !state.docSel.has(id) : force;
  on ? state.docSel.add(id) : state.docSel.delete(id);
  renderDocsSel();
}
function renderDocsSel() {
  $$('#docs-grid .doc-card').forEach(card => {
    const did = +card.dataset.id, on = state.docSel.has(did);
    card.classList.toggle('sel', on);
    const c = card.querySelector('.doc-check'); if (c) c.checked = on;
  });
  const n = state.docSel.size;
  const bar = $('#docs-selbar'); if (bar) bar.classList.toggle('hidden', n === 0);
  const sn = $('#docs-seln'); if (sn) sn.textContent = n;
}
function docURL(id, download) { return `${state.docBase}/documentos/${id}${download ? '?download=1' : ''}`; }
function abrirDoc(id) { window.open(docURL(id), '_blank'); }
function abrirDocsSeleccionados() { [...state.docSel].forEach(id => window.open(docURL(id), '_blank')); }
function descargarDocsSeleccionados() {
  const ids = [...state.docSel];
  if (ids.length === 0) return;
  if (ids.length === 1) { descargar(docURL(ids[0], true)); return; }
  descargar(`${state.docBase}/documentos/zip?ids=${ids.join(',')}`);
}
async function eliminarDocsSeleccionados() {
  const ids = [...state.docSel];
  if (ids.length === 0) return;
  if (!await confirmar(`¿Eliminar ${ids.length} documento(s)? Esta acción no se puede deshacer.`, { peligro: true, ok: 'Eliminar' })) return;
  let fail = 0;
  for (const id of ids) { try { await api(`${state.docBase}/documentos/${id}`, { method: 'DELETE' }); } catch { fail++; } }
  if (fail) toast(`${fail} documento(s) no se pudieron eliminar`, 'err'); else toast('Documentos eliminados');
  state.docSel.clear(); cargarDocumentos(state.docBase);
}
// Modal de subida de documento (recibe la URL base de la entidad).
const modalDocumento = $('#modal-documento');
function abrirSubirDocumento(base) {
  $('#form-documento').reset();
  $('#doc-error').textContent = '';
  $('#doc-atleta-id').value = base;
  $('#doc-hint').textContent = `PDF o imagen (JPG, PNG), hasta ${state.maxUploadMB} MB.`;
  abrir(modalDocumento);
}
$('#form-documento').addEventListener('submit', async (e) => {
  e.preventDefault();
  $('#doc-error').textContent = '';
  const base = $('#doc-atleta-id').value;
  const nombre = $('#doc-nombre').value.trim();
  const file = $('#doc-file').files[0];
  if (!nombre) { $('#doc-error').textContent = 'Escriba un nombre para el documento.'; return; }
  if (!file) { $('#doc-error').textContent = 'Seleccione un archivo.'; return; }
  const okTipo = /\.(pdf|jpe?g|png)$/i.test(file.name) || ['application/pdf', 'image/jpeg', 'image/png'].includes(file.type);
  if (!okTipo) { $('#doc-error').textContent = 'Solo se aceptan PDF o imágenes (JPG, PNG).'; return; }
  if (file.size > state.maxUploadMB * 1024 * 1024) { $('#doc-error').textContent = `El documento supera el máximo de ${state.maxUploadMB} MB.`; return; }
  const fd = new FormData(); fd.append('nombre', nombre); fd.append('archivo', file);
  try { await postForm(`${base}/documentos`, fd); toast('Documento subido'); cerrar(modalDocumento); cargarDocumentos(base); }
  catch (err) { $('#doc-error').textContent = err.message; }
});
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
    onEdit: e => abrirEscuela(e), onDelete: e => borrarEscuela(e),
    detailTitle: e => e.nombre, delUrl: e => '/api/escuelas/' + e.id, reload: cargarEscuelas,
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
    onEdit: u => abrirUsuario(u), onDelete: u => borrarUsuario(u),
    detailTitle: u => `${u.nombres} ${u.apellidos}`.trim() || u.username,
    delUrl: u => '/api/usuarios/' + u.id, reload: cargarUsuarios,
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
      { label: 'Cinturón', value: m => m.cinturon_color ? (m.cinturon_color + (m.dan ? ` ${romano(m.dan)} DAN` : '')) : '—', type: 'text', html: m => chipCinturon(m.cinturon_color, m.dan) },
      { label: 'Atletas', value: m => m.num_atletas, type: 'number', filter: false },
      { label: 'Activo', value: m => m.activo ? 'Sí' : 'No', type: 'bool', html: m => m.activo ? '<span class="chip state-activo">Sí</span>' : '<span class="chip state-retirado">No</span>' },
    ],
    onOpen: m => verDetalleEntrenador(m),
    onVer: m => verAtletasDe(m), verLabel: 'Ver sus atletas',
    onEdit: m => abrirEntrenador(m), onDelete: m => borrarEntrenador(m),
    detailTitle: m => `${m.nombres} ${m.apellidos}`.trim(),
    delUrl: m => '/api/entrenadores/' + m.id, reload: cargarEntrenadores,
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
    dtMaestro = DataTable({ mount: '#dt-maestro', columns: maestroColumns(tipo), onEdit: x => abrirMaestro(x), onDelete: x => borrarMaestro(x), detailTitle: x => x.nombre || x.color || 'Detalle', delUrl: x => '/api/maestros/' + state.maestroTipo + '/' + x.id, reload: () => cargarMaestro(state.maestroTipo) });
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
$('#btn-dl-full').addEventListener('click', () => { descargar('/api/backup/full'); setTimeout(cargarUltimoRespaldo, 1500); });
$('#btn-dl-db').addEventListener('click', () => { descargar('/api/backup/db'); setTimeout(cargarUltimoRespaldo, 1500); });
// Último respaldo: muestra "DD/MM/YYYY HH:MM AM/PM" si existe registro.
async function cargarUltimoRespaldo() {
  const el = $('#ultimo-respaldo');
  try {
    const c = await api('/api/config');
    if (c && c.ultimo_respaldo) { el.textContent = 'Último respaldo: ' + fmtFechaHora(c.ultimo_respaldo); el.classList.remove('hidden'); }
    else { el.classList.add('hidden'); }
  } catch { el.classList.add('hidden'); }
}
// fmtFechaHora: ISO → "DD/MM/YYYY HH:MM AM/PM".
function fmtFechaHora(iso) {
  const d = new Date(iso);
  if (isNaN(d)) return iso;
  const p = n => String(n).padStart(2, '0');
  let h = d.getHours(); const ampm = h >= 12 ? 'PM' : 'AM'; h = h % 12 || 12;
  return `${p(d.getDate())}/${p(d.getMonth() + 1)}/${d.getFullYear()} ${p(h)}:${p(d.getMinutes())} ${ampm}`;
}
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
   CONFIGURACIÓN
   ============================================================================ */
async function cargarConfig() {
  $('#cfg-msg').textContent = '';
  try { const c = await api('/api/config'); state.maxUploadMB = c.max_upload_mb || 5; $('#cfg-maxmb').value = state.maxUploadMB; }
  catch (e) { toast(e.message, 'err'); }
}
$('#form-config').addEventListener('submit', async (e) => {
  e.preventDefault();
  $('#cfg-msg').textContent = '';
  const mb = parseInt($('#cfg-maxmb').value, 10);
  if (!(mb >= 1 && mb <= 100)) { $('#cfg-msg').textContent = 'Ingrese un valor entre 1 y 100.'; return; }
  try {
    const c = await api('/api/config', { method: 'PUT', body: JSON.stringify({ max_upload_mb: mb }) });
    state.maxUploadMB = c.max_upload_mb; $('#cfg-maxmb').value = c.max_upload_mb;
    toast('Configuración guardada');
  } catch (err) {
    if (err.fields && err.fields.max_upload_mb) $('#cfg-msg').textContent = err.fields.max_upload_mb;
    else $('#cfg-msg').textContent = err.message;
  }
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

/* ---------------------- Campos con unidad "inteligentes" ---------------------- */
// Solo dígitos; la unidad y (para estatura) los decimales se forman solos,
// construyendo desde la derecha. Rango validado al perder el foco.
const SMART = {
  estatura: { unit: ' m', maxDigits: 3, decimals: 2, min: 0, max: 9.99 },   // 0,00 – 9,99
  peso:     { unit: ' kg', maxDigits: 3, decimals: 0, min: 0, max: 999 },
  imc:      { unit: '', maxDigits: 3, decimals: 1, min: 16, max: 40 },  // p.ej. 24,8
  fc:       { unit: ' ppm', maxDigits: 3, decimals: 0, min: 30, max: 170 },
};
function smartFmt(cfg, digits) {
  if (!digits) return '';
  if (cfg.decimals) {
    const p = digits.padStart(cfg.decimals + 1, '0');
    const intp = String(parseInt(p.slice(0, p.length - cfg.decimals), 10));
    return `${intp},${p.slice(p.length - cfg.decimals)}${cfg.unit}`;
  }
  return `${parseInt(digits, 10)}${cfg.unit}`;
}
function smartRender(el, cfg) {
  el.value = smartFmt(cfg, el.dataset.digits || '');
  const pos = el.value.length - cfg.unit.length;
  try { el.setSelectionRange(pos, pos); } catch { /* ignore */ }
}
function smartUnit(el, key) {
  const cfg = SMART[key]; el._smartCfg = cfg;
  el.setAttribute('inputmode', 'numeric');
  el.addEventListener('keydown', (e) => {
    if (e.key >= '0' && e.key <= '9') {
      e.preventDefault();
      el.dataset.digits = ((el.dataset.digits || '') + e.key).slice(-cfg.maxDigits);
      smartRender(el, cfg);
    } else if (e.key === 'Backspace') {
      e.preventDefault(); el.dataset.digits = (el.dataset.digits || '').slice(0, -1); smartRender(el, cfg);
    } else if (e.key === 'Delete') {
      e.preventDefault(); el.dataset.digits = ''; smartRender(el, cfg);
    } else if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
      e.preventDefault(); // bloquear cualquier otro carácter
    }
  });
  el.addEventListener('blur', () => {
    let d = el.dataset.digits || '';
    if (!d) return;
    let num = cfg.decimals ? parseInt(d, 10) / Math.pow(10, cfg.decimals) : parseInt(d, 10);
    num = Math.min(cfg.max, Math.max(cfg.min, num));
    d = cfg.decimals ? String(Math.round(num * Math.pow(10, cfg.decimals))) : String(num);
    el.dataset.digits = d.slice(-cfg.maxDigits);
    el.value = smartFmt(cfg, el.dataset.digits);
  });
}
function smartSet(el, value) {
  const cfg = el._smartCfg;
  if (!cfg) { el.value = value || ''; return; }
  el.dataset.digits = String(value || '').replace(/\D/g, '').slice(-cfg.maxDigits);
  el.value = smartFmt(cfg, el.dataset.digits);
}
// Sí/No de la planilla médica: bool ⇄ valor del select ('si'|'no'|'').
function boolToSiNo(b) { return b === true ? 'si' : (b === false ? 'no' : ''); }
function siNoToBool(sel) { const v = $(sel).value; return v === 'si' ? true : (v === 'no' ? false : null); }
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
function soloMesAnio(f) { return f && f.length >= 7 ? `${f.slice(5, 7)}/${f.slice(0, 4)}` : (f || '—'); }
// fmtFecha convierte "YYYY-MM-DD" a "DD/MM/YYYY" para mostrar al usuario.
function fmtFecha(f) { return (f && f.length >= 10 && f[4] === '-') ? `${f.slice(8, 10)}/${f.slice(5, 7)}/${f.slice(0, 4)}` : (f || '—'); }
function pad2(n) { return String(n).padStart(2, '0'); }
function kv(k, v) { return `<div><div class="k">${k}</div>${esc(String(v))}</div>`; }
function sexoTexto(s) { return s === 'M' ? 'Masculino' : (s === 'F' ? 'Femenino' : '—'); }
function siNoTexto(b) { return b === true ? 'Sí' : (b === false ? 'No' : '—'); }
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
  limitInput($('#cfg-maxmb'), { allow: /[0-9]/, maxLen: 3 });
  smartUnit($('#a-estatura'), 'estatura');
  smartUnit($('#a-peso'), 'peso');
  smartUnit($('#a-imc'), 'imc');
  smartUnit($('#a-fc'), 'fc');

  const calcIMC = () => {
    const e = $('#a-estatura').dataset.digits;
    const p = $('#a-peso').dataset.digits;
    const imcInput = $('#a-imc');
    if (!e || !p) {
      imcInput.dataset.digits = '';
      imcInput.value = '';
      return;
    }
    const estM = parseInt(e, 10) / 100;
    const pesoK = parseInt(p, 10);
    if (estM <= 0) {
      imcInput.dataset.digits = '';
      imcInput.value = '';
      return;
    }
    const imc = pesoK / (estM * estM);
    let imcStr = String(Math.round(imc * 10));
    // Limitar al maxDigits (3) por seguridad
    if (imcStr.length > 3) imcStr = '999';
    imcInput.dataset.digits = imcStr;
    smartRender(imcInput, SMART.imc);
  };
  $('#a-estatura').addEventListener('blur', calcIMC);
  $('#a-peso').addEventListener('blur', calcIMC);
}
// Acordeones de la sección Ayuda: el título despliega/recoge su contenido.
function initAcordeonesAyuda() {
  $$('#sec-ayuda .help-card').forEach((card, i) => {
    const h3 = card.querySelector('h3');
    if (!h3) return;
    card.classList.add('acc');
    const content = document.createElement('div');
    content.className = 'acc-content';
    let n = h3.nextSibling;
    while (n) { const next = n.nextSibling; content.appendChild(n); n = next; }
    card.appendChild(content);
    h3.classList.add('acc-header');
    h3.insertAdjacentHTML('beforeend', `<span class="acc-chevron">${ICON.chevron}</span>`);
    h3.addEventListener('click', () => {
      const open = card.classList.toggle('open');
      content.style.maxHeight = open ? content.scrollHeight + 'px' : '';
    });
    if (i === 0) { card.classList.add('open'); content.style.maxHeight = content.scrollHeight + 'px'; }
  });
}

initLimiters();
bindFiltros();
bindPager();
bindSeleccion();
initRespaldo();
initAcordeonesAyuda();
iniciar();
