package database

import (
	"database/sql"
	"strconv"
)

// Configuración del sistema como pares clave/valor (tabla 'config'). Por ahora
// el único ajuste es el tamaño máximo por archivo subido (fotos y documentos).

const (
	claveMaxUploadMB = "max_upload_mb"
	defaultMaxUpload = 5
)

// GetConfig devuelve todos los ajustes como un mapa clave→valor.
func GetConfig(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query(`SELECT clave, valor FROM config`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// SetConfig inserta o actualiza un ajuste.
func SetConfig(db *sql.DB, clave, valor string) error {
	_, err := db.Exec(
		`INSERT INTO config (clave, valor) VALUES (?, ?)
		 ON CONFLICT(clave) DO UPDATE SET valor = excluded.valor`,
		clave, valor)
	return err
}

// MaxUploadMB devuelve el límite de tamaño (MB) por archivo, con respaldo al
// valor por defecto si el ajuste no existe o es inválido.
func MaxUploadMB(db *sql.DB) int {
	var v string
	if err := db.QueryRow(`SELECT valor FROM config WHERE clave = ?`, claveMaxUploadMB).Scan(&v); err != nil {
		return defaultMaxUpload
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return defaultMaxUpload
	}
	return n
}
