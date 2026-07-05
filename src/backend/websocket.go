package backend

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Evento difundido a los clientes conectados cuando cambia un recurso, para que
// las listas abiertas en otros navegadores se refresquen en vivo. No es un flujo
// de alta frecuencia: solo se emite ante acciones concretas del usuario.
type Evento struct {
	Tipo    string `json:"tipo"`              // p.ej. "atleta.creado", "atleta.actualizado"
	Recurso string `json:"recurso"`           // "atleta"
	ID      int64  `json:"id,omitempty"`      // id afectado
	Por     string `json:"por,omitempty"`     // username que originó el cambio
	TS      int64  `json:"ts"`                // epoch ms
}

// Hub mantiene el conjunto de clientes WS y difunde eventos a todos. También
// detecta cuándo se cierra el navegador (todos los clientes se desconectan y no
// vuelven) para poder apagar el servidor automáticamente.
type Hub struct {
	mu            sync.RWMutex
	clients       map[*client]struct{}
	everConnected bool         // hubo al menos un cliente
	idleTimer     *time.Timer  // cuenta regresiva de apagado cuando no queda nadie
	idleGrace     time.Duration
	onIdle        func()
}

type client struct {
	conn *websocket.Conn
	send chan Evento
}

func NewHub() *Hub {
	// La gracia debe superar el reintento de reconexión del frontend (3s) para
	// no apagar el servidor ante una simple recarga de la página.
	return &Hub{clients: make(map[*client]struct{}), idleGrace: 8 * time.Second}
}

// SetOnIdle registra la acción a ejecutar cuando, tras haber tenido clientes,
// todos se desconectan y no regresan dentro de idleGrace (navegador cerrado).
func (h *Hub) SetOnIdle(fn func()) {
	h.mu.Lock()
	h.onIdle = fn
	h.mu.Unlock()
}

// Broadcast encola un evento para todos los clientes conectados.
func (h *Hub) Broadcast(ev Evento) {
	ev.TS = time.Now().UnixMilli()
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- ev:
		default: // cliente lento: se descarta el evento para no bloquear
		}
	}
}

func (h *Hub) add(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.everConnected = true
	if h.idleTimer != nil { // llegó un cliente: cancelar apagado pendiente
		h.idleTimer.Stop()
		h.idleTimer = nil
	}
	h.mu.Unlock()
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	// Si no queda nadie (y hubo alguien) arrancar la cuenta regresiva de apagado.
	if len(h.clients) == 0 && h.everConnected && h.onIdle != nil {
		if h.idleTimer != nil {
			h.idleTimer.Stop()
		}
		fn := h.onIdle
		h.idleTimer = time.AfterFunc(h.idleGrace, func() {
			h.mu.RLock()
			vacio := len(h.clients) == 0
			h.mu.RUnlock()
			if vacio {
				fn()
			}
		})
	}
	h.mu.Unlock()
}

var upgrader = websocket.Upgrader{
	// Un solo origen (localhost); se acepta la conexión local.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// handleWS maneja una conexión entrante ya autenticada.
func (h *Hub) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] upgrade falló: %v", err)
		return
	}
	c := &client{conn: conn, send: make(chan Evento, 16)}
	h.add(c)

	go c.writePump()
	c.readPump(h) // bloquea hasta que el cliente se desconecta
}

// readPump descarta los mensajes entrantes (canal solo de salida) y detecta
// el cierre de la conexión para limpiar el cliente.
func (c *client) readPump(h *Hub) {
	defer func() {
		h.remove(c)
		c.conn.Close()
	}()
	c.conn.SetReadLimit(512)
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *client) writePump() {
	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	for {
		select {
		case ev, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			b, _ := json.Marshal(ev)
			if err := c.conn.WriteMessage(websocket.TextMessage, b); err != nil {
				return
			}
		case <-ping.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
