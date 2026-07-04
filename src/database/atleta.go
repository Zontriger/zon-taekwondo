package database

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// AtletaInput son los datos que llegan del formulario para crear/editar.
type AtletaInput struct {
	FotoPath             *string        `json:"foto_path"`
	Nombres              string         `json:"nombres"`
	Apellidos            string         `json:"apellidos"`
	CedulaTipo           *string        `json:"cedula_tipo"`
	CedulaNumero         *string        `json:"cedula_numero"`
	FechaNacimiento      string         `json:"fecha_nacimiento"`
	Telefono             *string        `json:"telefono"` // principal
	TelefonosContacto    []string       `json:"telefonos_contacto"`
	EstadoID             *int64         `json:"estado_id"`
	CiudadID             *int64         `json:"ciudad_id"`
	MunicipioID          *int64         `json:"municipio_id"`
	ParroquiaID          *int64         `json:"parroquia_id"`
	DireccionDetalle     *string        `json:"direccion_detalle"`
	EscuelaID            *int64         `json:"escuela_id"`
	FechaInscripcion     string         `json:"fecha_inscripcion"`
	InscripcionDiaExacto bool           `json:"inscripcion_dia_exacto"`
	Representante        *Representante `json:"representante"`
	// Cinturón inicial (solo en creación).
	CinturonID *int64 `json:"cinturon_id"`
	Dan        *int   `json:"dan"`
}

// CreateAtleta inserta el atleta y sus registros iniciales de forma atómica:
// representante (si aplica), primer cinturón (si se indica) y el primer
// periodo de actividad, cuyo inicio coincide con la fecha de inscripción.
func CreateAtleta(db *sql.DB, in AtletaInput, entrenadorID int64) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		INSERT INTO atleta
			(foto_path, nombres, apellidos, cedula_tipo, cedula_numero, fecha_nacimiento,
			 telefono, estado_id, ciudad_id, municipio_id, parroquia_id, direccion_detalle, escuela_id,
			 fecha_inscripcion, inscripcion_dia_exacto)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		in.FotoPath, in.Nombres, in.Apellidos, in.CedulaTipo, in.CedulaNumero, in.FechaNacimiento,
		in.Telefono, in.EstadoID, in.CiudadID, in.MunicipioID, in.ParroquiaID, in.DireccionDetalle,
		in.EscuelaID, in.FechaInscripcion, boolToInt(in.InscripcionDiaExacto))
	if err != nil {
		return 0, mapErr(err)
	}
	id, _ := res.LastInsertId()

	if err := reemplazarTelefonosTx(tx, id, in.TelefonosContacto); err != nil {
		return 0, err
	}
	if in.Representante != nil {
		if err := upsertRepresentanteTx(tx, id, in.Representante); err != nil {
			return 0, err
		}
	}

	if in.CinturonID != nil {
		_, err = tx.Exec(`
			INSERT INTO historial_cinturon (atleta_id, cinturon_id, dan, fecha_cambio, registrado_por)
			VALUES (?,?,?,?,?)`,
			id, *in.CinturonID, in.Dan, in.FechaInscripcion, nullID(entrenadorID))
		if err != nil {
			return 0, err
		}
	}

	// Regla #4: el primer periodo inicia en la fecha de inscripción.
	if _, err = tx.Exec(
		`INSERT INTO periodo_actividad (atleta_id, fecha_inicio) VALUES (?, ?)`,
		id, in.FechaInscripcion,
	); err != nil {
		return 0, err
	}

	Auditar(tx, entrenadorID, "INSERT", "atleta", id,
		map[string]string{"nombres": in.Nombres, "apellidos": in.Apellidos})

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// UpdateAtleta actualiza los datos personales y el representante. El cinturón y
// los periodos se gestionan con sus propias operaciones.
func UpdateAtleta(db *sql.DB, id int64, in AtletaInput, entrenadorID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		UPDATE atleta SET
			foto_path=?, nombres=?, apellidos=?, cedula_tipo=?, cedula_numero=?, fecha_nacimiento=?,
			telefono=?, estado_id=?, ciudad_id=?, municipio_id=?, parroquia_id=?, direccion_detalle=?,
			escuela_id=?, fecha_inscripcion=?, inscripcion_dia_exacto=?
		WHERE id=?`,
		in.FotoPath, in.Nombres, in.Apellidos, in.CedulaTipo, in.CedulaNumero, in.FechaNacimiento,
		in.Telefono, in.EstadoID, in.CiudadID, in.MunicipioID, in.ParroquiaID, in.DireccionDetalle,
		in.EscuelaID, in.FechaInscripcion, boolToInt(in.InscripcionDiaExacto), id)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}

	if err := reemplazarTelefonosTx(tx, id, in.TelefonosContacto); err != nil {
		return err
	}
	if in.Representante != nil {
		if err := upsertRepresentanteTx(tx, id, in.Representante); err != nil {
			return err
		}
	}

	Auditar(tx, entrenadorID, "UPDATE", "atleta", id, nil)
	return tx.Commit()
}

func DeleteAtleta(db *sql.DB, id int64, entrenadorID int64) error {
	res, err := db.Exec(`DELETE FROM atleta WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	Auditar(db, entrenadorID, "DELETE", "atleta", id, nil)
	return nil
}

