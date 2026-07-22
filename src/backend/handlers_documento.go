package backend

import (
	"archive/zip"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"zon-taekwondo/database"
)

// handleDocumentos enruta el repositorio de documentos de una entidad (atleta o
// entrenador). sub llega ya sin el id: "documentos", "documentos/{docId}" o
// "documentos/zip".
func (s *Server) handleDocumentos(w http.ResponseWriter, r *http.Request, ent entArchivos, ownerID int64, sub string) {
	rest := strings.TrimPrefix(sub, "documentos")
	rest = strings.Trim(rest, "/")

	switch {
	case rest == "":
		switch r.Method {
		case http.MethodGet:
			s.listarDocumentos(w, ent, ownerID)
		case http.MethodPost:
			s.subirDocumento(w, r, ent, ownerID)
		default:
			writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		}
	case rest == "zip":
		s.descargarDocumentosZip(w, r, ent, ownerID)
	default:
		docID, err := strconv.ParseInt(rest, 10, 64)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "id de documento inválido")
			return
		}
		switch r.Method {
		case http.MethodGet:
			s.servirDocumento(w, r, ent, ownerID, docID)
		case http.MethodDelete:
			s.eliminarDocumento(w, r, ent, ownerID, docID)
		default:
			writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		}
	}
}

func (s *Server) listarDocumentos(w http.ResponseWriter, ent entArchivos, ownerID int64) {
	docs, err := database.ListDocumentos(s.db, ent.docs, ownerID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Enriquecer con el tipo (pdf/imagen) y si el archivo sigue en disco.
	for i := range docs {
		docs[i].Tipo = tipoDoc(docs[i].Archivo)
		_, e := os.Stat(docs[i].Archivo)
		docs[i].Existe = e == nil
	}
	writeJSON(w, http.StatusOK, docs)
}

func (s *Server) subirDocumento(w http.ResponseWriter, r *http.Request, ent entArchivos, ownerID int64) {
	if !s.soloAdmin(w, r) {
		return
	}
	if !database.ExisteRegistro(s.db, ent.tabla, ownerID) {
		writeErr(w, http.StatusNotFound, "registro no encontrado")
		return
	}
	maxBytes := int64(database.MaxUploadMB(s.db)) * 1024 * 1024
	if err := r.ParseMultipartForm(maxBytes + 1024); err != nil {
		writeErr(w, http.StatusBadRequest, "no se pudo leer el archivo")
		return
	}
	nombre := strings.TrimSpace(r.FormValue("nombre"))
	if nombre == "" {
		writeErr(w, http.StatusUnprocessableEntity, "el documento requiere un nombre")
		return
	}
	if len([]rune(nombre)) > 80 {
		nombre = string([]rune(nombre)[:80])
	}
	file, _, err := r.FormFile("archivo")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "adjunte un archivo en el campo 'archivo'")
		return
	}
	defer file.Close()

	head := make([]byte, 8)
	n, _ := io.ReadFull(file, head)
	ext := extDesdeMagic(head[:n])
	if ext == "" {
		writeErr(w, http.StatusUnprocessableEntity, "solo se aceptan archivos PDF o imágenes (JPG, PNG)")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeErr(w, http.StatusInternalServerError, "error leyendo el archivo")
		return
	}

	// Insertar primero para obtener el id y nombrar el archivo con él.
	docID, err := database.CrearDocumento(s.db, ent.docs, ownerID, nombre, "")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	destino := filepath.Join(ent.docsDir(ownerID), fmt.Sprintf("%d%s", docID, ext))
	if err := guardarArchivo(destino, file, maxBytes); err != nil {
		database.EliminarDocumento(s.db, ent.docs, ownerID, docID) // revertir el registro
		if errors.Is(err, errArchivoGrande) {
			writeErr(w, http.StatusRequestEntityTooLarge,
				fmt.Sprintf("el documento supera el máximo de %d MB", database.MaxUploadMB(s.db)))
			return
		}
		writeErr(w, http.StatusInternalServerError, "no se pudo guardar el documento")
		return
	}
	if err := database.SetDocumentoArchivo(s.db, ent.docs, docID, destino); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	ses, _ := s.sessions.obtener(r)
	database.Auditar(s.db, ses.EntrenadorID, "INSERT", ent.docs.Tabla, docID, map[string]string{"nombre": nombre})
	s.hub.Broadcast(Evento{Tipo: ent.recurso + ".actualizado", Recurso: ent.recurso, ID: ownerID, Por: ses.Username})
	writeJSON(w, http.StatusCreated, map[string]any{"id": docID, "nombre": nombre})
}

