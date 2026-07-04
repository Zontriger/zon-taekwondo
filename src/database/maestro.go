package database

import "database/sql"

// MaestroInput son los datos de alta/edición de un entrenador deportivo.
type MaestroInput struct {
	Nombres      string  `json:"nombres"`
	Apellidos    string  `json:"apellidos"`
	CedulaTipo   *string `json:"cedula_tipo"`
	CedulaNumero *string `json:"cedula_numero"`
	Telefono     *string `json:"telefono"`
	EscuelaID    *int64  `json:"escuela_id"`
	CinturonID   *int64  `json:"cinturon_id"`
	Dan          *int    `json:"dan"`
	Activo       bool    `json:"activo"`
}

func ListMaestros(db *sql.DB) ([]Maestro, error) {
	rows, err := db.Query(`
		SELECT m.id, m.nombres, m.apellidos, m.cedula_tipo, m.cedula_numero, m.telefono,
		       m.escuela_id, e.nombre, m.cinturon_id, c.color, m.dan, m.activo,
		       (SELECT count(*) FROM atleta a WHERE a.maestro_id = m.id)
		  FROM maestro m
		  LEFT JOIN escuela e  ON e.id = m.escuela_id
		  LEFT JOIN cinturon c ON c.id = m.cinturon_id
		 ORDER BY m.apellidos, m.nombres`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Maestro{}
	for rows.Next() {
		var m Maestro
		if err := rows.Scan(&m.ID, &m.Nombres, &m.Apellidos, &m.CedulaTipo, &m.CedulaNumero, &m.Telefono,
			&m.EscuelaID, &nullStr{&m.EscuelaNombre}, &m.CinturonID, &nullStr{&m.CinturonColor},
			&m.Dan, &m.Activo, &m.NumAtletas); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func CrearMaestro(db *sql.DB, in MaestroInput) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO maestro (nombres, apellidos, cedula_tipo, cedula_numero, telefono,
		                     escuela_id, cinturon_id, dan, activo)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		in.Nombres, in.Apellidos, in.CedulaTipo, in.CedulaNumero, in.Telefono,
		in.EscuelaID, in.CinturonID, in.Dan, boolToInt(in.Activo))
	if err != nil {
		return 0, mapGeoErr(err)
	}
	return res.LastInsertId()
}

func ActualizarMaestro(db *sql.DB, id int64, in MaestroInput) error {
	return afectado(db.Exec(`
		UPDATE maestro SET nombres=?, apellidos=?, cedula_tipo=?, cedula_numero=?, telefono=?,
		                   escuela_id=?, cinturon_id=?, dan=?, activo=? WHERE id=?`,
		in.Nombres, in.Apellidos, in.CedulaTipo, in.CedulaNumero, in.Telefono,
		in.EscuelaID, in.CinturonID, in.Dan, boolToInt(in.Activo), id))
}

func EliminarMaestro(db *sql.DB, id int64) error {
	return afectado(db.Exec(`DELETE FROM maestro WHERE id=?`, id))
}
