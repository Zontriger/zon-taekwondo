package backend

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"zon-taekwondo/database"
)

// GET /api/atletas  (lista con facetas)  |  POST /api/atletas (crear)
func (s *Server) handleAtletas(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listarAtletas(w, r)
	case http.MethodPost:
		s.crearAtleta(w, r)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
	}
}

func (s *Server) listarAtletas(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := database.AtletaFiltro{
		Texto:  q.Get("q"),
		Limit:  int(atoi64(q.Get("limit"))),
		Offset: int(atoi64(q.Get("offset"))),
	}
	// Filtro avanzado: dominio JSON codificado en el parámetro 'domain'.
	if d := q.Get("domain"); strings.TrimSpace(d) != "" {
		var dom database.Dominio
		if err := json.Unmarshal([]byte(d), &dom); err != nil {
			writeErr(w, http.StatusBadRequest, "filtro avanzado inválido")
			return
		}
		f.Dominio = &dom
	}
	items, total, err := database.ListAtletas(s.db, f)
	if err != nil {
		// Errores de dominio (campo/operador/valor inválido) → 422 legible.
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if items == nil {
		items = []database.Atleta{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (s *Server) crearAtleta(w http.ResponseWriter, r *http.Request) {
	if !s.soloAdmin(w, r) {
		return
	}
	ses, _ := s.sessions.obtener(r)
	var in database.AtletaInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	if errs := s.validarAtleta(&in); len(errs) > 0 {
		writeFieldErrs(w, errs)
		return
	}
	id, err := database.CreateAtleta(s.db, in, ses.EntrenadorID)
	if errors.Is(err, database.ErrCedulaDuplicada) {
		writeFieldErrs(w, map[string]string{"cedula_numero": err.Error()})
		return
	}
	if err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	s.hub.Broadcast(Evento{Tipo: "atleta.creado", Recurso: "atleta", ID: id, Por: ses.Username})
	full, _ := database.GetAtleta(s.db, id)
	writeJSON(w, http.StatusCreated, full)
}

// /api/atletas/{id}[/sub]
func (s *Server) handleAtletaItem(w http.ResponseWriter, r *http.Request) {
	id, sub, ok := idFromPath(r.URL.Path, "/api/atletas/")
	if !ok {
		writeErr(w, http.StatusBadRequest, "id inválido")
		return
	}
	switch sub {
	case "":
		switch r.Method {
		case http.MethodGet:
			s.obtenerAtleta(w, id)
		case http.MethodPut:
			s.actualizarAtleta(w, r, id)
		case http.MethodDelete:
			s.eliminarAtleta(w, r, id)
		default:
			writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		}
	case "cinturon":
		s.agregarCinturon(w, r, id)
	case "retirar":
		s.retirarAtleta(w, r, id)
	case "reactivar":
		s.reactivarAtleta(w, r, id)
	case "planilla.pdf":
		s.handlePlanillaAtleta(w, r, id)
	case "foto":
		s.handleFoto(w, r, entAtleta, id)
	default:
		if strings.HasPrefix(sub, "documentos") {
			s.handleDocumentos(w, r, entAtleta, id, sub)
			return
		}
		writeErr(w, http.StatusNotFound, "recurso no encontrado")
	}
}

func (s *Server) obtenerAtleta(w http.ResponseWriter, id int64) {
	a, err := database.GetAtleta(s.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "atleta no encontrado")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) actualizarAtleta(w http.ResponseWriter, r *http.Request, id int64) {
	if !s.soloAdmin(w, r) {
		return
	}
	ses, _ := s.sessions.obtener(r)
	var in database.AtletaInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	if errs := s.validarAtleta(&in); len(errs) > 0 {
		writeFieldErrs(w, errs)
		return
	}
	err := database.UpdateAtleta(s.db, id, in, ses.EntrenadorID)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "atleta no encontrado")
		return
	}
	if errors.Is(err, database.ErrCedulaDuplicada) {
		writeFieldErrs(w, map[string]string{"cedula_numero": err.Error()})
		return
	}
	if err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	s.hub.Broadcast(Evento{Tipo: "atleta.actualizado", Recurso: "atleta", ID: id, Por: ses.Username})
	full, _ := database.GetAtleta(s.db, id)
	writeJSON(w, http.StatusOK, full)
}

func (s *Server) eliminarAtleta(w http.ResponseWriter, r *http.Request, id int64) {
	ses, _ := s.sessions.obtener(r)
	// Solo el administrador puede borrar definitivamente (dato sensible).
	if !ses.EsAdmin {
		writeErr(w, http.StatusForbidden, "solo un administrador puede eliminar atletas")
		return
	}
	err := database.DeleteAtleta(s.db, id, ses.EntrenadorID)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "atleta no encontrado")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.hub.Broadcast(Evento{Tipo: "atleta.eliminado", Recurso: "atleta", ID: id, Por: ses.Username})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) agregarCinturon(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	if !s.soloAdmin(w, r) {
		return
	}
	ses, _ := s.sessions.obtener(r)
	var in struct {
		CinturonID int64  `json:"cinturon_id"`
		Dan        *int   `json:"dan"`
		Fecha      string `json:"fecha_cambio"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}
	if in.CinturonID == 0 {
		writeErr(w, http.StatusUnprocessableEntity, "cinturón requerido")
		return
	}
	if err := s.validarDan(in.CinturonID, &in.Dan); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if in.Fecha == "" {
		in.Fecha = time.Now().Format("2006-01-02")
	}
	if err := database.AgregarCinturon(s.db, id, in.CinturonID, in.Dan, in.Fecha, ses.EntrenadorID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.hub.Broadcast(Evento{Tipo: "atleta.actualizado", Recurso: "atleta", ID: id, Por: ses.Username})
	full, _ := database.GetAtleta(s.db, id)
	writeJSON(w, http.StatusOK, full)
}

func (s *Server) retirarAtleta(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	if !s.soloAdmin(w, r) {
		return
	}
	ses, _ := s.sessions.obtener(r)
	var in struct {
		Fecha  string  `json:"fecha"`
		Motivo *string `json:"motivo"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}
	if in.Fecha == "" {
		in.Fecha = time.Now().Format("2006-01-02")
	}
	if err := database.Retirar(s.db, id, in.Fecha, in.Motivo, ses.EntrenadorID); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	s.hub.Broadcast(Evento{Tipo: "atleta.actualizado", Recurso: "atleta", ID: id, Por: ses.Username})
	full, _ := database.GetAtleta(s.db, id)
	writeJSON(w, http.StatusOK, full)
}

func (s *Server) reactivarAtleta(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	if !s.soloAdmin(w, r) {
		return
	}
	ses, _ := s.sessions.obtener(r)
	var in struct {
		Fecha string `json:"fecha"`
	}
	_ = decode(r, &in)
	if in.Fecha == "" {
		in.Fecha = time.Now().Format("2006-01-02")
	}
	if err := database.Reactivar(s.db, id, in.Fecha, ses.EntrenadorID); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	s.hub.Broadcast(Evento{Tipo: "atleta.actualizado", Recurso: "atleta", ID: id, Por: ses.Username})
	full, _ := database.GetAtleta(s.db, id)
	writeJSON(w, http.StatusOK, full)
}

// ---- Validaciones de negocio (nivel aplicación) ---------------------------

// validarAtleta normaliza y valida el input. Devuelve un mapa {campo: mensaje}
// (vacío si todo es válido) para que el formulario resalte cada campo en rojo.
// Las claves coinciden con las que espera el frontend.
func (s *Server) validarAtleta(in *database.AtletaInput) map[string]string {
	errs := map[string]string{}

	in.Nombres = strings.TrimSpace(in.Nombres)
	in.Apellidos = strings.TrimSpace(in.Apellidos)
	if in.Nombres == "" {
		errs["nombres"] = "Requerido"
	}
	if in.Apellidos == "" {
		errs["apellidos"] = "Requerido"
	}

	nac, errNac := time.Parse("2006-01-02", in.FechaNacimiento)
	if errNac != nil {
		errs["fecha_nacimiento"] = "Fecha inválida"
	}
	if _, err := time.Parse("2006-01-02", in.FechaInscripcion); err != nil {
		errs["fecha_inscripcion"] = "Fecha inválida"
	}

	// Regla #6/#10: cédula = tipo (V/E/P) + número, coherentes entre sí.
	validarCedula(trimPtr(in.CedulaTipo), trimPtr(in.CedulaNumero), &in.CedulaTipo, &in.CedulaNumero, errs, "cedula_tipo", "cedula_numero")

	// Regla #6/#1: DAN solo válido si el cinturón inicial es negro.
	if in.CinturonID != nil {
		if err := s.validarDan(*in.CinturonID, &in.Dan); err != nil {
			errs["dan"] = err.Error()
		}
	} else {
		in.Dan = nil
	}

	// Regla #7: máximo 3 teléfonos de contacto (se ignoran los vacíos).
	limpios := in.TelefonosContacto[:0]
	for _, t := range in.TelefonosContacto {
		if strings.TrimSpace(t) != "" {
			limpios = append(limpios, strings.TrimSpace(t))
		}
	}
	in.TelefonosContacto = limpios
	if len(in.TelefonosContacto) > 3 {
		errs["telefonos_contacto"] = "Máximo 3 teléfonos de contacto"
	}

	// Regla #2: menor de 18 → representante obligatorio; su teléfono se usa
	// como principal del atleta si no se indicó otro.
	if errNac == nil && edadDesde(nac) < 18 {
		r := in.Representante
		if r == nil || vacio(r.Nombres) {
			errs["rep_nombres"] = "Requerido para menores"
		}
		if r == nil || vacio(r.Apellidos) {
			errs["rep_apellidos"] = "Requerido para menores"
		}
		if r == nil || vacio(r.Telefono) {
			errs["rep_telefono"] = "Requerido para menores"
		}
		if r != nil && !vacio(r.Telefono) && vacio(in.Telefono) {
			in.Telefono = r.Telefono
		}
	}
	if r := in.Representante; r != nil {
		validarCedula(trimPtr(r.CedulaTipo), trimPtr(r.CedulaNumero), &r.CedulaTipo, &r.CedulaNumero, errs, "rep_cedula_tipo", "rep_cedula_numero")
	}

	// Regla #11: el teléfono principal es requerido (a nivel app).
	if vacio(in.Telefono) {
		errs["telefono"] = "El teléfono principal es requerido"
	}

	// Información médica (planilla): si la respuesta es "sí", el detalle es
	// obligatorio; si es "no"/sin responder, el detalle se descarta.
	validarMedica(in.MedEnfermedad, &in.MedEnfermedadDet, errs, "med_enfermedad_detalle")
	validarMedica(in.MedAlergia, &in.MedAlergiaDet, errs, "med_alergia_detalle")
	validarMedica(in.MedOperado, &in.MedOperadoDet, errs, "med_operado_detalle")

	// Ubicación jerárquica: rellenar ancestros desde el nivel más profundo.
	database.NormalizarUbicacion(s.db, in)
	return errs
}

// validarCedula normaliza la cédula. La presencia se decide SOLO por el número
// (el tipo por defecto es "V"): si no hay número, la cédula se considera ausente
// y ambos quedan en NULL. Si hay número, el tipo debe ser V/E/P.
func validarCedula(tipo, numero string, dstTipo, dstNumero **string, errs map[string]string, keyTipo, keyNumero string) {
	if numero == "" {
		*dstTipo, *dstNumero = nil, nil
		return
	}
	if tipo != "V" && tipo != "E" && tipo != "P" {
		errs[keyTipo] = "Seleccione tipo (V/E/P)"
	}
	if !soloDigitos(numero) {
		errs[keyNumero] = "Solo dígitos"
	}
	*dstTipo = strPtr(tipo)
	*dstNumero = strPtr(numero)
}

// validarMedica exige el detalle cuando la respuesta es "sí" y lo limpia cuando
// es "no" o no se respondió.
func validarMedica(estado *bool, detalle **string, errs map[string]string, key string) {
	if estado != nil && *estado {
		if vacio(*detalle) {
			errs[key] = "Especifique (obligatorio si respondió Sí)"
		}
	} else {
		*detalle = nil
	}
}

// validarDan aplica la regla: negro ⇒ dan 1..9 obligatorio; no negro ⇒ dan NULL.
func (s *Server) validarDan(cinturonID int64, dan **int) error {
	negro, err := database.CinturonEsNegro(s.db, cinturonID)
	if err != nil {
		return err
	}
	if negro {
		if *dan == nil || **dan < 1 || **dan > 9 {
			return errors.New("el cinturón negro requiere un DAN entre 1 y 9")
		}
	} else {
		*dan = nil // se ignora cualquier DAN enviado para cinturones no negros
	}
	return nil
}

// ---- helpers ---------------------------------------------------------------

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

func vacio(s *string) bool {
	return s == nil || strings.TrimSpace(*s) == ""
}

func edadDesde(nac time.Time) int {
	now := time.Now()
	y := now.Year() - nac.Year()
	if now.YearDay() < nac.YearDay() {
		y--
	}
	return y
}

// writeFieldErrs responde 422 con {"errors": {campo: mensaje}} para que el
// formulario resalte los campos inválidos.
func writeFieldErrs(w http.ResponseWriter, errs map[string]string) {
	writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"errors": errs})
}

// trimPtr devuelve el contenido recortado de un *string (o "" si es nil).
func trimPtr(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

// strPtr devuelve un *string, o nil si la cadena está vacía.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func soloDigitos(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}
