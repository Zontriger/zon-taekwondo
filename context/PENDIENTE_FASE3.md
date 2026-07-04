# SIGAT — Trabajo pendiente (Fase 3)

Estado: **Inc. 1 y 2 completados y commiteados**. Faltan **Inc. 3 y 4**.
Plan aprobado: `~/.claude/plans/magical-leaping-moon.md`. Login: `admin` / `admin123`.

## Ya hecho (referencia)
- **Inc. 1** (`13cb280`): caja de ayuda con motivos del botón deshabilitado; retorno a la
  ficha tras editar/cambiar cinturón/retirar/reactivar; DAN oculto en "Cambiar cinturón"
  salvo negro; reporte PDF en **carta vertical**; `.gitignore` con `data/`.
- **Inc. 2** (`4c9e867`): entidad **Maestro** (tabla `maestro`) + sección **Entrenadores**
  (CRUD, acción "Ver atletas"); atleta con `maestro_id` y `tipo_sangre` (en ficha y filtros);
  **ficha técnica PDF** (`GET /api/atletas/{id}/ficha.pdf`, carta vertical).

---

## Incremento 3 — Fotos, documentos y Configuración (PENDIENTE, el más grande)

### Decisiones ya tomadas
- Archivos en **carpeta `data/`** en disco (junto a `app.db`), ruta por atleta.
- Preview de PDF con **PDF.js vendorizado localmente** (sin CDN).
- El respaldo deja de ser "un solo archivo": hay que respaldar `app.db` **+** `data/`.

### 3.1 Almacenamiento y arranque
- `main.go`: crear carpeta `data/` al iniciar (junto a `app.db`).
- Rutas físicas: foto → `data/atleta/{id}/foto.<ext>`; documentos → `data/atleta/{id}/doc_<docid>.pdf`.
- `atleta.foto_path` (columna ya existe) guarda la ruta relativa de la foto.
- Nueva tabla `documento(id, atleta_id, nombre, archivo, mime, tamano, creado_en)` + migración
  idempotente (patrón `columnExists`/`CREATE TABLE IF NOT EXISTS` en `db.go`).

### 3.2 Configuración (sección nueva, solo admin)
- Tabla `config` clave-valor; clave `max_upload_mb` (default **5**).
- `GET/PUT /api/config` (admin). Sección "Configuración" en el sidebar (icono engranaje),
  con un campo numérico para el límite de MB. Se lee al validar cada subida.

### 3.3 Subida y validación (multipart/form-data)
- Foto: solo `jpg/jpeg/png`. Documento: solo `pdf`. Tamaño ≤ `max_upload_mb`.
- Validar **extensión + MIME + tamaño** en backend (`handlers_archivo.go` nuevo).
- Descarga restringida: datos sensibles de menores → **solo admin** (regla #5 de `MODELO_DATOS.md`).
- Endpoints:
  - Foto: `GET/POST/DELETE /api/atletas/{id}/foto` (reemplazable → "renovar").
  - Documentos: `GET /api/atletas/{id}/documentos` (lista); `POST` (subir, **nombre obligatorio**);
    `GET .../documentos/{docid}` (abrir/descargar); `DELETE .../documentos/{docid}`;
    `POST .../documentos/zip` (varios → `{{nombre_atleta}}_documentos.zip`, con `archive/zip` de stdlib).
- La ficha técnica PDF ya intenta incrustar la foto si el archivo existe (`os.Stat` en
  `handlers_reporte.go`); al implementar la foto queda funcionando sin cambios.

### 3.4 UI en la ficha del atleta
- **Foto** arriba con botón "Actualizar foto" (input file, valida tipo/tamaño, muestra preview).
- Bloque **"Documentos"** en cuadrícula (grid de tarjetas):
  - Cada tarjeta: **miniatura** (PDF.js renderiza la 1ª página en un `<canvas>`) + nombre.
  - **1 click** selecciona → aparecen botones **Abrir / Descargar / Eliminar**.
  - **Doble click** abre en pestaña nueva (visor nativo del navegador).
  - **Multi-selección** → abrir / descargar (zip) / eliminar en lote.
  - Subir exige asignar un **nombre** al documento.
- Vendorizar PDF.js: `curl` de `pdf.min.js` + `pdf.worker.min.js` a `src/frontend/vendor/`
  (se embeben con el resto del frontend). Configurar `GlobalWorkerOptions.workerSrc`.

### 3.5 Respaldo (ajuste)
- Actualizar la sección Respaldo: el botón "Descargar base de datos" debería pasar a
  "Descargar respaldo" = **zip con `app.db` + carpeta `data/`** (usar `archive/zip`).
- Documentar que restaurar implica reponer también `data/`.

**Archivos**: nuevos `src/backend/handlers_archivo.go`, `src/database/documento.go`,
`src/frontend/vendor/pdf*.js`; modificar `main.go`, `server.go`, `handlers_backup.go`,
`schema.sql`, `db.go`, `frontend/index.html`, `js/app.js`, `css/style.css`.

---

## Incremento 4 — Selección de filas y acciones masivas (PENDIENTE)

- **Columna de selección** en el componente `DataTable` (`js/app.js`) y en la tabla de atletas:
  - checkbox por fila; **shift-click** selecciona un rango;
  - checkbox de cabecera con **3 estados**: marcar visibles → marcar todos → desmarcar.
- **Barra de acciones masivas** que aparece con la selección:
  - **Generar PDF** de los seleccionados y **Eliminar** seleccionados (admin).
- Backend:
  - El reporte (`handlers_reporte.go`) acepta una lista `ids` (además de q + domain).
  - `POST /api/atletas/eliminar` para borrado en lote (admin).
  - En atletas, "marcar todos" = todos los que cumplen el filtro (no solo la página visible).

**Archivos**: `js/app.js`, `css/style.css`, `handlers_reporte.go`, `handlers_atleta.go`.

---

## Notas para retomar
- Fecha objetivo del cliente: **22/jul** (regalo de cumpleaños de los atletas).
- Convención de commits: `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`.
- En este entorno, `git` funciona solo por **PowerShell** (la tool Bash no ve el `.git`);
  los mensajes multilínea se pasan con `git commit -F archivo.txt`.
- Verificar cada incremento con el preview (`preview_start`, `admin/admin123`) y **dejar la BD limpia**.
