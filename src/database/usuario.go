package database

import (
	"database/sql"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// UsuarioInput son los datos de alta/edición de un usuario del sistema
// (entrenador). El rol se expresa con EsAdmin (Administrador/Consultor).
type UsuarioInput struct {
	Username  string  `json:"username"`
	Password  *string `json:"password"` // requerido al crear; opcional al editar
	Nombres   string  `json:"nombres"`
	Apellidos string  `json:"apellidos"`
	EsAdmin   bool    `json:"es_admin"`
	Estado    string  `json:"estado"` // activo | retirado
	EscuelaID *int64  `json:"escuela_id"`
}

// ListEntrenadores devuelve todos los usuarios del sistema.
func ListEntrenadores(db *sql.DB) ([]Entrenador, error) {
	rows, err := db.Query(
		`SELECT id, username, nombres, apellidos, es_admin, estado FROM entrenador ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Entrenador{}
	for rows.Next() {
		var e Entrenador
		if err := rows.Scan(&e.ID, &e.Username, &e.Nombres, &e.Apellidos, &e.EsAdmin, &e.Estado); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func CrearEntrenador(db *sql.DB, in UsuarioInput) (int64, error) {
	pass := ""
	if in.Password != nil {
		pass = *in.Password
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	estado := in.Estado
	if estado == "" {
		estado = "activo"
	}
	res, err := db.Exec(
		`INSERT INTO entrenador (username, password_hash, nombres, apellidos, es_admin, estado, escuela_id)
		 VALUES (?,?,?,?,?,?,?)`,
		in.Username, string(hash), in.Nombres, in.Apellidos, boolToInt(in.EsAdmin), estado, in.EscuelaID)
	if err != nil {
		return 0, mapUsuarioErr(err)
	}
	return res.LastInsertId()
}

// ActualizarEntrenador actualiza datos y, si se indica, la contraseña.
func ActualizarEntrenador(db *sql.DB, id int64, in UsuarioInput) error {
	estado := in.Estado
	if estado == "" {
		estado = "activo"
	}
	if err := afectado(db.Exec(
		`UPDATE entrenador SET username=?, nombres=?, apellidos=?, es_admin=?, estado=?, escuela_id=? WHERE id=?`,
		in.Username, in.Nombres, in.Apellidos, boolToInt(in.EsAdmin), estado, in.EscuelaID, id)); err != nil {
		return mapUsuarioErr(err)
	}
	if in.Password != nil && *in.Password != "" {
		return CambiarPassword(db, id, *in.Password)
	}
	return nil
}

func EliminarEntrenador(db *sql.DB, id int64) error {
	return afectado(db.Exec(`DELETE FROM entrenador WHERE id=?`, id))
}

// ---- salvaguardas ---------------------------------------------------------

// IDAdminOrigen devuelve el id del administrador de origen (el de menor id).
func IDAdminOrigen(db *sql.DB) int64 {
	var id int64
	db.QueryRow(`SELECT id FROM entrenador ORDER BY id LIMIT 1`).Scan(&id)
	return id
}

// ContarAdminsActivos cuenta administradores en estado activo (excluyendo un id).
func ContarAdminsActivos(db *sql.DB, exceptoID int64) int {
	var n int
	db.QueryRow(
		`SELECT count(*) FROM entrenador WHERE es_admin=1 AND estado='activo' AND id<>?`, exceptoID).Scan(&n)
	return n
}

func mapUsuarioErr(err error) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return ErrUsernameTaken
	}
	return err
}
