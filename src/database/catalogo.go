package database

import "database/sql"

// Catálogos de solo lectura usados para poblar los selectores del frontend.

func ListCinturones(db *sql.DB) ([]Cinturon, error) {
	rows, err := db.Query(`SELECT id, color, orden, es_negro FROM cinturon ORDER BY orden`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Cinturon{}
	for rows.Next() {
		var c Cinturon
		if err := rows.Scan(&c.ID, &c.Color, &c.Orden, &c.EsNegro); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func ListMunicipios(db *sql.DB) ([]Municipio, error) {
	rows, err := db.Query(`SELECT id, estado_id, ciudad_id, nombre FROM municipio ORDER BY nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Municipio{}
	for rows.Next() {
		var m Municipio
		if err := rows.Scan(&m.ID, &m.EstadoID, &m.CiudadID, &m.Nombre); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func ListEscuelas(db *sql.DB) ([]Escuela, error) {
	rows, err := db.Query(`
		SELECT e.id, e.nombre, e.municipio_id, m.nombre, e.direccion, e.activa
		  FROM escuela e
		  JOIN municipio m ON m.id = e.municipio_id
		 ORDER BY e.nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Escuela{}
	for rows.Next() {
		var e Escuela
		if err := rows.Scan(&e.ID, &e.Nombre, &e.MunicipioID, &e.MunicipioNom, &e.Direccion, &e.Activa); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CinturonEsNegro indica si un id de cinturón corresponde al color negro
// (usado para validar la regla del DAN a nivel de aplicación).
func CinturonEsNegro(db *sql.DB, cinturonID int64) (bool, error) {
	var negro bool
	err := db.QueryRow(`SELECT es_negro FROM cinturon WHERE id = ?`, cinturonID).Scan(&negro)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return negro, err
}
