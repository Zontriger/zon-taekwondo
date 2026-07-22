package database

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode"
)

// Los documentos y fotos existen para dos entidades: atletas y entrenadores
// (maestros). Ambas comparten la misma estructura de tabla de documentos y una
// columna foto_path, así que las operaciones se parametrizan con DocTabla /
// nombre de tabla (siempre desde esta lista blanca, nunca del usuario).

// DocTabla identifica la tabla de documentos de una entidad y su columna dueño.
type DocTabla struct {
	Tabla string // "documento" | "documento_maestro"
	Col   string // "atleta_id" | "maestro_id"
}

var (
	DocsAtleta  = DocTabla{"documento", "atleta_id"}
	DocsMaestro = DocTabla{"documento_maestro", "maestro_id"}
)

// tablasConFoto es la lista blanca de tablas con columna foto_path.
var tablasConFoto = map[string]bool{"atleta": true, "maestro": true}

// Documento es un archivo (PDF o imagen) asociado a un atleta o entrenador.
// El archivo vive en disco bajo data/; en la base solo se guarda su ruta.
type Documento struct {
	ID       int64  `json:"id"`
	OwnerID  int64  `json:"-"` // id del atleta o maestro dueño
	Nombre   string `json:"nombre"`
	Archivo  string `json:"-"` // ruta relativa en disco (no se expone al cliente)
	CreadoEn string `json:"creado_en"`
	// Enriquecidos por el handler (no son columnas):
	Tipo   string `json:"tipo"`   // "pdf" | "img"
	Existe bool   `json:"existe"` // si el archivo sigue en disco
}

// ListDocumentos devuelve los documentos del dueño (más reciente primero).
func ListDocumentos(db *sql.DB, dt DocTabla, ownerID int64) ([]Documento, error) {
	rows, err := db.Query(fmt.Sprintf(
		`SELECT id, %s, nombre, archivo, creado_en
		   FROM %s WHERE %s = ? ORDER BY creado_en DESC, id DESC`, dt.Col, dt.Tabla, dt.Col), ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Documento{}
	for rows.Next() {
		var d Documento
		if err := rows.Scan(&d.ID, &d.OwnerID, &d.Nombre, &d.Archivo, &d.CreadoEn); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetDocumento devuelve un documento por id verificando que pertenezca al dueño.
func GetDocumento(db *sql.DB, dt DocTabla, ownerID, docID int64) (*Documento, error) {
	var d Documento
	err := db.QueryRow(fmt.Sprintf(
		`SELECT id, %s, nombre, archivo, creado_en
		   FROM %s WHERE id = ? AND %s = ?`, dt.Col, dt.Tabla, dt.Col), docID, ownerID).
		Scan(&d.ID, &d.OwnerID, &d.Nombre, &d.Archivo, &d.CreadoEn)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// CrearDocumento inserta un documento y devuelve su id.
func CrearDocumento(db *sql.DB, dt DocTabla, ownerID int64, nombre, archivo string) (int64, error) {
	res, err := db.Exec(fmt.Sprintf(
		`INSERT INTO %s (%s, nombre, archivo) VALUES (?, ?, ?)`, dt.Tabla, dt.Col),
		ownerID, nombre, archivo)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SetDocumentoArchivo fija la ruta en disco del documento (tras guardarlo).
func SetDocumentoArchivo(db *sql.DB, dt DocTabla, docID int64, archivo string) error {
	_, err := db.Exec(fmt.Sprintf(`UPDATE %s SET archivo = ? WHERE id = ?`, dt.Tabla), archivo, docID)
	return err
}

// EliminarDocumento borra el registro y devuelve la ruta del archivo a eliminar.
func EliminarDocumento(db *sql.DB, dt DocTabla, ownerID, docID int64) (string, error) {
	d, err := GetDocumento(db, dt, ownerID, docID)
	if err != nil {
		return "", err
	}
	if _, err := db.Exec(fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, dt.Tabla), docID); err != nil {
		return "", err
	}
	return d.Archivo, nil
}

// SetFotoPath actualiza la ruta de la foto del registro (o la limpia con "").
func SetFotoPath(db *sql.DB, tabla string, id int64, path string) error {
	if !tablasConFoto[tabla] {
		return fmt.Errorf("tabla sin foto: %s", tabla)
	}
	var p any
	if path != "" {
		p = path
	}
	_, err := db.Exec(fmt.Sprintf(`UPDATE %s SET foto_path = ? WHERE id = ?`, tabla), p, id)
	return err
}

// FotoPath devuelve la ruta de la foto del registro ("" si no tiene).
func FotoPath(db *sql.DB, tabla string, id int64) (string, error) {
	if !tablasConFoto[tabla] {
		return "", fmt.Errorf("tabla sin foto: %s", tabla)
	}
	var p sql.NullString
	err := db.QueryRow(fmt.Sprintf(`SELECT foto_path FROM %s WHERE id = ?`, tabla), id).Scan(&p)
	if err != nil {
		return "", err
	}
	return p.String, nil
}

// ExisteRegistro indica si existe un registro con ese id en la tabla dada.
func ExisteRegistro(db *sql.DB, tabla string, id int64) bool {
	if !tablasConFoto[tabla] {
		return false
	}
	var uno int
	return db.QueryRow(fmt.Sprintf(`SELECT 1 FROM %s WHERE id = ?`, tabla), id).Scan(&uno) == nil
}

// EsMenorDeEdad indica si el atleta es menor de 18 años (para restringir el
// acceso a información sensible: fotos y documentos de menores).
func EsMenorDeEdad(db *sql.DB, atletaID int64) (bool, error) {
	var nac string
	if err := db.QueryRow(`SELECT fecha_nacimiento FROM atleta WHERE id = ?`, atletaID).Scan(&nac); err != nil {
		return false, err
	}
	return edad(nac) < 18, nil
}

// NombreArchivoDe devuelve un nombre de archivo seguro (apellidos_nombres) del
// atleta o maestro, para nombrar descargas (planillas, .zip de documentos).
func NombreArchivoDe(db *sql.DB, tabla string, id int64) string {
	if !tablasConFoto[tabla] {
		return "registro"
	}
	var nombres, apellidos string
	if err := db.QueryRow(fmt.Sprintf(`SELECT nombres, apellidos FROM %s WHERE id = ?`, tabla), id).
		Scan(&nombres, &apellidos); err != nil {
		return "registro"
	}
	return slug(apellidos + "_" + nombres)
}

// slug normaliza un texto a un nombre de archivo seguro: minúsculas, conserva
// letras (incluidas ñ y tildes), dígitos, '_' y '-'.
func slug(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-':
			b.WriteRune(r)
		case r == ' ' || r == '_':
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "registro"
	}
	return out
}