func (s *Server) servirDocumento(w http.ResponseWriter, r *http.Request, ent entArchivos, ownerID, docID int64) {
	// Regla #5: los documentos de un atleta menor son sensibles (solo admin).
	if s.esMenorRestringido(w, r, ent, ownerID) {
		return
	}
	d, err := database.GetDocumento(s.db, ent.docs, ownerID, docID)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "documento no encontrado")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	f, err := os.Open(d.Archivo)
	if err != nil {
		writeErr(w, http.StatusNotFound, "el archivo del documento no existe")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", contentTypeDoc(d.Archivo))
	// ?download=1 fuerza la descarga; por defecto se abre en el visor del navegador.
	disp := "inline"
	if r.URL.Query().Get("download") == "1" {
		disp = "attachment"
	}
	setDisposition(w, disp, database.NombreArchivoDe(s.db, ent.tabla, ownerID)+"_"+sanitizar(d.Nombre)+strings.ToLower(filepath.Ext(d.Archivo)))
	io.Copy(w, f)
}

func (s *Server) eliminarDocumento(w http.ResponseWriter, r *http.Request, ent entArchivos, ownerID, docID int64) {
	if !s.soloAdmin(w, r) {
		return
	}
	archivo, err := database.EliminarDocumento(s.db, ent.docs, ownerID, docID)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "documento no encontrado")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if archivo != "" {
		os.Remove(archivo)
	}
	ses, _ := s.sessions.obtener(r)
	database.Auditar(s.db, ses.EntrenadorID, "DELETE", ent.docs.Tabla, docID, nil)
	s.hub.Broadcast(Evento{Tipo: ent.recurso + ".actualizado", Recurso: ent.recurso, ID: ownerID, Por: ses.Username})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// descargarDocumentosZip agrupa varios documentos de la entidad en un .zip.
// Recibe los ids en 'ids' (coma-separados); sin ids, incluye todos. Nombre:
// {apellidos_nombres}_documentos.zip.
func (s *Server) descargarDocumentosZip(w http.ResponseWriter, r *http.Request, ent entArchivos, ownerID int64) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	// Descarga de documentos de un menor: solo administrador.
	if s.esMenorRestringido(w, r, ent, ownerID) {
		return
	}
	docs, err := database.ListDocumentos(s.db, ent.docs, ownerID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	sel := idsSeleccionados(r.URL.Query().Get("ids"))
	base := database.NombreArchivoDe(s.db, ent.tabla, ownerID)

	w.Header().Set("Content-Type", "application/zip")
	setAttachment(w, base+"_documentos.zip")
	zw := zip.NewWriter(w)
	defer zw.Close()

	usados := map[string]int{}
	for _, d := range docs {
		if len(sel) > 0 && !sel[d.ID] {
			continue
		}
		f, err := os.Open(d.Archivo)
		if err != nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(d.Archivo))
		nombre := sanitizar(d.Nombre) + ext
		usados[nombre]++
		if usados[nombre] > 1 { // evitar colisiones de nombre dentro del zip
			nombre = fmt.Sprintf("%s_%d%s", sanitizar(d.Nombre), usados[nombre]-1, ext)
		}
		fw, err := zw.Create(nombre)
		if err == nil {
			io.Copy(fw, f)
		}
		f.Close()
	}
}

// idsSeleccionados parsea "1,2,3" a un conjunto {id:true}. Vacío = sin filtro.
func idsSeleccionados(s string) map[int64]bool {
	out := map[int64]bool{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			if id, err := strconv.ParseInt(p, 10, 64); err == nil {
				out[id] = true
			}
		}
	}
	return out
}

// sanitizar limpia un texto para usarlo como nombre de archivo.
func sanitizar(s string) string {
	s = strings.TrimSpace(s)
	repl := func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', '\n', '\r', '\t':
			return '_'
		}
		return r
	}
	s = strings.Map(repl, s)
	if s == "" {
		return "documento"
	}
	return s
}
