package backend

import (
	"net/http"
	"strconv"

	"zon-taekwondo/database"
)

// handleConfig gestiona los ajustes del sistema.
//   GET /api/config  → cualquier usuario autenticado (para validar subidas).
//   PUT /api/config  → solo administrador.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, _ := database.GetConfig(s.db)
		writeJSON(w, http.StatusOK, map[string]any{
			"max_upload_mb":   database.MaxUploadMB(s.db),
			"ultimo_respaldo": cfg["ultimo_respaldo"],
		})
	case http.MethodPut:
		if !s.soloAdmin(w, r) {
			return
		}
		var in struct {
			MaxUploadMB *int `json:"max_upload_mb"`
		}
		if err := decode(r, &in); err != nil {
			writeErr(w, http.StatusBadRequest, "cuerpo inválido")
			return
		}
		if in.MaxUploadMB != nil {
			if *in.MaxUploadMB < 1 || *in.MaxUploadMB > 100 {
				writeFieldErrs(w, map[string]string{"max_upload_mb": "Debe estar entre 1 y 100 MB"})
				return
			}
			if err := database.SetConfig(s.db, "max_upload_mb", strconv.Itoa(*in.MaxUploadMB)); err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		ses, _ := s.sessions.obtener(r)
		s.hub.Broadcast(Evento{Tipo: "config.actualizada", Recurso: "config", Por: ses.Username})
		writeJSON(w, http.StatusOK, map[string]any{"max_upload_mb": database.MaxUploadMB(s.db)})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
	}
}
