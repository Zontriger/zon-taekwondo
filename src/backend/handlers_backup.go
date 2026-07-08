package backend

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"zon-taekwondo/database"
)

// Tablas permitidas para exportar/importar por CSV (lista blanca).
var tablasRespaldo = map[string]bool{
	"estado": true, "ciudad": true, "municipio": true, "parroquia": true,
	"cinturon": true, "escuela": true, "entrenador": true,
	"atleta": true, "representante": true, "atleta_telefono_contacto": true,
	"historial_cinturon": true, "periodo_actividad": true,
}

// handleBackup enruta las operaciones de respaldo (todas solo admin).
//   GET  /api/backup/db                    → descarga la BD completa
//   GET  /api/backup/tabla/{nombre}.csv    → exporta una tabla a CSV
//   POST /api/backup/tabla/{nombre}        → importa (reemplaza) una tabla desde CSV
func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	if !s.soloAdmin(w, r) {
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/backup/")
	switch {
	case rest == "db" && r.Method == http.MethodGet:
		s.descargarDB(w)
	case rest == "full" && r.Method == http.MethodGet:
		s.descargarRespaldoCompleto(w)
	case strings.HasPrefix(rest, "tabla/"):
		nombre := strings.TrimPrefix(rest, "tabla/")
		nombre = strings.TrimSuffix(nombre, ".csv")
		if !tablasRespaldo[nombre] {
			writeErr(w, http.StatusNotFound, "tabla no permitida")
			return
		}
		switch r.Method {
		case http.MethodGet:
			s.exportarTabla(w, nombre)
		case http.MethodPost:
			s.importarTabla(w, r, nombre)
		default:
			writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		}
	default:
		writeErr(w, http.StatusNotFound, "recurso no encontrado")
	}
}

// registrarRespaldo guarda la fecha/hora del último respaldo descargado.
func (s *Server) registrarRespaldo() {
	database.SetConfig(s.db, "ultimo_respaldo", time.Now().Format(time.RFC3339))
}

func (s *Server) descargarDB(w http.ResponseWriter) {
	s.registrarRespaldo()
	// Consolidar el WAL en el archivo principal antes de servirlo.
	s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	f, err := os.Open(s.dbPath)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "no se pudo abrir la base de datos")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="respaldo.db"`)
	if _, err := io.Copy(w, f); err != nil {
		fmt.Printf("[backup] error enviando BD: %v\n", err)
	}
}

// descargarRespaldoCompleto empaqueta en un .zip la base de datos (app.db) junto
// con toda la carpeta data/ (fotos y documentos), de modo que un solo archivo
// contenga TODO el sistema para respaldar o migrar.
func (s *Server) descargarRespaldoCompleto(w http.ResponseWriter) {
	s.registrarRespaldo()
	// Consolidar el WAL en el archivo principal antes de empaquetarlo.
	s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="respaldo-completo-%s.zip"`, time.Now().Format("2006-01-02")))
	zw := zip.NewWriter(w)
	defer zw.Close()

	// 1) La base de datos, en la raíz del zip.
	if f, err := os.Open(s.dbPath); err == nil {
		if fw, e := zw.Create("app.db"); e == nil {
			io.Copy(fw, f)
		}
		f.Close()
	}

	// 2) Toda la carpeta data/ (fotos y documentos), conservando su estructura.
	filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(".", path)
		if e != nil {
			rel = path
		}
		nombre := filepath.ToSlash(rel) // rutas con '/' dentro del zip
		fw, e := zw.Create(nombre)
		if e != nil {
			return nil
		}
		f, e := os.Open(path)
		if e != nil {
			return nil
		}
		io.Copy(fw, f)
		f.Close()
		return nil
	})
}

func (s *Server) exportarTabla(w http.ResponseWriter, tabla string) {
	rows, err := s.db.Query(`SELECT * FROM ` + tabla)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	cols, _ := rows.Columns()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, tabla))
	cw := csv.NewWriter(w)
	defer cw.Flush()
	cw.Write(cols)

	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return
		}
		rec := make([]string, len(cols))
		for i, v := range vals {
			rec[i] = celdaTexto(v)
		}
		cw.Write(rec)
	}
}

func (s *Server) importarTabla(w http.ResponseWriter, r *http.Request, tabla string) {
	cr := csv.NewReader(r.Body)
	registros, err := cr.ReadAll()
	if err != nil {
		writeErr(w, http.StatusBadRequest, "CSV inválido: "+err.Error())
		return
	}
	if len(registros) == 0 {
		writeErr(w, http.StatusUnprocessableEntity, "el CSV está vacío")
		return
	}
	header := registros[0]
	// Validar que las columnas del CSV existan en la tabla.
	validas := map[string]bool{}
	colRows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tabla))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	for colRows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt any
		colRows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		validas[name] = true
	}
	colRows.Close()
	for _, h := range header {
		if !validas[h] {
			writeErr(w, http.StatusUnprocessableEntity, "columna desconocida en el CSV: "+h)
			return
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM ` + tabla); err != nil {
		writeErr(w, http.StatusConflict, "no se pudo vaciar la tabla: "+err.Error())
		return
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(header)), ",")
	stmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tabla, strings.Join(header, ","), placeholders)
	insertadas := 0
	for i, fila := range registros[1:] {
		if len(fila) != len(header) {
			writeErr(w, http.StatusUnprocessableEntity, fmt.Sprintf("fila %d: número de columnas incorrecto", i+2))
			return
		}
		args := make([]any, len(fila))
		for j, v := range fila {
			if v == "" {
				args[j] = nil
			} else {
				args[j] = v
			}
		}
		if _, err := tx.Exec(stmt, args...); err != nil {
			writeErr(w, http.StatusConflict, fmt.Sprintf("fila %d: %v", i+2, err))
			return
		}
		insertadas++
	}
	if err := tx.Commit(); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	ses, _ := s.sessions.obtener(r)
	database.Auditar(s.db, ses.EntrenadorID, "IMPORT", tabla, 0, map[string]int{"filas": insertadas})
	s.hub.Broadcast(Evento{Tipo: "cambio", Recurso: tabla, Por: ses.Username})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "insertadas": insertadas})
}

func celdaTexto(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(t)
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}
