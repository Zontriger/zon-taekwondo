package backend

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

const (
	cookieName    = "zon_sid"
	sessionMaxAge = 8 * time.Hour
)

// Sesión de un entrenador autenticado.
type sesion struct {
	EntrenadorID int64
	Username     string
	EsAdmin      bool
	expira       time.Time
}

// sessionStore es un almacén de sesiones en memoria protegido por mutex.
// Al ser un binario de un solo proceso, no se requiere backend externo.
type sessionStore struct {
	mu   sync.RWMutex
	data map[string]sesion
}

func newSessionStore() *sessionStore {
	s := &sessionStore{data: make(map[string]sesion)}
	go s.gc()
	return s
}

// gc elimina periódicamente las sesiones expiradas.
func (s *sessionStore) gc() {
	for range time.Tick(15 * time.Minute) {
		now := time.Now()
		s.mu.Lock()
		for k, v := range s.data {
			if now.After(v.expira) {
				delete(s.data, k)
			}
		}
		s.mu.Unlock()
	}
}

func (s *sessionStore) crear(w http.ResponseWriter, ses sesion) {
	token := nuevoToken()
	ses.expira = time.Now().Add(sessionMaxAge)
	s.mu.Lock()
	s.data[token] = ses
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionMaxAge.Seconds()),
	})
}

// obtener devuelve la sesión asociada a la petición, si es válida.
func (s *sessionStore) obtener(r *http.Request) (sesion, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return sesion{}, false
	}
	s.mu.RLock()
	ses, ok := s.data[c.Value]
	s.mu.RUnlock()
	if !ok || time.Now().After(ses.expira) {
		return sesion{}, false
	}
	return ses, true
}

// setUsername actualiza el username en la sesión activa (tras renombrarse).
func (s *sessionStore) setUsername(r *http.Request, username string) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return
	}
	s.mu.Lock()
	if ses, ok := s.data[c.Value]; ok {
		ses.Username = username
		s.data[c.Value] = ses
	}
	s.mu.Unlock()
}

func (s *sessionStore) destruir(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		s.mu.Lock()
		delete(s.data, c.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: "", Path: "/", HttpOnly: true, MaxAge: -1,
	})
}

func nuevoToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
