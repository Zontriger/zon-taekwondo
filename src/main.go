// Sistema de Gestión de Atletas — Asociación de Taekwondo (Edo. Miranda).
//
// Binario único y portátil: sirve el frontend embebido, usa SQLite CGO-free y
// abre el navegador automáticamente. Punto de entrada de la aplicación.
package main

import (
	"embed"
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"zon-taekwondo/backend"
	"zon-taekwondo/database"
)

//go:embed all:frontend
var frontendFS embed.FS

const (
	addr   = "localhost:8080"
	dbFile = "app.db"
)

func main() {
	log.SetFlags(log.Ltime)

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

	// 3) Hub de WebSockets + servidor HTTP.
	hub := backend.NewHub()
	srv := backend.NewServer(db, hub, sub, dbFile)

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// 4) Abrir el navegador cuando el servidor ya esté escuchando.
	go func() {
		url := "http://" + addr
		esperarListo(url)
		log.Printf("servidor listo en %s — abriendo navegador…", url)
		abrirNavegador(url)
	}()

	log.Printf("iniciando servidor en http://%s", addr)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("servidor detenido: %v", err)
	}
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
