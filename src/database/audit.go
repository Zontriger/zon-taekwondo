package database

import (
	"database/sql"
	"encoding/json"
	"log"
)

// Auditar registra un cambio en la tabla auditoria. Nunca hace fallar la
// operación principal: si la traza falla, solo se registra en el log.
// entrenadorID = 0 se guarda como NULL (acción del sistema).
func Auditar(execer execer, entrenadorID int64, accion, tabla string, registroID int64, detalle any) {
	var det sql.NullString
	if detalle != nil {
		if b, err := json.Marshal(detalle); err == nil {
			det = sql.NullString{String: string(b), Valid: true}
		}
	}
	var ent sql.NullInt64
	if entrenadorID > 0 {
		ent = sql.NullInt64{Int64: entrenadorID, Valid: true}
	}
	var reg sql.NullInt64
	if registroID > 0 {
		reg = sql.NullInt64{Int64: registroID, Valid: true}
	}
	_, err := execer.Exec(
		`INSERT INTO auditoria (entrenador_id, accion, tabla, registro_id, detalle)
		 VALUES (?, ?, ?, ?, ?)`,
		ent, accion, tabla, reg, det,
	)
	if err != nil {
		log.Printf("[audit] no se pudo registrar %s/%s: %v", accion, tabla, err)
	}
}

// execer abstrae *sql.DB y *sql.Tx para poder auditar dentro o fuera de una tx.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}
