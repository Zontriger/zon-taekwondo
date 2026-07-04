package database

import "database/sql"

// EscuelaInput son los datos de alta/edición de una escuela.
type EscuelaInput struct {
	Nombre      string  `json:"nombre"`
	MunicipioID int64   `json:"municipio_id"`
	Direccion   *string `json:"direccion"`
	Activa      bool    `json:"activa"`
}

func CrearEscuela(db *sql.DB, in EscuelaInput) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO escuela (nombre, municipio_id, direccion, activa) VALUES (?,?,?,?)`,
		in.Nombre, in.MunicipioID, in.Direccion, boolToInt(in.Activa))
	if err != nil {
		return 0, mapGeoErr(err)
	}
	return res.LastInsertId()
}

func ActualizarEscuela(db *sql.DB, id int64, in EscuelaInput) error {
	return afectado(db.Exec(
		`UPDATE escuela SET nombre=?, municipio_id=?, direccion=?, activa=? WHERE id=?`,
		in.Nombre, in.MunicipioID, in.Direccion, boolToInt(in.Activa), id))
}

func EliminarEscuela(db *sql.DB, id int64) error {
	return afectado(db.Exec(`DELETE FROM escuela WHERE id=?`, id))
}
