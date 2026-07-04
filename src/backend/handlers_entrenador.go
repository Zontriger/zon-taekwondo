package backend

import (
	"net/http"
	"strings"

	"zon-taekwondo/database"
)

// GET/POST /api/entrenadores  (entrenadores deportivos / maestros)
func (s *Server) handleEntrenadores(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ms, err := database.ListMaestros(s.db)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ms)
	case http.MethodPost:
		if !s.soloAdmin(w, r) {
			return
		}
		in, ok := s.decodeMaestro(w, r)
		if !ok {
			return
		}
		id, err := database.CrearMaestro(s.db, in)
		if err != nil {
			writeErr(w, http.StatusConflict, err.Error())
			return
		}
		s.auditYAvisa(r, "INSERT", "maestro", id)
		writeJSON(w, http.StatusCreated, map[string]any{"id": id})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
	}
}

// PUT/DELETE /api/entrenadores/{id}
func (s *Server) handleEntrenadorItem(w http.ResponseWriter, r *http.Request) {
	id, _, ok := idFromPath(r.URL.Path, "/api/entrenadores/")
	if !ok {
		writeErr(w, http.StatusBadRequest, "id inválido")
		return
	}
	if !s.soloAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodPut:
		in, ok := s.decodeMaestro(w, r)
		if !ok {
			return
		}
		if err := errNotFound(database.ActualizarMaestro(s.db, id, in)); err != nil {
			writeErr(w, statusFor(err), err.Error())
			return
		}
		s.auditYAvisa(r, "UPDATE", "maestro", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		if err := errNotFound(database.EliminarMaestro(s.db, id)); err != nil {
			writeErr(w, statusFor(err), err.Error())
			return
		}
		s.auditYAvisa(r, "DELETE", "maestro", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
	}
}

func (s *Server) decodeMaestro(w http.ResponseWriter, r *http.Request) (database.MaestroInput, bool) {
	var in database.MaestroInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo inválido")
		return in, false
	}
	errs := map[string]string{}
	in.Nombres = strings.TrimSpace(in.Nombres)
	in.Apellidos = strings.TrimSpace(in.Apellidos)
	if in.Nombres == "" {
		errs["nombres"] = "Requerido"
	}
	if in.Apellidos == "" {
		errs["apellidos"] = "Requerido"
	}
	validarCedula(trimPtr(in.CedulaTipo), trimPtr(in.CedulaNumero), &in.CedulaTipo, &in.CedulaNumero, errs, "cedula_tipo", "cedula_numero")
	if in.CinturonID != nil {
		if err := s.validarDan(*in.CinturonID, &in.Dan); err != nil {
			errs["dan"] = err.Error()
		}
	} else {
		in.Dan = nil
	}
	if len(errs) > 0 {
		writeFieldErrs(w, errs)
		return in, false
	}
	return in, true
}
