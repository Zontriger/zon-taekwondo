package database

import (
	"database/sql"
	"strings"
	"unicode"
)

// Documento es un archivo PDF asociado a un atleta (partida de nacimiento,
// cédula, certificados de cinturón, etc.). El archivo vive en disco bajo
// data/docs/{atleta_id}/; en la base solo se guarda su ruta y nombre visible.
type Documento struct {
	ID       int64  `json:"id"`
	AtletaID int64  `json:"atleta_id"`
	Nombre   string `json:"nombre"`
	Archivo  string `json:"-"` // ruta relativa en disco (no se expone al cliente)
	CreadoEn string `json:"creado_en"`
	// Enriquecidos por el handler (no son columnas):
	Tipo   string `json:"tipo"`   // "pdf" | "img"
	Existe bool   `json:"existe"` // si el archivo sigue en disco
}

// ListDocumentos devuelve los documentos de un atleta (más reciente primero).
func ListDocumentos(db *sql.DB, atletaID int64) ([]Documento, error) {
	rows, err := db.Query(
		`SELECT id, atleta_id, nombre, archivo, creado_en
		   FROM documento WHERE atleta_id = ? ORDER BY creado_en DESC, id DESC`, atletaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Documento{}
	for rows.Next() {
		var d Documento
		if err := rows.Scan(&d.ID, &d.AtletaID, &d.Nombre, &d.Archivo, &d.CreadoEn); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetDocumento devuelve un documento por id verificando que pertenezca al atleta.
func GetDocumento(db *sql.DB, atletaID, docID int64) (*Documento, error) {
	var d Documento
	err := db.QueryRow(
		`SELECT id, atleta_id, nombre, archivo, creado_en
		   FROM documento WHERE id = ? AND atleta_id = ?`, docID, atletaID).
		Scan(&d.ID, &d.AtletaID, &d.Nombre, &d.Archivo, &d.CreadoEn)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// CrearDocumento inserta un documento y devuelve su id.
func CrearDocumento(db *sql.DB, atletaID int64, nombre, archivo string) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO documento (atleta_id, nombre, archivo) VALUES (?, ?, ?)`,
		atletaID, nombre, archivo)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SetDocumentoArchivo fija la ruta en disco del documento (tras guardarlo).
func SetDocumentoArchivo(db *sql.DB, docID int64, archivo string) error {
	_, err := db.Exec(`UPDATE documento SET archivo = ? WHERE id = ?`, archivo, docID)
	return err
}

// EliminarDocumento borra el registro y devuelve la ruta del archivo a eliminar.
func EliminarDocumento(db *sql.DB, atletaID, docID int64) (string, error) {
	d, err := GetDocumento(db, atletaID, docID)
	if err != nil {
		return "", err
	}
	if _, err := db.Exec(`DELETE FROM documento WHERE id = ?`, docID); err != nil {
		return "", err
	}
	return d.Archivo, nil
}

// SetFotoPath actualiza la ruta de la foto del atleta (o la limpia con "").
func SetFotoPath(db *sql.DB, atletaID int64, path string) error {
	var p any
	if path != "" {
		p = path
	}
	_, err := db.Exec(`UPDATE atleta SET foto_path = ? WHERE id = ?`, p, atletaID)
	return err
}

// FotoPath devuelve la ruta de la foto del atleta ("" si no tiene).
func FotoPath(db *sql.DB, atletaID int64) (string, error) {
	var p sql.NullString
	err := db.QueryRow(`SELECT foto_path FROM atleta WHERE id = ?`, atletaID).Scan(&p)
	if err != nil {
		return "", err
	}
	return p.String, nil
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

// NombreArchivoAtleta devuelve un nombre de archivo seguro basado en el atleta
// (apellidos_nombres) para nombrar descargas .zip de sus documentos.
func NombreArchivoAtleta(db *sql.DB, atletaID int64) string {
	var nombres, apellidos string
	if err := db.QueryRow(`SELECT nombres, apellidos FROM atleta WHERE id = ?`, atletaID).
		Scan(&nombres, &apellidos); err != nil {
		return "atleta"
	}
	return slug(apellidos + "_" + nombres)
}

// slug normaliza un texto a un nombre de archivo seguro: minúsculas, sin
// espacios ni caracteres problemáticos (deja letras, dígitos, '_' y '-').
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
		return "atleta"
	}
	return out
}
