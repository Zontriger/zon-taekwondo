// Package database gestiona la persistencia en SQLite (driver CGO-free).
//
// El esquema es la fuente de verdad: se embebe schema.sql (copia de la raíz del
// proyecto) y se aplica solo cuando la base de datos está vacía, de modo que el
// SEED se ejecute una única vez. En cada arranque se corren, además, migraciones
// idempotentes para bases ya existentes.
package database

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite" // driver 100% Go puro, sin CGO
)

//go:embed schema.sql
var schemaSQL string

//go:embed geo_seed.sql
var geoSeedSQL string

// Credenciales del administrador inicial creado en la primera ejecución.
const (
	defaultAdminUser = "admin"
	defaultAdminPass = "admin123"
)

// Init abre (o crea) la base de datos en filepath, aplica el esquema si hace
// falta y garantiza que exista al menos un entrenador administrador.
func Init(filepath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", filepath)
	if err != nil {
		return nil, fmt.Errorf("abrir sqlite: %w", err)
	}

	// Claves foráneas y espera ante bloqueos concurrentes (WS + REST).
	if _, err := db.Exec("PRAGMA foreign_keys = ON; PRAGMA busy_timeout = 5000;"); err != nil {
		return nil, fmt.Errorf("pragmas: %w", err)
	}

	fresh, err := isFresh(db)
	if err != nil {
		return nil, err
	}
	if fresh {
		log.Println("[db] base de datos nueva: aplicando esquema y seed…")
		if _, err := db.Exec(schemaSQL); err != nil {
			return nil, fmt.Errorf("aplicar esquema: %w", err)
		}
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migraciones: %w", err)
	}
	if err := ensureAdmin(db); err != nil {
		return nil, fmt.Errorf("crear admin inicial: %w", err)
	}
	return db, nil
}

// isFresh indica si la base no tiene todavía la tabla principal 'atleta'.
func isFresh(db *sql.DB) (bool, error) {
	var n int
	err := db.QueryRow(
		"SELECT count(*) FROM sqlite_master WHERE type='table' AND name='atleta'",
	).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("inspeccionar sqlite_master: %w", err)
	}
	return n == 0, nil
}

