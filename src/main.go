// Sistema de Gestión de Atletas — Asociación de Taekwondo (Edo. Miranda).
//
// Binario único y portátil: sirve el frontend embebido, usa SQLite CGO-free y
// abre el navegador automáticamente. Punto de entrada de la aplicación.
package main

import (
	"context"
	"embed"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"zon-taekwondo/backend"
	"zon-taekwondo/database"
)

//go:embed all:frontend
var frontendFS embed.FS

//go:embed icon.ico
var iconICO []byte

const (
	addr   = "localhost:8080"
	dbFile = "app.db"
	url    = "http://" + addr
)

// trayConfig configura el icono de la bandeja del sistema. La implementación
// real está en tray_windows.go; en otros sistemas las funciones son no-op.
type trayConfig struct {
	tooltip string
	iconICO []byte
	onOpen  func() // abrir el sistema en el navegador
	onQuit  func() // cerrar el servidor y salir
}

func main() {
	// Logging a consola + log/app.log (para diagnosticar cuando el ejecutable
	// se abre con doble clic y la consola se cierra al instante si falla).
	setupLog()
	log.Printf("==== SIGAT: inicio del servidor ====")

	// En Windows: ocultar la consola propia (doble clic) para quedar solo con
	// el icono de la bandeja. Si se lanzó desde una consola existente, no oculta.
	hideConsoleIfOwned()

	// 1) Base de datos (se crea con esquema + seed si no existe).
	db, err := database.Init(dbFile)
	if err != nil {
		log.Fatalf("no se pudo inicializar la base de datos: %v", err)
	}
	defer db.Close()

	// 2) Frontend embebido (sub-FS de la carpeta frontend/).
	sub, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		log.Fatalf("no se pudo cargar el frontend embebido: %v", err)
	}

	// 3) Enlazar el puerto primero. Si ya está en uso, lo más probable es que el
	// sistema ya esté abierto: se abre el navegador a esa instancia y se sale.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if puertoOcupado(err) {
			log.Printf("el puerto %s ya está en uso; el sistema ya está abierto. Abriendo el navegador…", addr)
			abrirNavegador(url)
		} else {
			log.Printf("no se pudo escuchar en %s: %v", addr, err)
		}
		return
	}

	// 4) Hub de WebSockets + servidor HTTP.
	hub := backend.NewHub()
	srv := backend.NewServer(db, hub, sub, dbFile)
	httpSrv := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// 5) Señal de apagado unificada: navegador cerrado, "Salir" en la bandeja,
	// o error del servidor. Se dispara una sola vez.
	var once sync.Once
	shutdown := make(chan struct{})
	trigger := func() { once.Do(func() { close(shutdown) }) }

	// Apagado automático cuando se cierra el navegador (todos los WS caen y no
	// vuelven dentro del periodo de gracia).
	hub.SetOnIdle(func() {
		log.Printf("navegador cerrado: apagando el servidor…")
		trigger()
	})

	// 6) Servir en segundo plano.
	go func() {
		log.Printf("iniciando servidor en %s", url)
		if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("servidor detenido: %v", err)
			trigger()
		}
	}()

	// 7) Abrir el navegador cuando el servidor responda.
	go func() {
		esperarListo(url)
		log.Printf("servidor listo en %s — abriendo navegador…", url)
		abrirNavegador(url)
	}()

	// 8) Icono en la bandeja del sistema (Windows). Un clic abre el menú con
	// "Abrir sistema" y "Salir".
	runTray(trayConfig{
		tooltip: "SIGAT — Sistema de Gestión de Atletas",
		iconICO: iconICO,
		onOpen:  func() { abrirNavegador(url) },
		onQuit:  func() { log.Printf("salida solicitada desde la bandeja"); trigger() },
	})

	// 9) Esperar la señal y apagar ordenadamente.
	<-shutdown
	log.Printf("cerrando…")
	stopTray()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Printf("apagado forzado: %v", err)
	}
	log.Printf("==== SIGAT: servidor detenido ====")
}

// puertoOcupado indica si el fallo de arranque se debe a que el puerto ya está
// en uso (otra instancia abierta). Extrae el errno del sistema y compara contra
// EADDRINUSE (POSIX) y WSAEADDRINUSE = 10048 (Windows), sin depender del texto
// del mensaje (que varía según el idioma del sistema).
func puertoOcupado(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.EADDRINUSE || errno == 10048
	}
	return false
}

// setupLog envía los logs a la consola y también a log/app.log, para poder
// revisar los errores de arranque cuando el ejecutable se abre con doble clic
// (si el arranque falla, la ventana de consola se cierra al instante). El
// archivo se escribe primero para que quede registro aunque no haya consola.
func setupLog() {
	log.SetFlags(log.Ldate | log.Ltime)
	if err := os.MkdirAll("log", 0o755); err != nil {
		log.Printf("no se pudo crear la carpeta log/: %v", err)
		return
	}
	f, err := os.OpenFile(filepath.Join("log", "app.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("no se pudo abrir log/app.log: %v", err)
		return
	}
	log.SetOutput(io.MultiWriter(f, os.Stderr))
}

// esperarListo hace polling al endpoint hasta que responda, antes de abrir el
// navegador, para evitar una pestaña con "conexión rechazada".
func esperarListo(url string) {
	client := http.Client{Timeout: 300 * time.Millisecond}
	for i := 0; i < 50; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := client.Do(req)
		cancel()
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// abrirNavegador abre la URL en el navegador predeterminado según el SO.
func abrirNavegador(url string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default: // linux, *bsd…
		err = exec.Command("xdg-open", url).Start()
	}
	if err != nil {
		log.Printf("no se pudo abrir el navegador automáticamente (%v). Abra %s manualmente.", err, url)
	}
}
