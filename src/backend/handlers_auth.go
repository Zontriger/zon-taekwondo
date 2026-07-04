package backend

import (
	"errors"
	"net/http"
	"strings"

	"zon-taekwondo/database"
)

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}
	ent, err := database.Autenticar(s.db, strings.TrimSpace(in.Username), in.Password)
	if errors.Is(err, database.ErrCredenciales) {
		writeErr(w, http.StatusUnauthorized, "usuario o contraseña inválidos")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error de autenticación")
		return
	}
	s.sessions.crear(w, sesion{
		EntrenadorID: ent.ID,
		Username:     ent.Username,
		EsAdmin:      ent.EsAdmin,
	})
	writeJSON(w, http.StatusOK, ent)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.sessions.destruir(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	ses, _ := s.sessions.obtener(r)
	ent, err := database.GetEntrenador(s.db, ses.EntrenadorID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "no se pudo cargar el perfil")
		return
	}
	writeJSON(w, http.StatusOK, ent)
}

// handlePerfil actualiza el username y/o la contraseña del entrenador en sesión.
// Devuelve errores por campo (mapa) con 422 para que el formulario los resalte.
func (s *Server) handlePerfil(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	ses, _ := s.sessions.obtener(r)
	var in struct {
		Username *string `json:"username"`
		Password *string `json:"password"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}

	fieldErrs := map[string]string{}

	if in.Username != nil {
		u := strings.TrimSpace(*in.Username)
		if len(u) < 3 {
			fieldErrs["username"] = "Mínimo 3 caracteres"
		} else if err := database.ActualizarUsername(s.db, ses.EntrenadorID, u); err != nil {
			if errors.Is(err, database.ErrUsernameTaken) {
				fieldErrs["username"] = err.Error()
			} else {
				writeErr(w, http.StatusInternalServerError, "no se pudo actualizar el usuario")
				return
			}
		} else {
			s.sessions.setUsername(r, u)
		}
	}

	if in.Password != nil {
		if msg := passwordDebil(*in.Password); msg != "" {
			fieldErrs["password"] = msg
		} else if err := database.CambiarPassword(s.db, ses.EntrenadorID, *in.Password); err != nil {
			writeErr(w, http.StatusInternalServerError, "no se pudo actualizar la contraseña")
			return
		}
	}

	if len(fieldErrs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"errors": fieldErrs})
		return
	}
	ent, _ := database.GetEntrenador(s.db, ses.EntrenadorID)
	writeJSON(w, http.StatusOK, ent)
}

// passwordDebil devuelve un mensaje si la contraseña no cumple la política
// (8+ caracteres, con mayúscula, minúscula y dígito); "" si es válida.
func passwordDebil(p string) string {
	if len([]rune(p)) < 8 {
		return "Mínimo 8 caracteres"
	}
	var lower, upper, digit bool
	for _, c := range p {
		switch {
		case c >= 'a' && c <= 'z':
			lower = true
		case c >= 'A' && c <= 'Z':
			upper = true
		case c >= '0' && c <= '9':
			digit = true
		}
	}
	if !lower || !upper || !digit {
		return "Debe incluir mayúscula, minúscula y número"
	}
	return ""
}

func (s *Server) handleCatalogos(w http.ResponseWriter, r *http.Request) {
	cinturones, err := database.ListCinturones(s.db)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	escuelas, err := database.ListEscuelas(s.db)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	maestros, err := database.ListMaestros(s.db)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"cinturones":    cinturones,
		"escuelas":      escuelas,
		"maestros":      maestros,
		"campos_filtro": database.FilterFields(),
	})
}
