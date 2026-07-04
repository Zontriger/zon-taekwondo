package database

import (
	"database/sql"
	"errors"
	"strings"
)

// Listados de la jerarquía geográfica (para selects en cascada y datos maestros).

func ListEstados(db *sql.DB) ([]Estado, error) {
	rows, err := db.Query(`SELECT id, nombre FROM estado ORDER BY nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Estado{}
	for rows.Next() {
		var e Estado
		if err := rows.Scan(&e.ID, &e.Nombre); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func ListCiudades(db *sql.DB) ([]Ciudad, error) {
	rows, err := db.Query(`SELECT id, estado_id, nombre FROM ciudad ORDER BY nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Ciudad{}
	for rows.Next() {
		var c Ciudad
		if err := rows.Scan(&c.ID, &c.EstadoID, &c.Nombre); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func ListParroquias(db *sql.DB) ([]Parroquia, error) {
	rows, err := db.Query(`SELECT id, estado_id, municipio_id, nombre FROM parroquia ORDER BY nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Parroquia{}
	for rows.Next() {
		var p Parroquia
		if err := rows.Scan(&p.ID, &p.EstadoID, &p.MunicipioID, &p.Nombre); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ---- CRUD Estado ----------------------------------------------------------

func CrearEstado(db *sql.DB, nombre string) (int64, error) {
	res, err := db.Exec(`INSERT INTO estado (nombre) VALUES (?)`, nombre)
	if err != nil {
		return 0, mapGeoErr(err)
	}
	return res.LastInsertId()
}
func ActualizarEstado(db *sql.DB, id int64, nombre string) error {
	return afectado(db.Exec(`UPDATE estado SET nombre=? WHERE id=?`, nombre, id))
}
func EliminarEstado(db *sql.DB, id int64) error {
	return afectado(db.Exec(`DELETE FROM estado WHERE id=?`, id))
}

// ---- CRUD Ciudad ----------------------------------------------------------

func CrearCiudad(db *sql.DB, estadoID int64, nombre string) (int64, error) {
	res, err := db.Exec(`INSERT INTO ciudad (estado_id, nombre) VALUES (?,?)`, estadoID, nombre)
	if err != nil {
		return 0, mapGeoErr(err)
	}
	return res.LastInsertId()
}
func ActualizarCiudad(db *sql.DB, id, estadoID int64, nombre string) error {
	return afectado(db.Exec(`UPDATE ciudad SET estado_id=?, nombre=? WHERE id=?`, estadoID, nombre, id))
}
func EliminarCiudad(db *sql.DB, id int64) error {
	return afectado(db.Exec(`DELETE FROM ciudad WHERE id=?`, id))
}

// ---- CRUD Municipio -------------------------------------------------------
// Si se indica ciudad, el estado se deriva de ella (coherencia jerárquica).

func CrearMunicipio(db *sql.DB, estadoID int64, ciudadID *int64, nombre string) (int64, error) {
	estadoID = estadoDeCiudad(db, ciudadID, estadoID)
	res, err := db.Exec(`INSERT INTO municipio (estado_id, ciudad_id, nombre) VALUES (?,?,?)`, estadoID, ciudadID, nombre)
	if err != nil {
		return 0, mapGeoErr(err)
	}
	return res.LastInsertId()
}
func ActualizarMunicipio(db *sql.DB, id, estadoID int64, ciudadID *int64, nombre string) error {
	estadoID = estadoDeCiudad(db, ciudadID, estadoID)
	return afectado(db.Exec(`UPDATE municipio SET estado_id=?, ciudad_id=?, nombre=? WHERE id=?`, estadoID, ciudadID, nombre, id))
}
func EliminarMunicipio(db *sql.DB, id int64) error {
	return afectado(db.Exec(`DELETE FROM municipio WHERE id=?`, id))
}

// ---- CRUD Parroquia -------------------------------------------------------
// El estado se deriva del municipio.

func CrearParroquia(db *sql.DB, municipioID int64, nombre string) (int64, error) {
	estadoID, err := estadoDeMunicipio(db, municipioID)
	if err != nil {
		return 0, err
	}
	res, err := db.Exec(`INSERT INTO parroquia (estado_id, municipio_id, nombre) VALUES (?,?,?)`, estadoID, municipioID, nombre)
	if err != nil {
		return 0, mapGeoErr(err)
	}
	return res.LastInsertId()
}
func ActualizarParroquia(db *sql.DB, id, municipioID int64, nombre string) error {
	estadoID, err := estadoDeMunicipio(db, municipioID)
	if err != nil {
		return err
	}
	return afectado(db.Exec(`UPDATE parroquia SET estado_id=?, municipio_id=?, nombre=? WHERE id=?`, estadoID, municipioID, nombre, id))
}
func EliminarParroquia(db *sql.DB, id int64) error {
	return afectado(db.Exec(`DELETE FROM parroquia WHERE id=?`, id))
}

// ---- CRUD Cinturón --------------------------------------------------------

func CrearCinturon(db *sql.DB, color string, orden int, esNegro bool) (int64, error) {
	res, err := db.Exec(`INSERT INTO cinturon (color, orden, es_negro) VALUES (?,?,?)`, color, orden, boolToInt(esNegro))
	if err != nil {
		return 0, mapGeoErr(err)
	}
	return res.LastInsertId()
}
func ActualizarCinturon(db *sql.DB, id int64, color string, orden int, esNegro bool) error {
	return afectado(db.Exec(`UPDATE cinturon SET color=?, orden=?, es_negro=? WHERE id=?`, color, orden, boolToInt(esNegro), id))
}
func EliminarCinturon(db *sql.DB, id int64) error {
	return afectado(db.Exec(`DELETE FROM cinturon WHERE id=?`, id))
}

// ---- helpers --------------------------------------------------------------

func estadoDeCiudad(db *sql.DB, ciudadID *int64, fallback int64) int64 {
	if ciudadID == nil {
		return fallback
	}
	var est int64
	if err := db.QueryRow(`SELECT estado_id FROM ciudad WHERE id=?`, *ciudadID).Scan(&est); err == nil {
		return est
	}
	return fallback
}

func estadoDeMunicipio(db *sql.DB, municipioID int64) (int64, error) {
	var est int64
	err := db.QueryRow(`SELECT estado_id FROM municipio WHERE id=?`, municipioID).Scan(&est)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errors.New("municipio inválido")
	}
	return est, err
}

func afectado(res sql.Result, err error) error {
	if err != nil {
		return mapGeoErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// mapGeoErr traduce errores de SQLite (UNIQUE / FOREIGN KEY) a mensajes legibles.
func mapGeoErr(err error) error {
	if err == nil {
		return nil
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "UNIQUE"):
		return errors.New("ya existe un registro con ese nombre")
	case strings.Contains(s, "FOREIGN KEY"):
		return errors.New("no se puede eliminar: tiene registros asociados (municipios, parroquias o atletas)")
	}
	return err
}
