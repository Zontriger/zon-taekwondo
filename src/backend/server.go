package backend

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Server agrupa las dependencias compartidas por los handlers HTTP/WS.
type Server struct {
	db       *sql.DB
	hub      *Hub
	sessions *sessionStore
	frontend fs.FS
	dbPath   string // ruta del archivo .db (para respaldo)
}

// NewServer construye el servidor con la conexión, el hub WS y el FS embebido.
func NewServer(db *sql.DB, hub *Hub, frontend fs.FS, dbPath string) *Server {
	return &Server{
		db:       db,
		hub:      hub,
		sessions: newSessionStore(),
		frontend: frontend,
		dbPath:   dbPath,
	}
}

// Handler arma el enrutador con las rutas de API, WS y estáticos embebidos.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// --- Autenticación ---
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/me", s.requireAuth(s.handleMe))
	mux.HandleFunc("/api/profile", s.requireAuth(s.handlePerfil))

	// --- Catálogos (para selectores del formulario) ---
	mux.HandleFunc("/api/catalogos", s.requireAuth(s.handleCatalogos))
	mux.HandleFunc("/api/geo", s.requireAuth(s.handleGeo))

	// --- Configuración del sistema (GET todos; PUT solo admin) ---
	mux.HandleFunc("/api/config", s.requireAuth(s.handleConfig))

	// --- Atletas ---
	mux.HandleFunc("/api/atletas", s.requireAuth(s.handleAtletas))     // GET lista, POST crea
	mux.HandleFunc("/api/atletas/", s.requireAuth(s.handleAtletaItem)) // /{id} y subrutas

	// --- Escuelas y datos maestros ---
	mux.HandleFunc("/api/escuelas", s.requireAuth(s.handleEscuelas))
	mux.HandleFunc("/api/escuelas/", s.requireAuth(s.handleEscuelaItem))
	mux.HandleFunc("/api/maestros/", s.requireAuth(s.handleMaestros))
	mux.HandleFunc("/api/entrenadores", s.requireAuth(s.handleEntrenadores))
	mux.HandleFunc("/api/entrenadores/", s.requireAuth(s.handleEntrenadorItem))

	// --- Usuarios del sistema (solo admin, verificado en el handler) ---
	mux.HandleFunc("/api/usuarios", s.requireAuth(s.handleUsuarios))
	mux.HandleFunc("/api/usuarios/", s.requireAuth(s.handleUsuarioItem))

	// --- Reportes PDF ---
	mux.HandleFunc("/api/reportes/atletas.pdf", s.requireAuth(s.handleReporteAtletas))
	mux.HandleFunc("/api/reportes/planilla-blanco.pdf", s.requireAuth(s.handlePlanillaBlanco))

	// --- Respaldo import/export (solo admin, verificado en el handler) ---
	mux.HandleFunc("/api/backup/", s.requireAuth(s.handleBackup))

	// --- WebSocket (requiere sesión) ---
	mux.HandleFunc("/ws", s.requireAuth(s.hub.handleWS))

	// --- Frontend embebido ---
	mux.Handle("/", http.FileServer(http.FS(s.frontend)))

	return logRequests(mux)
}

// ---- Middleware -----------------------------------------------------------

// requireAuth exige una sesión válida y coloca la sesión en el contexto vía
// parámetro implícito: los handlers la recuperan con s.sessions.obtener(r).
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.sessions.obtener(r); !ok {
			writeErr(w, http.StatusUnauthorized, "no autenticado")
			return
		}
		next(w, r)
	}
}

// soloAdmin escribe 403 y devuelve false si la sesión no es de administrador.
// Centraliza el enforcement: el rol Consultor es de solo lectura.
func (s *Server) soloAdmin(w http.ResponseWriter, r *http.Request) bool {
	ses, ok := s.sessions.obtener(r)
	if !ok || !ses.EsAdmin {
		writeErr(w, http.StatusForbidden, "requiere rol administrador")
		return false
	}
	return true
}

// errNotFound normaliza sql.ErrNoRows (no cambia otros errores).
func errNotFound(err error) error { return err }

// statusFor mapea un error de dominio a un código HTTP.
func statusFor(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, sql.ErrNoRows):
		return http.StatusNotFound
	default:
		return http.StatusConflict
	}
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api") || r.URL.Path == "/ws" {
			log.Printf("%s %s", r.Method, r.URL.Path)
		}
	})
}

// ---- Helpers JSON ---------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// setDisposition fija Content-Disposition con un nombre ASCII de respaldo y el
// nombre real en UTF-8 (RFC 5987), de modo que tildes y ñ se conserven en la
// descarga (p. ej. "planilla-peña_alejandro.pdf").
func setDisposition(w http.ResponseWriter, disp, filename string) {
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, disp, translitASCII(filename), url.PathEscape(filename)))
}

func setAttachment(w http.ResponseWriter, filename string) { setDisposition(w, "attachment", filename) }

// translitASCII produce una versión solo-ASCII (tildes/ñ → letra base) para el
// nombre de archivo de respaldo que exige la cabecera HTTP.
func translitASCII(s string) string {
	rep := map[rune]string{
		'á': "a", 'é': "e", 'í': "i", 'ó': "o", 'ú': "u", 'ü': "u", 'ñ': "n",
		'Á': "A", 'É': "E", 'Í': "I", 'Ó': "O", 'Ú': "U", 'Ü': "U", 'Ñ': "N",
	}
	var b strings.Builder
	for _, r := range s {
		if v, ok := rep[r]; ok {
			b.WriteString(v)
		} else if r < 128 {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

// decode lee el cuerpo JSON de la petición en dst.
func decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// idFromPath extrae el {id} y la posible subruta de /api/atletas/{id}[/sub].
// Devuelve (id, subruta, ok).
func idFromPath(path, prefix string) (int64, string, bool) {
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return 0, "", false
	}
	parts := strings.SplitN(rest, "/", 2)
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", false
	}
	sub := ""
	if len(parts) == 2 {
		sub = parts[1]
	}
	return id, sub, true
}