// migrate aplica cambios de esquema idempotentes para bases ya existentes.
func migrate(db *sql.DB) error {
	if !columnExists(db, "entrenador", "es_admin") {
		if _, err := db.Exec(
			`ALTER TABLE entrenador ADD COLUMN es_admin INTEGER NOT NULL DEFAULT 0`,
		); err != nil {
			return err
		}
		log.Println("[db] migración: columna entrenador.es_admin agregada")
	}

	// Cédula compuesta (tipo + número) en atleta y representante.
	if !columnExists(db, "atleta", "cedula_tipo") {
		for _, stmt := range []string{
			`ALTER TABLE atleta ADD COLUMN cedula_tipo TEXT`,
			`ALTER TABLE atleta ADD COLUMN cedula_numero TEXT`,
		} {
			if _, err := db.Exec(stmt); err != nil {
				return err
			}
		}
		// Migrar el valor antiguo 'cedula' (formato "V-123") si existía.
		if columnExists(db, "atleta", "cedula") {
			db.Exec(`UPDATE atleta SET
				cedula_tipo   = CASE WHEN instr(cedula,'-')>0 THEN substr(cedula,1,instr(cedula,'-')-1) ELSE NULL END,
				cedula_numero = CASE WHEN instr(cedula,'-')>0 THEN substr(cedula,instr(cedula,'-')+1)   ELSE cedula END
				WHERE cedula IS NOT NULL`)
		}
		log.Println("[db] migración: atleta.cedula_tipo/cedula_numero agregadas")
	}
	if !columnExists(db, "representante", "cedula_tipo") {
		for _, stmt := range []string{
			`ALTER TABLE representante ADD COLUMN cedula_tipo TEXT`,
			`ALTER TABLE representante ADD COLUMN cedula_numero TEXT`,
		} {
			if _, err := db.Exec(stmt); err != nil {
				return err
			}
		}
	}

	// Tabla de teléfonos de contacto (0..3) e índices nuevos.
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS atleta_telefono_contacto (
			id        INTEGER PRIMARY KEY,
			atleta_id INTEGER NOT NULL REFERENCES atleta(id) ON DELETE CASCADE,
			numero    TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_tel_contacto ON atleta_telefono_contacto (atleta_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_atleta_cedula ON atleta (cedula_tipo, cedula_numero)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	if err := migrarGeografia(db); err != nil {
		return err
	}
	if err := migrarEntrenadores(db); err != nil {
		return err
	}
	if err := migrarArchivos(db); err != nil {
		return err
	}
	if err := migrarPlanilla(db); err != nil {
		return err
	}
	return nil
}

// migrarPlanilla agrega las columnas del formato oficial "Planilla del Atleta"
// (Club Taekwondo Elite Oro Carrizal): datos físicos, estudios, información
// médica y datos laborales del representante. Idempotente.
func migrarPlanilla(db *sql.DB) error {
	cols := []struct{ tabla, col, tipo string }{
		{"atleta", "horario", "TEXT"},
		{"atleta", "sexo", "TEXT"}, // 'M' | 'F'
		{"atleta", "email", "TEXT"},
		{"atleta", "estatura", "TEXT"},
		{"atleta", "peso", "TEXT"},
		{"atleta", "imc", "TEXT"},
		{"atleta", "fc", "TEXT"},
		{"atleta", "talla_camisa", "TEXT"},
		{"atleta", "talla_pantalon", "TEXT"},
		{"atleta", "instituto", "TEXT"},
		{"atleta", "instituto_direccion", "TEXT"},
		{"atleta", "med_enfermedad", "INTEGER"},
		{"atleta", "med_enfermedad_detalle", "TEXT"},
		{"atleta", "med_alergia", "INTEGER"},
		{"atleta", "med_alergia_detalle", "TEXT"},
		{"atleta", "med_operado", "INTEGER"},
		{"atleta", "med_operado_detalle", "TEXT"},
		{"atleta", "med_emergencia", "TEXT"},
		{"representante", "lugar_trabajo", "TEXT"},
		{"representante", "direccion_trabajo", "TEXT"},
		{"representante", "email", "TEXT"},
	}
	for _, c := range cols {
		if !columnExists(db, c.tabla, c.col) {
			if _, err := db.Exec("ALTER TABLE " + c.tabla + " ADD COLUMN " + c.col + " " + c.tipo); err != nil {
				return err
			}
		}
	}
	return nil
}

// migrarArchivos crea las tablas de soporte para archivos subidos (config del
// sistema y documentos por atleta). Idempotente.
func migrarArchivos(db *sql.DB) error {
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS config (
			clave TEXT PRIMARY KEY,
			valor TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS documento (
			id         INTEGER PRIMARY KEY,
			atleta_id  INTEGER NOT NULL REFERENCES atleta(id) ON DELETE CASCADE,
			nombre     TEXT NOT NULL,
			archivo    TEXT NOT NULL,
			creado_en  TEXT NOT NULL DEFAULT (datetime('now')))`,
		`CREATE INDEX IF NOT EXISTS idx_documento_atleta ON documento (atleta_id)`,
		`INSERT OR IGNORE INTO config (clave, valor) VALUES ('max_upload_mb', '5')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// migrarEntrenadores crea la tabla 'maestro' (entrenadores deportivos) y agrega
// a 'atleta' las columnas maestro_id y tipo_sangre (idempotente).
func migrarEntrenadores(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS maestro (
		id INTEGER PRIMARY KEY, nombres TEXT NOT NULL, apellidos TEXT NOT NULL,
		cedula_tipo TEXT, cedula_numero TEXT, telefono TEXT,
		escuela_id INTEGER REFERENCES escuela(id), cinturon_id INTEGER REFERENCES cinturon(id),
		dan INTEGER, activo INTEGER NOT NULL DEFAULT 1,
		creado_en TEXT NOT NULL DEFAULT (datetime('now')))`); err != nil {
		return err
	}
	if !columnExists(db, "atleta", "maestro_id") {
		if _, err := db.Exec(`ALTER TABLE atleta ADD COLUMN maestro_id INTEGER REFERENCES maestro(id)`); err != nil {
			return err
		}
	}
	if !columnExists(db, "atleta", "tipo_sangre") {
		if _, err := db.Exec(`ALTER TABLE atleta ADD COLUMN tipo_sangre TEXT`); err != nil {
			return err
		}
	}
	return nil
}

// migrarGeografia introduce la jerarquía estado→ciudad→municipio→parroquia
// (v2) y siembra los datos de Venezuela desde geo_seed.sql.
func migrarGeografia(db *sql.DB) error {
	// Tablas nuevas (idempotente).
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS ciudad (
			id INTEGER PRIMARY KEY, estado_id INTEGER NOT NULL REFERENCES estado(id),
			nombre TEXT NOT NULL, UNIQUE(estado_id, nombre))`,
		`CREATE TABLE IF NOT EXISTS parroquia (
			id INTEGER PRIMARY KEY, estado_id INTEGER NOT NULL REFERENCES estado(id),
			municipio_id INTEGER NOT NULL REFERENCES municipio(id),
			nombre TEXT NOT NULL, UNIQUE(municipio_id, nombre))`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	// Columnas de jerarquía (antes de indexar municipio.ciudad_id).
	if !columnExists(db, "municipio", "ciudad_id") {
		if _, err := db.Exec(`ALTER TABLE municipio ADD COLUMN ciudad_id INTEGER REFERENCES ciudad(id)`); err != nil {
			return err
		}
	}
	for _, col := range []string{"estado_id", "ciudad_id", "parroquia_id"} {
		if !columnExists(db, "atleta", col) {
			if _, err := db.Exec(`ALTER TABLE atleta ADD COLUMN ` + col + ` INTEGER`); err != nil {
				return err
			}
		}
	}
	// Índices (ya existen las columnas).
	for _, stmt := range []string{
		`CREATE INDEX IF NOT EXISTS idx_ciudad_estado ON ciudad (estado_id)`,
		`CREATE INDEX IF NOT EXISTS idx_municipio_ciudad ON municipio (ciudad_id)`,
		`CREATE INDEX IF NOT EXISTS idx_parroquia_mun ON parroquia (municipio_id)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	// Sembrar la geografía una sola vez (cuando 'ciudad' está vacía).
	var nCiudad int
	if err := db.QueryRow(`SELECT count(*) FROM ciudad`).Scan(&nCiudad); err != nil {
		return err
	}
	if nCiudad > 0 {
		return nil
	}
	// Reset de la geografía v1 (estado/municipio con ids antiguos) para evitar
	// choques de id con el nuevo seed. Solo es seguro si nada la referencia aún.
	var nEsc int
	db.QueryRow(`SELECT count(*) FROM escuela`).Scan(&nEsc)
	if nEsc == 0 {
		db.Exec(`UPDATE atleta SET estado_id=NULL, ciudad_id=NULL, municipio_id=NULL, parroquia_id=NULL`)
		for _, t := range []string{"parroquia", "municipio", "ciudad", "estado"} {
			if _, err := db.Exec(`DELETE FROM ` + t); err != nil {
				return err
			}
		}
	} else {
		log.Println("[db] geo: existen escuelas; se cargan datos geográficos sin resetear (INSERT OR IGNORE)")
	}
	if _, err := db.Exec(geoSeedSQL); err != nil {
		return fmt.Errorf("seed geográfico: %w", err)
	}
	log.Println("[db] geografía de Venezuela sembrada (estados/ciudades/municipios/parroquias)")
	return nil
}

func columnExists(db *sql.DB, table, column string) bool {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid, notnull, pk       int
			name, ctype            string
			dfltValue              sql.NullString
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

// ensureAdmin crea el administrador por defecto si no existe ningún entrenador.
func ensureAdmin(db *sql.DB) error {
	var n int
	if err := db.QueryRow("SELECT count(*) FROM entrenador").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(defaultAdminPass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`INSERT INTO entrenador (username, password_hash, nombres, apellidos, es_admin, estado)
		 VALUES (?, ?, 'Administrador', 'del Sistema', 1, 'activo')`,
		defaultAdminUser, string(hash),
	)
	if err != nil {
		return err
	}
	log.Printf("[db] administrador inicial creado → usuario: %q  contraseña: %q (cámbiela)",
		defaultAdminUser, defaultAdminPass)
	return nil
}
