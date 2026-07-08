package backend

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Raíz del almacenamiento de archivos subidos, junto al ejecutable. Se respalda
// completa (junto con app.db) en el "respaldo completo".
const dataDir = "data"

func fotosDir() string       { return filepath.Join(dataDir, "fotos") }
func docsDir(id int64) string { return filepath.Join(dataDir, "docs", fmt.Sprintf("%d", id)) }

// extensionesFoto permitidas para la foto del atleta.
var extensionesFoto = map[string]bool{".jpg": true, ".jpeg": true, ".png": true}

// guardarArchivo escribe el contenido de r en destino, creando la carpeta si
// hace falta. Limita la lectura a maxBytes+1 para detectar excesos de tamaño.
func guardarArchivo(destino string, r io.Reader, maxBytes int64) error {
	if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
		return err
	}
	tmp := destino + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	limited := io.LimitReader(r, maxBytes+1)
	n, err := io.Copy(f, limited)
	f.Close()
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if n > maxBytes {
		os.Remove(tmp)
		return errArchivoGrande
	}
	// Renombrado atómico sobre el destino final.
	if err := os.Rename(tmp, destino); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

var errArchivoGrande = fmt.Errorf("el archivo supera el tamaño máximo permitido")

// esPDF verifica los primeros bytes ("%PDF-") de la cabecera del archivo.
func esPDF(head []byte) bool {
	return len(head) >= 5 && string(head[:5]) == "%PDF-"
}

// extDesdeMagic devuelve la extensión (.pdf/.jpg/.png) según la firma binaria,
// o "" si el archivo no es un tipo admitido para documentos.
func extDesdeMagic(head []byte) string {
	if esPDF(head) {
		return ".pdf"
	}
	if len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF {
		return ".jpg"
	}
	if len(head) >= 8 && string(head[:8]) == "\x89PNG\r\n\x1a\n" {
		return ".png"
	}
	return ""
}

// tipoDoc clasifica un documento por su extensión: "pdf" | "img" | "".
func tipoDoc(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		return "pdf"
	case ".jpg", ".jpeg", ".png":
		return "img"
	}
	return ""
}

// contentTypeDoc devuelve el Content-Type según la extensión del documento.
func contentTypeDoc(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	default:
		return "image/jpeg"
	}
}

// esImagen verifica la firma binaria (magic bytes) de JPEG o PNG.
func esImagen(head []byte) bool {
	if len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF {
		return true // JPEG
	}
	if len(head) >= 8 && string(head[:8]) == "\x89PNG\r\n\x1a\n" {
		return true // PNG
	}
	return false
}

// extFoto normaliza la extensión de una foto a partir del nombre del archivo
// original; devuelve "" si no es una extensión de imagen permitida.
func extFoto(nombre string) string {
	ext := strings.ToLower(filepath.Ext(nombre))
	if ext == ".jpeg" {
		ext = ".jpg"
	}
	if !extensionesFoto[ext] && ext != ".jpg" {
		return ""
	}
	return ext
}

// borrarFotosDe elimina cualquier archivo de foto previo del atleta (sin
// importar su extensión) antes de guardar una nueva o al eliminarla.
func borrarFotosDe(id int64) {
	for ext := range extensionesFoto {
		os.Remove(filepath.Join(fotosDir(), fmt.Sprintf("%d%s", id, ext)))
	}
}
