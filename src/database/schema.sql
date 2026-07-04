-- ============================================================================
-- Sistema de Gestión de Atletas — Asociación de Taekwondo, Edo. Miranda
-- Esquema de base de datos (SQLite)
--
-- Este archivo es la FUENTE DE VERDAD del modelo de datos.
-- Léase junto con MODELO_DATOS.md, que documenta las reglas de negocio.
-- ============================================================================

PRAGMA foreign_keys = ON;

-- ----------------------------------------------------------------------------
-- 1. GEOGRAFÍA (reutilizada por escuelas y por la dirección del atleta)
-- ----------------------------------------------------------------------------

CREATE TABLE estado (
    id      INTEGER PRIMARY KEY,
    nombre  TEXT NOT NULL UNIQUE
);

-- Jerarquía geográfica: estado -> ciudad -> municipio -> parroquia.
-- Cada nivel guarda la ascendencia denormalizada (estado_id, ...) para que el
-- filtrado en cascada del frontend sea directo y robusto.
--   'Ciudad' es un nivel intermedio OPCIONAL (Venezuela no lo tiene como división
--   oficial; se deriva de la capital municipal y es editable en Datos maestros).

CREATE TABLE ciudad (
    id         INTEGER PRIMARY KEY,
    estado_id  INTEGER NOT NULL REFERENCES estado(id),
    nombre     TEXT NOT NULL,
    UNIQUE (estado_id, nombre)
);

CREATE TABLE municipio (
    id         INTEGER PRIMARY KEY,
    estado_id  INTEGER NOT NULL REFERENCES estado(id),
    ciudad_id  INTEGER REFERENCES ciudad(id),
    nombre     TEXT NOT NULL,
    UNIQUE (estado_id, nombre)
);

CREATE TABLE parroquia (
    id            INTEGER PRIMARY KEY,
    estado_id     INTEGER NOT NULL REFERENCES estado(id),
    municipio_id  INTEGER NOT NULL REFERENCES municipio(id),
    nombre        TEXT NOT NULL,
    UNIQUE (municipio_id, nombre)
);

-- ----------------------------------------------------------------------------
-- 2. CATÁLOGO DE CINTURONES
--    'orden' define la progresión (menor = más principiante).
--    'es_negro' marca el único color que habilita DAN (1-9).
-- ----------------------------------------------------------------------------

CREATE TABLE cinturon (
    id        INTEGER PRIMARY KEY,
    color     TEXT NOT NULL UNIQUE,
    orden     INTEGER NOT NULL,
    es_negro  INTEGER NOT NULL DEFAULT 0 CHECK (es_negro IN (0,1))
);

-- ----------------------------------------------------------------------------
-- 3. ESCUELAS  (una escuela pertenece a UN solo municipio)
-- ----------------------------------------------------------------------------

CREATE TABLE escuela (
    id            INTEGER PRIMARY KEY,
    nombre        TEXT NOT NULL,
    municipio_id  INTEGER NOT NULL REFERENCES municipio(id),
    direccion     TEXT,
    activa        INTEGER NOT NULL DEFAULT 1 CHECK (activa IN (0,1))
);

-- ----------------------------------------------------------------------------
-- 4. ENTRENADOR  (único ROL de usuario del sistema; puede haber varios)
--    Un entrenador pertenece a una escuela (uno-a-muchos).
--    'dan' solo debe llenarse si su cinturón es negro (regla a nivel app).
-- ----------------------------------------------------------------------------

CREATE TABLE entrenador (
    id             INTEGER PRIMARY KEY,
    escuela_id     INTEGER REFERENCES escuela(id),
    username       TEXT NOT NULL UNIQUE,
    password_hash  TEXT NOT NULL,
    nombres        TEXT NOT NULL,
    apellidos      TEXT NOT NULL,
    cinturon_id    INTEGER REFERENCES cinturon(id),
    dan            INTEGER CHECK (dan BETWEEN 1 AND 9),
    telefono       TEXT,
    correo         TEXT,
    es_admin       INTEGER NOT NULL DEFAULT 0 CHECK (es_admin IN (0,1)),  -- acceso a backups y datos sensibles de menores
    estado         TEXT NOT NULL DEFAULT 'activo' CHECK (estado IN ('activo','retirado')),
    fecha_registro TEXT NOT NULL DEFAULT (date('now'))
);

-- ----------------------------------------------------------------------------
-- 5. ATLETA
--    - La cédula = TIPO (V/E/P) + NÚMERO; la combinación es única (idx_atleta_cedula).
--      Ambos NULL-ables: un menor puede no tener cédula.
--    - telefono = teléfono PRINCIPAL (requerido a nivel app). Los de contacto
--      (0..3) viven en atleta_telefono_contacto.
--    - municipio_id + direccion_detalle componen la dirección (estado→habitación).
--    - escuela_id = escuela ACTUAL.
--    - fecha_inscripcion guarda día=01 cuando el día es desconocido;
--      inscripcion_dia_exacto indica si el día es real (1) o placeholder (0).
-- ----------------------------------------------------------------------------

