package backend

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"zon-taekwondo/database"
)

// handleFoto enruta la foto de un atleta: /api/atletas/{id}/foto
//   GET    → sirve la imagen (menores: solo administrador).
//   POST   → sube/renueva la foto (multipart, campo "foto"; solo admin).
//   DELETE → elimina la foto (solo admin).
func (s *Server) handleFoto(w http.ResponseWriter, r *http.Request, id int64) {
	switch r.Method {
	case http.MethodGet:
		s.servirFoto(w, r, id)
	case http.MethodPost:
		s.subirFoto(w, r, id)
	case http.MethodDelete:
		s.eliminarFoto(w, r, id)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
	}
}

func (s *Server) servirFoto(w http.ResponseWriter, r *http.Request, id int64) {
	// Regla #5: la foto de un menor es información sensible (solo admin).
	if s.esMenorRestringido(w, r, id) {
		return
	}
	path, err := database.FotoPath(s.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "atleta no encontrado")
		return
	}
	if err != nil || path == "" {
		writeErr(w, http.StatusNotFound, "el atleta no tiene foto")
		return
	}
	if _, err := os.Stat(path); err != nil {
		writeErr(w, http.StatusNotFound, "el atleta no tiene foto")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, path)
}

func (s *Server) subirFoto(w http.ResponseWriter, r *http.Request, id int64) {
	if !s.soloAdmin(w, r) {
		return
	}
	if _, err := database.GetAtleta(s.db, id); errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "atleta no encontrado")
		return
	}
	maxBytes := int64(database.MaxUploadMB(s.db)) * 1024 * 1024
	if err := r.ParseMultipartForm(maxBytes + 1024); err != nil {
		writeErr(w, http.StatusBadRequest, "no se pudo leer el archivo")
		return
	}
	file, hdr, err := r.FormFile("foto")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "adjunte una imagen en el campo 'foto'")
		return
	}
	defer file.Close()

	ext := extFoto(hdr.Filename)
	if ext == "" {
		writeErr(w, http.StatusUnprocessableEntity, "formato no admitido: use JPG, JPEG o PNG")
		return
	}
	// Verificar la firma binaria real (no confiar solo en la extensión).
	head := make([]byte, 8)
	n, _ := io.ReadFull(file, head)
	if !esImagen(head[:n]) {
		writeErr(w, http.StatusUnprocessableEntity, "el archivo no es una imagen JPG o PNG válida")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeErr(w, http.StatusInternalServerError, "error leyendo el archivo")
		return
	}

	borrarFotosDe(id) // quitar cualquier foto previa (aunque cambie la extensión)
	destino := filepath.Join(fotosDir(), fmt.Sprintf("%d%s", id, ext))
	if err := guardarArchivo(destino, file, maxBytes); err != nil {
		if errors.Is(err, errArchivoGrande) {
			writeErr(w, http.StatusRequestEntityTooLarge,
				fmt.Sprintf("la imagen supera el máximo de %d MB", database.MaxUploadMB(s.db)))
			return
		}
		writeErr(w, http.StatusInternalServerError, "no se pudo guardar la imagen")
		return
	}
	if err := database.SetFotoPath(s.db, id, destino); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	ses, _ := s.sessions.obtener(r)
	database.Auditar(s.db, ses.EntrenadorID, "UPDATE", "atleta", id, map[string]string{"foto": "actualizada"})
	s.hub.Broadcast(Evento{Tipo: "atleta.actualizado", Recurso: "atleta", ID: id, Por: ses.Username})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) eliminarFoto(w http.ResponseWriter, r *http.Request, id int64) {
	if !s.soloAdmin(w, r) {
		return
	}
	borrarFotosDe(id)
	if err := database.SetFotoPath(s.db, id, ""); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	ses, _ := s.sessions.obtener(r)
	database.Auditar(s.db, ses.EntrenadorID, "UPDATE", "atleta", id, map[string]string{"foto": "eliminada"})
	s.hub.Broadcast(Evento{Tipo: "atleta.actualizado", Recurso: "atleta", ID: id, Por: ses.Username})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// esMenorRestringido escribe 403 y devuelve true si el atleta es menor de edad
// y el usuario en sesión NO es administrador (protege fotos/documentos sensibles).
func (s *Server) esMenorRestringido(w http.ResponseWriter, r *http.Request, id int64) bool {
	ses, _ := s.sessions.obtener(r)
	if ses.EsAdmin {
		return false
	}
	menor, err := database.EsMenorDeEdad(s.db, id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "atleta no encontrado")
		return true
	}
	if menor {
		writeErr(w, http.StatusForbidden, "información de un menor: solo el administrador puede acceder")
		return true
	}
	return false
}