// GetAtleta devuelve la ficha completa con representante, historial de
// cinturones (más reciente primero) y periodos de actividad.
func GetAtleta(db *sql.DB, id int64) (*Atleta, error) {
	a := &Atleta{}
	var diaExacto int
	err := db.QueryRow(`
		SELECT a.id, a.foto_path, a.nombres, a.apellidos, a.cedula_tipo, a.cedula_numero,
		       a.fecha_nacimiento, a.telefono,
		       a.estado_id, a.ciudad_id, a.municipio_id, a.parroquia_id, a.direccion_detalle,
		       a.escuela_id, a.fecha_inscripcion, a.inscripcion_dia_exacto,
		       e.nombre, es.nombre, ci.nombre, m.nombre, p.nombre
		  FROM atleta a
		  LEFT JOIN escuela e   ON e.id = a.escuela_id
		  LEFT JOIN estado es   ON es.id = a.estado_id
		  LEFT JOIN ciudad ci   ON ci.id = a.ciudad_id
		  LEFT JOIN municipio m ON m.id = a.municipio_id
		  LEFT JOIN parroquia p ON p.id = a.parroquia_id
		 WHERE a.id = ?`, id).Scan(
		&a.ID, &a.FotoPath, &a.Nombres, &a.Apellidos, &a.CedulaTipo, &a.CedulaNumero,
		&a.FechaNacimiento, &a.Telefono,
		&a.EstadoID, &a.CiudadID, &a.MunicipioID, &a.ParroquiaID, &a.DireccionDetalle,
		&a.EscuelaID, &a.FechaInscripcion, &diaExacto,
		&nullStr{&a.EscuelaNombre}, &nullStr{&a.EstadoNom}, &nullStr{&a.CiudadNom},
		&nullStr{&a.MunicipioNom}, &nullStr{&a.ParroquiaNom})
	if err != nil {
		return nil, err
	}
	a.InscripcionDiaExacto = diaExacto == 1
	a.Edad = edad(a.FechaNacimiento)
	a.EsMenor = a.Edad < 18

	// Teléfonos de contacto (0..3).
	if a.TelefonosContacto, err = telefonosContacto(db, id); err != nil {
		return nil, err
	}

	// Representante (puede no existir).
	var r Representante
	err = db.QueryRow(
		`SELECT cedula_tipo, cedula_numero, nombres, apellidos, telefono, parentesco
		   FROM representante WHERE atleta_id = ?`, id).
		Scan(&r.CedulaTipo, &r.CedulaNumero, &r.Nombres, &r.Apellidos, &r.Telefono, &r.Parentesco)
	if err == nil {
		a.Representante = &r
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if a.Cinturones, err = historialCinturon(db, id); err != nil {
		return nil, err
	}
	if len(a.Cinturones) > 0 {
		a.CinturonColor = a.Cinturones[0].Color
		a.CinturonDan = a.Cinturones[0].Dan
	}
	if a.Periodos, err = periodos(db, id); err != nil {
		return nil, err
	}
	a.Estado = "retirado"
	for _, p := range a.Periodos {
		if p.FechaFin == nil {
			a.Estado = "activo"
			break
		}
	}
	return a, nil
}

func historialCinturon(db *sql.DB, atletaID int64) ([]HistorialCinturon, error) {
	rows, err := db.Query(`
		SELECT h.id, h.cinturon_id, c.color, h.dan, h.fecha_cambio, h.registrado_por
		  FROM historial_cinturon h
		  JOIN cinturon c ON c.id = h.cinturon_id
		 WHERE h.atleta_id = ?
		 ORDER BY h.fecha_cambio DESC, h.id DESC`, atletaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HistorialCinturon
	for rows.Next() {
		var h HistorialCinturon
		if err := rows.Scan(&h.ID, &h.CinturonID, &h.Color, &h.Dan, &h.FechaCambio, &h.RegistradoPor); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func periodos(db *sql.DB, atletaID int64) ([]Periodo, error) {
	rows, err := db.Query(`
		SELECT id, fecha_inicio, fecha_fin, motivo_retiro
		  FROM periodo_actividad WHERE atleta_id = ?
		 ORDER BY fecha_inicio DESC, id DESC`, atletaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Periodo
	for rows.Next() {
		var p Periodo
		if err := rows.Scan(&p.ID, &p.FechaInicio, &p.FechaFin, &p.MotivoRetiro); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListAtletas devuelve la página de resultados según las facetas y el total
// global que cumple el filtro (para paginación).
func ListAtletas(db *sql.DB, f AtletaFiltro) ([]Atleta, int, error) {
	where, args, err := buildFiltro(f)
	if err != nil {
		return nil, 0, err
	}

	var total int
	if err := db.QueryRow(`SELECT count(*) FROM atleta a `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 25
	}
	q := `
		SELECT a.id, a.foto_path, a.nombres, a.apellidos, a.cedula_tipo, a.cedula_numero,
		       a.fecha_nacimiento, a.escuela_id, e.nombre, m.nombre,
		       (SELECT c.color FROM historial_cinturon h JOIN cinturon c ON c.id=h.cinturon_id
		         WHERE h.atleta_id=a.id ORDER BY h.fecha_cambio DESC, h.id DESC LIMIT 1) AS cint,
		       (SELECT h.dan FROM historial_cinturon h
		         WHERE h.atleta_id=a.id ORDER BY h.fecha_cambio DESC, h.id DESC LIMIT 1) AS dan,
		       CASE WHEN EXISTS(SELECT 1 FROM periodo_actividad p
		                         WHERE p.atleta_id=a.id AND p.fecha_fin IS NULL)
		            THEN 'activo' ELSE 'retirado' END AS estado
		  FROM atleta a
		  LEFT JOIN escuela e   ON e.id = a.escuela_id
		  LEFT JOIN municipio m ON m.id = a.municipio_id
		` + where + `
		 ORDER BY a.apellidos, a.nombres
		 LIMIT ? OFFSET ?`
	args = append(args, limit, f.Offset)

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []Atleta
	for rows.Next() {
		var a Atleta
		if err := rows.Scan(&a.ID, &a.FotoPath, &a.Nombres, &a.Apellidos, &a.CedulaTipo, &a.CedulaNumero,
			&a.FechaNacimiento, &a.EscuelaID, &nullStr{&a.EscuelaNombre},
			&nullStr{&a.MunicipioNom}, &nullStr{&a.CinturonColor}, &a.CinturonDan,
			&a.Estado); err != nil {
			return nil, 0, err
		}
		a.Edad = edad(a.FechaNacimiento)
		a.EsMenor = a.Edad < 18
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// ---- Cinturón y periodos (operaciones puntuales) ---------------------------

// AgregarCinturon añade una fila a la línea de tiempo de grados.
func AgregarCinturon(db *sql.DB, atletaID, cinturonID int64, dan *int, fecha string, entrenadorID int64) error {
	_, err := db.Exec(`
		INSERT INTO historial_cinturon (atleta_id, cinturon_id, dan, fecha_cambio, registrado_por)
		VALUES (?,?,?,?,?)`, atletaID, cinturonID, dan, fecha, nullID(entrenadorID))
	if err != nil {
		return err
	}
	Auditar(db, entrenadorID, "INSERT", "historial_cinturon", atletaID,
		map[string]any{"cinturon_id": cinturonID, "dan": dan})
	return nil
}

// Retirar cierra el periodo activo (si existe) con la fecha y motivo dados.
func Retirar(db *sql.DB, atletaID int64, fecha string, motivo *string, entrenadorID int64) error {
	res, err := db.Exec(`
		UPDATE periodo_actividad SET fecha_fin = ?, motivo_retiro = ?
		 WHERE atleta_id = ? AND fecha_fin IS NULL`, fecha, motivo, atletaID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("el atleta ya está retirado")
	}
	Auditar(db, entrenadorID, "UPDATE", "periodo_actividad", atletaID,
		map[string]any{"accion": "retiro", "fecha": fecha})
	return nil
}

// Reactivar abre un nuevo periodo activo (regla #3: no debe haber otro abierto).
func Reactivar(db *sql.DB, atletaID int64, fecha string, entrenadorID int64) error {
	var abierto int
	if err := db.QueryRow(
		`SELECT count(*) FROM periodo_actividad WHERE atleta_id=? AND fecha_fin IS NULL`,
		atletaID).Scan(&abierto); err != nil {
		return err
	}
	if abierto > 0 {
		return errors.New("el atleta ya está activo")
	}
	if _, err := db.Exec(
		`INSERT INTO periodo_actividad (atleta_id, fecha_inicio) VALUES (?, ?)`,
		atletaID, fecha); err != nil {
		return err
	}
	Auditar(db, entrenadorID, "INSERT", "periodo_actividad", atletaID,
		map[string]any{"accion": "reactivacion", "fecha": fecha})
	return nil
}

// ---- helpers internos ------------------------------------------------------

func upsertRepresentanteTx(tx *sql.Tx, atletaID int64, r *Representante) error {
	_, err := tx.Exec(`
		INSERT INTO representante (atleta_id, cedula_tipo, cedula_numero, nombres, apellidos, telefono, parentesco)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(atleta_id) DO UPDATE SET
			cedula_tipo=excluded.cedula_tipo, cedula_numero=excluded.cedula_numero,
			nombres=excluded.nombres, apellidos=excluded.apellidos,
			telefono=excluded.telefono, parentesco=excluded.parentesco`,
		atletaID, r.CedulaTipo, r.CedulaNumero, r.Nombres, r.Apellidos, r.Telefono, r.Parentesco)
	return err
}

// reemplazarTelefonosTx borra y reinserta los teléfonos de contacto del atleta.
func reemplazarTelefonosTx(tx *sql.Tx, atletaID int64, telefonos []string) error {
	if _, err := tx.Exec(`DELETE FROM atleta_telefono_contacto WHERE atleta_id = ?`, atletaID); err != nil {
		return err
	}
	for _, t := range telefonos {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO atleta_telefono_contacto (atleta_id, numero) VALUES (?, ?)`, atletaID, t,
		); err != nil {
			return err
		}
	}
	return nil
}

func telefonosContacto(db *sql.DB, atletaID int64) ([]string, error) {
	rows, err := db.Query(
		`SELECT numero FROM atleta_telefono_contacto WHERE atleta_id = ? ORDER BY id`, atletaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func edad(fechaNac string) int {
	t, err := time.Parse("2006-01-02", fechaNac)
	if err != nil {
		return 0
	}
	now := time.Now()
	years := now.Year() - t.Year()
	if now.YearDay() < t.YearDay() {
		years--
	}
	if years < 0 {
		years = 0
	}
	return years
}

// rowQuerier abstrae *sql.DB / *sql.Tx para consultas de una fila.
type rowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

// NormalizarUbicacion rellena los niveles superiores de la ubicación a partir
// del nivel más profundo seleccionado, garantizando coherencia jerárquica
// (parroquia → municipio → ciudad → estado). Ignora selecciones inferiores
// incoherentes.
func NormalizarUbicacion(q rowQuerier, in *AtletaInput) {
	switch {
	case in.ParroquiaID != nil:
		var est, mun int64
		var ciu sql.NullInt64
		if err := q.QueryRow(
			`SELECT p.estado_id, p.municipio_id, m.ciudad_id
			   FROM parroquia p JOIN municipio m ON m.id = p.municipio_id
			  WHERE p.id = ?`, *in.ParroquiaID).Scan(&est, &mun, &ciu); err == nil {
			in.EstadoID, in.MunicipioID, in.CiudadID = &est, &mun, nullToPtr(ciu)
		}
	case in.MunicipioID != nil:
		var est int64
		var ciu sql.NullInt64
		if err := q.QueryRow(
			`SELECT estado_id, ciudad_id FROM municipio WHERE id = ?`, *in.MunicipioID).
			Scan(&est, &ciu); err == nil {
			in.EstadoID, in.CiudadID = &est, nullToPtr(ciu)
		}
		in.ParroquiaID = nil
	case in.CiudadID != nil:
		var est int64
		if err := q.QueryRow(`SELECT estado_id FROM ciudad WHERE id = ?`, *in.CiudadID).
			Scan(&est); err == nil {
			in.EstadoID = &est
		}
		in.MunicipioID, in.ParroquiaID = nil, nil
	default:
		// Solo estado (o nada): limpiar niveles inferiores.
		in.CiudadID, in.MunicipioID, in.ParroquiaID = nil, nil, nil
	}
}

func nullToPtr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullID(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}

// ErrCedulaDuplicada indica que ya existe un atleta con la misma cédula
// (combinación tipo+número). Los handlers la traducen a un error de campo.
var ErrCedulaDuplicada = errors.New("ya existe un atleta con esa cédula")

// mapErr traduce errores de restricción de SQLite a mensajes de dominio.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	// La unicidad real es sobre (cedula_tipo, cedula_numero); SQLite reporta
	// "UNIQUE constraint failed: atleta.cedula_tipo, atleta.cedula_numero".
	if strings.Contains(err.Error(), "UNIQUE constraint failed") &&
		strings.Contains(err.Error(), "cedula_numero") {
		return ErrCedulaDuplicada
	}
	return err
}

// nullStr escanea un TEXT posiblemente NULL a un *string destino.
type nullStr struct{ dst *string }

func (n *nullStr) Scan(v any) error {
	if v == nil {
		*n.dst = ""
		return nil
	}
	switch s := v.(type) {
	case string:
		*n.dst = s
	case []byte:
		*n.dst = string(s)
	}
	return nil
}