CREATE TABLE atleta (
    id                     INTEGER PRIMARY KEY,
    foto_path              TEXT,
    nombres                TEXT NOT NULL,
    apellidos              TEXT NOT NULL,
    cedula_tipo            TEXT CHECK (cedula_tipo IN ('V','E','P')),  -- V, E o P
    cedula_numero          TEXT,
    fecha_nacimiento       TEXT NOT NULL,                 -- YYYY-MM-DD
    telefono               TEXT,                          -- PRINCIPAL (requerido a nivel app)
    -- Ubicación jerárquica (todos opcionales; se rellena el nivel más profundo elegido
    -- y se normalizan los ancestros).
    estado_id              INTEGER REFERENCES estado(id),
    ciudad_id              INTEGER REFERENCES ciudad(id),
    municipio_id           INTEGER REFERENCES municipio(id),
    parroquia_id           INTEGER REFERENCES parroquia(id),
    direccion_detalle      TEXT,                          -- sector, calle, casa/apto, habitación
    escuela_id             INTEGER REFERENCES escuela(id),
    fecha_inscripcion      TEXT NOT NULL,                 -- YYYY-MM-DD
    inscripcion_dia_exacto INTEGER NOT NULL DEFAULT 1 CHECK (inscripcion_dia_exacto IN (0,1)),
    creado_en              TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Teléfonos de contacto adicionales del atleta (0 a 3; máximo validado en app).
CREATE TABLE atleta_telefono_contacto (
    id         INTEGER PRIMARY KEY,
    atleta_id  INTEGER NOT NULL REFERENCES atleta(id) ON DELETE CASCADE,
    numero     TEXT NOT NULL
);

-- ----------------------------------------------------------------------------
-- 6. REPRESENTANTE  (1:1 opcional; solo aplica a menores)
--    Se conserva como histórico aunque el atleta cumpla 18 años.
--    Cédula tipo+número (sin unicidad).
-- ----------------------------------------------------------------------------

CREATE TABLE representante (
    atleta_id     INTEGER PRIMARY KEY REFERENCES atleta(id) ON DELETE CASCADE,
    cedula_tipo   TEXT CHECK (cedula_tipo IN ('V','E','P')),
    cedula_numero TEXT,
    nombres       TEXT,
    apellidos     TEXT,
    telefono      TEXT,
    parentesco    TEXT
);

-- ----------------------------------------------------------------------------
-- 7. HISTORIAL DE CINTURÓN
--    El "cinturón actual" del atleta = fila con fecha_cambio más reciente.
--    'dan' obligatorio si el cinturón es negro; NULL en caso contrario (app).
-- ----------------------------------------------------------------------------

CREATE TABLE historial_cinturon (
    id              INTEGER PRIMARY KEY,
    atleta_id       INTEGER NOT NULL REFERENCES atleta(id) ON DELETE CASCADE,
    cinturon_id     INTEGER NOT NULL REFERENCES cinturon(id),
    dan             INTEGER CHECK (dan BETWEEN 1 AND 9),
    fecha_cambio    TEXT NOT NULL,                        -- YYYY-MM-DD
    registrado_por  INTEGER REFERENCES entrenador(id)
);

-- ----------------------------------------------------------------------------
-- 8. PERIODO DE ACTIVIDAD  (soporta retiro + reactivación sin perder antigüedad)
--    Estado actual = ¿existe un periodo con fecha_fin NULL? activo : retirado.
--    Fecha de retiro = fecha_fin del último periodo cerrado.
--    Reactivar = insertar un nuevo periodo con fecha_fin NULL.
-- ----------------------------------------------------------------------------

CREATE TABLE periodo_actividad (
    id             INTEGER PRIMARY KEY,
    atleta_id      INTEGER NOT NULL REFERENCES atleta(id) ON DELETE CASCADE,
    fecha_inicio   TEXT NOT NULL,                         -- YYYY-MM-DD
    fecha_fin      TEXT,                                  -- NULL = actualmente activo
    motivo_retiro  TEXT
);

-- ----------------------------------------------------------------------------
-- 9. AUDITORÍA  (quién cambió qué y cuándo)
-- ----------------------------------------------------------------------------

CREATE TABLE auditoria (
    id             INTEGER PRIMARY KEY,
    entrenador_id  INTEGER REFERENCES entrenador(id),
    accion         TEXT NOT NULL,                         -- INSERT / UPDATE / DELETE
    tabla          TEXT NOT NULL,
    registro_id    INTEGER,
    detalle        TEXT,                                  -- JSON con los cambios
    fecha_hora     TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ----------------------------------------------------------------------------
-- ÍNDICES  (búsqueda por facetas + paginación eficiente)
-- ----------------------------------------------------------------------------

CREATE INDEX        idx_atleta_nombre    ON atleta (apellidos, nombres);
CREATE INDEX        idx_atleta_escuela   ON atleta (escuela_id);
CREATE INDEX        idx_atleta_municipio ON atleta (municipio_id);
CREATE UNIQUE INDEX idx_atleta_cedula    ON atleta (cedula_tipo, cedula_numero);
CREATE INDEX        idx_tel_contacto     ON atleta_telefono_contacto (atleta_id);
CREATE INDEX        idx_ciudad_estado    ON ciudad (estado_id);
CREATE INDEX        idx_municipio_ciudad ON municipio (ciudad_id);
CREATE INDEX        idx_parroquia_mun    ON parroquia (municipio_id);
CREATE INDEX idx_hist_cint_atleta    ON historial_cinturon (atleta_id, fecha_cambio);
CREATE INDEX idx_periodo_atleta      ON periodo_actividad (atleta_id, fecha_fin);
CREATE INDEX idx_auditoria_fecha     ON auditoria (fecha_hora);

-- ============================================================================
-- SEED / DATOS INICIALES
-- ============================================================================

-- La geografía (estados, ciudades, municipios, parroquias de Venezuela) se
-- carga desde geo_seed.sql (embebido) en la inicialización/migración.

-- Progresión de cinturones (conjunto común; el orden/colores son editables)
INSERT INTO cinturon (color, orden, es_negro) VALUES
    ('Blanco',   1, 0),
    ('Amarillo', 2, 0),
    ('Naranja',  3, 0),
    ('Verde',    4, 0),
    ('Azul',     5, 0),
    ('Rojo',     6, 0),
    ('Negro',    7, 1);
