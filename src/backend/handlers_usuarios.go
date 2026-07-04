package backend

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"zon-taekwondo/database"
)

// GET/POST /api/usuarios  (solo administrador)
func (s *Server) handleUsuarios(w http.ResponseWriter, r *http.Request) {
	if !s.soloAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		us, err := database.ListEntrenadores(s.db)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, us)
	case http.MethodPost:
		s.crearUsuario(w, r)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
	}
}

// /api/usuarios/{id}  (PUT/DELETE, solo administrador)
func (s *Server) handleUsuarioItem(w http.ResponseWriter, r *http.Request) {
	if !s.soloAdmin(w, r) {
		return
	}
	id, _, ok := idFromPath(r.URL.Path, "/api/usuarios/")
	if !ok {
		writeErr(w, http.StatusBadRequest, "id inválido")
		return
	}
	switch r.Method {
	case http.MethodPut:
		s.actualizarUsuario(w, r, id)
	case http.MethodDelete:
		s.eliminarUsuario(w, r, id)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
	}
}

func (s *Server) crearUsuario(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeUsuario(w, r)
	if !ok {
		return
	}
	errs := validarUsuario(in, true)
	if len(errs) > 0 {
		writeFieldErrs(w, errs)
		return
	}
	id, err := database.CrearEntrenador(s.db, in)
	if errors.Is(err, database.ErrUsernameTaken) {
		writeFieldErrs(w, map[string]string{"username": err.Error()})
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.auditYAvisa(r, "INSERT", "usuario", id)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (s *Server) actualizarUsuario(w http.ResponseWriter, r *http.Request, id int64) {
	in, ok := decodeUsuario(w, r)
	if !ok {
		return
	}
	errs := validarUsuario(in, false)
	if len(errs) > 0 {
		writeFieldErrs(w, errs)
		return
	}
	// Salvaguarda: no dejar el sistema sin administradores activos.
	actual, err := database.GetEntrenador(s.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "usuario no encontrado")
		return
	}
	quedaSinAdmin := actual.EsAdmin && actual.Estado == "activo" &&
		(!in.EsAdmin || in.Estado == "retirado") &&
		database.ContarAdminsActivos(s.db, id) == 0
	if quedaSinAdmin {
		writeErr(w, http.StatusConflict, "debe existir al menos un administrador activo")
		return
	}
	if err := errNotFound(database.ActualizarEntrenador(s.db, id, in)); err != nil {
		if errors.Is(err, database.ErrUsernameTaken) {
			writeFieldErrs(w, map[string]string{"username": err.Error()})
			return
		}
		writeErr(w, statusFor(err), err.Error())
		return
	}
	// Si se renombró al propio usuario en sesión, refrescar la cookie.
	if ses, _ := s.sessions.obtener(r); ses.EntrenadorID == id {
		s.sessions.setUsername(r, in.Username)
	}
	s.auditYAvisa(r, "UPDATE", "usuario", id)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) eliminarUsuario(w http.ResponseWriter, r *http.Request, id int64) {
	if id == database.IDAdminOrigen(s.db) {
		writeErr(w, http.StatusForbidden, "no se puede eliminar al administrador de origen")
		return
	}
	actual, err := database.GetEntrenador(s.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "usuario no encontrado")
		return
	}
	if actual.EsAdmin && actual.Estado == "activo" && database.ContarAdminsActivos(s.db, id) == 0 {
		writeErr(w, http.StatusConflict, "no puede eliminar al último administrador activo")
		return
	}
	if err := errNotFound(database.EliminarEntrenador(s.db, id)); err != nil {
		writeErr(w, statusFor(err), err.Error())
		return
	}
	s.auditYAvisa(r, "DELETE", "usuario", id)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func decodeUsuario(w http.ResponseWriter, r *http.Request) (database.UsuarioInput, bool) {
	var in database.UsuarioInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo inválido")
		return in, false
	}
	in.Username = strings.TrimSpace(in.Username)
	in.Nombres = strings.TrimSpace(in.Nombres)
	in.Apellidos = strings.TrimSpace(in.Apellidos)
	return in, true
}

// validarUsuario devuelve errores por campo. crear=true exige contraseña.
func validarUsuario(in database.UsuarioInput, crear bool) map[string]string {
	errs := map[string]string{}
	if len([]rune(in.Username)) < 3 {
		errs["username"] = "Mínimo 3 caracteres"
	}
	if in.Nombres == "" {
		errs["nombres"] = "Requerido"
	}
	if in.Estado != "" && in.Estado != "activo" && in.Estado != "retirado" {
		errs["estado"] = "Estado inválido"
	}
	tienePass := in.Password != nil && *in.Password != ""
	if crear && !tienePass {
		errs["password"] = "La contraseña es obligatoria"
	} else if tienePass {
		if msg := passwordDebil(*in.Password); msg != "" {
			errs["password"] = msg
		}
	}
	return errs
}
