package backend

import (
	"net/http"
	"strconv"
	"strings"

	"zon-taekwondo/database"
)

// handleGeo devuelve toda la jerarquía geográfica para los selects en cascada.
func (s *Server) handleGeo(w http.ResponseWriter, r *http.Request) {
	estados, err := database.ListEstados(s.db)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	ciudades, _ := database.ListCiudades(s.db)
	municipios, _ := database.ListMunicipios(s.db)
	parroquias, _ := database.ListParroquias(s.db)
	writeJSON(w, http.StatusOK, map[string]any{
		"estados": estados, "ciudades": ciudades,
		"municipios": municipios, "parroquias": parroquias,
	})
}

/* ---------------------- Escuelas ---------------------- */

func (s *Server) handleEscuelas(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		esc, err := database.ListEscuelas(s.db)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, esc)
	case http.MethodPost:
		if !s.soloAdmin(w, r) {
			return
		}
		in, ok := decodeEscuela(w, r)
		if !ok {
			return
		}
		id, err := database.CrearEscuela(s.db, in)
		if err != nil {
			writeErr(w, http.StatusConflict, err.Error())
			return
		}
		s.auditYAvisa(r, "INSERT", "escuela", id)
		writeJSON(w, http.StatusCreated, map[string]any{"id": id})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
	}
}

func (s *Server) handleEscuelaItem(w http.ResponseWriter, r *http.Request) {
	id, _, ok := idFromPath(r.URL.Path, "/api/escuelas/")
	if !ok {
		writeErr(w, http.StatusBadRequest, "id inválido")
		return
	}
	if !s.soloAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodPut:
		in, ok := decodeEscuela(w, r)
		if !ok {
			return
		}
		if err := errNotFound(database.ActualizarEscuela(s.db, id, in)); err != nil {
			writeErr(w, statusFor(err), err.Error())
			return
		}
		s.auditYAvisa(r, "UPDATE", "escuela", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		if err := errNotFound(database.EliminarEscuela(s.db, id)); err != nil {
			writeErr(w, statusFor(err), err.Error())
			return
		}
		s.auditYAvisa(r, "DELETE", "escuela", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
	}
}

func decodeEscuela(w http.ResponseWriter, r *http.Request) (database.EscuelaInput, bool) {
	var in database.EscuelaInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo inválido")
		return in, false
	}
	in.Nombre = strings.TrimSpace(in.Nombre)
	if in.Nombre == "" {
		writeErr(w, http.StatusUnprocessableEntity, "el nombre es obligatorio")
		return in, false
	}
	if in.MunicipioID == 0 {
		writeErr(w, http.StatusUnprocessableEntity, "seleccione el municipio")
		return in, false
	}
	return in, true
}

/* ---------------------- Datos maestros (geografía + cinturones) ---------------------- */

// entrada flexible para el CRUD de datos maestros.
type maestroInput struct {
	Nombre      string `json:"nombre"`
	EstadoID    *int64 `json:"estado_id"`
	CiudadID    *int64 `json:"ciudad_id"`
	MunicipioID *int64 `json:"municipio_id"`
	Color       string `json:"color"`
	Orden       int    `json:"orden"`
	EsNegro     bool   `json:"es_negro"`
}

var tiposMaestros = map[string]bool{
	"estados": true, "ciudades": true, "municipios": true, "parroquias": true, "cinturones": true,
}

// GET /api/maestros/{tipo} (lista) | POST /api/maestros/{tipo} (crear, admin)
func (s *Server) handleMaestros(w http.ResponseWriter, r *http.Request) {
	tipo, id := maestroPath(r.URL.Path)
	if !tiposMaestros[tipo] {
		writeErr(w, http.StatusNotFound, "tipo no válido")
		return
	}
	switch {
	case r.Method == http.MethodGet && id == 0:
		s.listarMaestro(w, tipo)
	case r.Method == http.MethodPost && id == 0:
		s.mutarMaestro(w, r, tipo, 0)
	case r.Method == http.MethodPut && id > 0:
		s.mutarMaestro(w, r, tipo, id)
	case r.Method == http.MethodDelete && id > 0:
		s.borrarMaestro(w, r, tipo, id)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
	}
}

