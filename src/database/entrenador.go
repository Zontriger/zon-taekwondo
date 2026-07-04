package database

import (
	"database/sql"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// ErrCredenciales se devuelve cuando el usuario no existe o la clave no coincide.
var ErrCredenciales = errors.New("usuario o contraseña inválidos")

// ErrUsernameTaken se devuelve al intentar usar un username ya existente.
var ErrUsernameTaken = errors.New("el nombre de usuario ya está en uso")

// Autenticar verifica username + password contra el hash bcrypt almacenado.
// Solo permite el acceso a entrenadores en estado 'activo'.
func Autenticar(db *sql.DB, username, password string) (*Entrenador, error) {
	var (
		e    Entrenador
		hash string
	)
	err := db.QueryRow(
		`SELECT id, username, password_hash, nombres, apellidos, es_admin, estado
		   FROM entrenador WHERE username = ? AND estado = 'activo'`,
		username,
	).Scan(&e.ID, &e.Username, &hash, &e.Nombres, &e.Apellidos, &e.EsAdmin, &e.Estado)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCredenciales
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return nil, ErrCredenciales
	}
	return &e, nil
}

// GetEntrenador devuelve los datos públicos de un entrenador por id.
func GetEntrenador(db *sql.DB, id int64) (*Entrenador, error) {
	var e Entrenador
	err := db.QueryRow(
		`SELECT id, username, nombres, apellidos, es_admin, estado
		   FROM entrenador WHERE id = ?`, id,
	).Scan(&e.ID, &e.Username, &e.Nombres, &e.Apellidos, &e.EsAdmin, &e.Estado)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// CambiarPassword actualiza el hash de la contraseña de un entrenador.
func CambiarPassword(db *sql.DB, id int64, nueva string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(nueva), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE entrenador SET password_hash = ? WHERE id = ?`, string(hash), id)
	return err
}

// ActualizarUsername cambia el username validando unicidad (excluyendo al propio).
func ActualizarUsername(db *sql.DB, id int64, username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("el usuario no puede estar vacío")
	}
	var n int
	if err := db.QueryRow(
		`SELECT count(*) FROM entrenador WHERE username = ? AND id <> ?`, username, id,
	).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrUsernameTaken
	}
	_, err := db.Exec(`UPDATE entrenador SET username = ? WHERE id = ?`, username, id)
	return err
}