func (s *Server) listarMaestro(w http.ResponseWriter, tipo string) {
	var (
		v   any
		err error
	)
	switch tipo {
	case "estados":
		v, err = database.ListEstados(s.db)
	case "ciudades":
		v, err = database.ListCiudades(s.db)
	case "municipios":
		v, err = database.ListMunicipios(s.db)
	case "parroquias":
		v, err = database.ListParroquias(s.db)
	case "cinturones":
		v, err = database.ListCinturones(s.db)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) mutarMaestro(w http.ResponseWriter, r *http.Request, tipo string, id int64) {
	if !s.soloAdmin(w, r) {
		return
	}
	var in maestroInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}
	in.Nombre = strings.TrimSpace(in.Nombre)
	in.Color = strings.TrimSpace(in.Color)

	var (
		newID int64
		err   error
	)
	switch tipo {
	case "estados":
		if in.Nombre == "" {
			writeErr(w, http.StatusUnprocessableEntity, "el nombre es obligatorio")
			return
		}
		if id == 0 {
			newID, err = database.CrearEstado(s.db, in.Nombre)
		} else {
			err = errNotFound(database.ActualizarEstado(s.db, id, in.Nombre))
		}
	case "ciudades":
		if in.Nombre == "" || in.EstadoID == nil {
			writeErr(w, http.StatusUnprocessableEntity, "nombre y estado son obligatorios")
			return
		}
		if id == 0 {
			newID, err = database.CrearCiudad(s.db, *in.EstadoID, in.Nombre)
		} else {
			err = errNotFound(database.ActualizarCiudad(s.db, id, *in.EstadoID, in.Nombre))
		}
	case "municipios":
		if in.Nombre == "" || (in.EstadoID == nil && in.CiudadID == nil) {
			writeErr(w, http.StatusUnprocessableEntity, "nombre y estado (o ciudad) son obligatorios")
			return
		}
		est := int64(0)
		if in.EstadoID != nil {
			est = *in.EstadoID
		}
		if id == 0 {
			newID, err = database.CrearMunicipio(s.db, est, in.CiudadID, in.Nombre)
		} else {
			err = errNotFound(database.ActualizarMunicipio(s.db, id, est, in.CiudadID, in.Nombre))
		}
	case "parroquias":
		if in.Nombre == "" || in.MunicipioID == nil {
			writeErr(w, http.StatusUnprocessableEntity, "nombre y municipio son obligatorios")
			return
		}
		if id == 0 {
			newID, err = database.CrearParroquia(s.db, *in.MunicipioID, in.Nombre)
		} else {
			err = errNotFound(database.ActualizarParroquia(s.db, id, *in.MunicipioID, in.Nombre))
		}
	case "cinturones":
		if in.Color == "" {
			writeErr(w, http.StatusUnprocessableEntity, "el color es obligatorio")
			return
		}
		if id == 0 {
			newID, err = database.CrearCinturon(s.db, in.Color, in.Orden, in.EsNegro)
		} else {
			err = errNotFound(database.ActualizarCinturon(s.db, id, in.Color, in.Orden, in.EsNegro))
		}
	}
	if err != nil {
		writeErr(w, statusFor(err), err.Error())
		return
	}
	accion := "UPDATE"
	rid := id
	if id == 0 {
		accion, rid = "INSERT", newID
	}
	s.auditYAvisa(r, accion, tipo, rid)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": rid})
}

func (s *Server) borrarMaestro(w http.ResponseWriter, r *http.Request, tipo string, id int64) {
	if !s.soloAdmin(w, r) {
		return
	}
	var err error
	switch tipo {
	case "estados":
		err = database.EliminarEstado(s.db, id)
	case "ciudades":
		err = database.EliminarCiudad(s.db, id)
	case "municipios":
		err = database.EliminarMunicipio(s.db, id)
	case "parroquias":
		err = database.EliminarParroquia(s.db, id)
	case "cinturones":
		err = database.EliminarCinturon(s.db, id)
	}
	if err = errNotFound(err); err != nil {
		writeErr(w, statusFor(err), err.Error())
		return
	}
	s.auditYAvisa(r, "DELETE", tipo, id)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// maestroPath extrae (tipo, id) de /api/maestros/{tipo}[/{id}].
func maestroPath(path string) (string, int64) {
	rest := strings.Trim(strings.TrimPrefix(path, "/api/maestros/"), "/")
	if rest == "" {
		return "", 0
	}
	parts := strings.SplitN(rest, "/", 2)
	var id int64
	if len(parts) == 2 {
		id, _ = strconv.ParseInt(parts[1], 10, 64)
	}
	return parts[0], id
}

// auditYAvisa registra la auditoría y difunde un evento de recarga por WS.
func (s *Server) auditYAvisa(r *http.Request, accion, tabla string, id int64) {
	ses, _ := s.sessions.obtener(r)
	database.Auditar(s.db, ses.EntrenadorID, accion, tabla, id, nil)
	s.hub.Broadcast(Evento{Tipo: "cambio", Recurso: tabla, ID: id, Por: ses.Username})
}
