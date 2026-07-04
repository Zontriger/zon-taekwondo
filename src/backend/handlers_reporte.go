package backend

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"

	"zon-taekwondo/database"
)

// GET /api/reportes/atletas.pdf?<mismos filtros que la lista>
// Reutiliza el filtrado de la lista (q + domain) y genera un PDF con la tabla
// de atletas y un resumen agregado (total, activos/retirados, cinturones).
func (s *Server) handleReporteAtletas(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := database.AtletaFiltro{Texto: q.Get("q"), Limit: 100000, Offset: 0}
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
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	pdf := fpdf.New("P", "mm", "Letter", "") // carta vertical
	tr := pdf.UnicodeTranslatorFromDescriptor("") // acentos/ñ
	pdf.SetTitle("Reporte de Atletas", false)
	pdf.SetMargins(10, 12, 10)
	pdf.AliasNbPages("")
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Arial", "I", 8)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 8, tr(fmt.Sprintf("Generado %s — página %d/{nb}",
			time.Now().Format("2006-01-02 15:04"), pdf.PageNo())), "", 0, "C", false, 0, "")
	})
	pdf.AddPage()

	// Encabezado
	pdf.SetFont("Arial", "B", 16)
	pdf.SetTextColor(0, 47, 108)
	pdf.CellFormat(0, 9, tr("Taekwondo Miranda — Reporte de Atletas"), "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(90, 90, 90)
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("Total de atletas: %d", total)), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// Cabecera de tabla
	cols := []struct {
		t string
		w float64
	}{{"Apellidos y nombres", 62}, {"Cédula", 26}, {"Edad", 13}, {"Escuela", 45}, {"Cinturón", 32}, {"Estado", 18}}
	var anchoTabla float64
	for _, c := range cols {
		anchoTabla += c.w
	}
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(0, 47, 108)
	pdf.SetTextColor(255, 255, 255)
	for _, c := range cols {
		pdf.CellFormat(c.w, 7, tr(c.t), "1", 0, "L", true, 0, "")
	}
	pdf.Ln(-1)

	// Filas
	pdf.SetFont("Arial", "", 8)
	pdf.SetTextColor(20, 20, 20)
	fill := false
	activos, retirados := 0, 0
	porCinturon := map[string]int{}
	for _, a := range items {
		if a.Estado == "activo" {
			activos++
		} else {
			retirados++
		}
		key := a.CinturonColor
		if key == "" {
			key = "(sin cinturón)"
		}
		if a.CinturonDan != nil {
			key = fmt.Sprintf("%s %d° DAN", key, *a.CinturonDan)
		}
		porCinturon[key]++

		if fill {
			pdf.SetFillColor(245, 247, 251)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		vals := []string{
			fmt.Sprintf("%s, %s", a.Apellidos, a.Nombres),
			cedulaTxt(a.CedulaTipo, a.CedulaNumero),
			fmt.Sprintf("%d", a.Edad),
			a.EscuelaNombre,
			cinturonTxt(a.CinturonColor, a.CinturonDan),
			a.Estado,
		}
		for i, c := range cols {
			pdf.CellFormat(c.w, 6, tr(recorta(vals[i], c.w)), "1", 0, "L", true, 0, "")
		}
		pdf.Ln(-1)
		fill = !fill
	}
	if len(items) == 0 {
		pdf.SetFont("Arial", "I", 9)
		pdf.CellFormat(anchoTabla, 8, tr("No hay atletas que coincidan con el filtro."), "1", 1, "C", false, 0, "")
	}

	// Resumen agregado
	pdf.Ln(4)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetTextColor(0, 47, 108)
	pdf.CellFormat(0, 7, tr("Resumen"), "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(20, 20, 20)
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("Activos: %d    Retirados: %d    Total: %d", activos, retirados, total)), "", 1, "L", false, 0, "")
	pdf.Ln(1)
	pdf.SetFont("Arial", "B", 9)
	pdf.CellFormat(0, 6, tr("Distribución por cinturón:"), "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	claves := make([]string, 0, len(porCinturon))
	for k := range porCinturon {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	for _, k := range claves {
		pdf.CellFormat(0, 5.5, tr(fmt.Sprintf("   • %s: %d", k, porCinturon[k])), "", 1, "L", false, 0, "")
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="reporte-atletas.pdf"`)
	if err := pdf.Output(w); err != nil {
		// Ya se enviaron headers; solo se registra.
		fmt.Printf("[reporte] error generando PDF: %v\n", err)
	}
}

// handleFichaAtleta genera la ficha técnica de un atleta en PDF (carta vertical).
func (s *Server) handleFichaAtleta(w http.ResponseWriter, id int64) {
	a, err := database.GetAtleta(s.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "atleta no encontrado")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	pdf := fpdf.New("P", "mm", "Letter", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 16)
	pdf.SetTextColor(0, 47, 108)
	pdf.CellFormat(0, 10, tr("SIGAT — Ficha Técnica del Atleta"), "", 1, "L", false, 0, "")
	pdf.SetDrawColor(0, 47, 108)
	pdf.SetLineWidth(0.5)
	y := pdf.GetY()
	pdf.Line(15, y, 201, y)
	pdf.Ln(4)

	// Foto (si existe el archivo en disco).
	if a.FotoPath != nil && *a.FotoPath != "" {
		if _, e := os.Stat(*a.FotoPath); e == nil {
			pdf.ImageOptions(*a.FotoPath, 160, 30, 36, 0, false, fpdf.ImageOptions{ImageType: "", ReadDpi: true}, 0, "")
		}
	}

	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(20, 20, 20)
	pdf.CellFormat(140, 8, tr(a.Apellidos+", "+a.Nombres), "", 1, "L", false, 0, "")
	pdf.Ln(1)

	campo := func(label, valor string) {
		if valor == "" {
			valor = "—"
		}
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(90, 90, 90)
		pdf.CellFormat(48, 6, tr(label), "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 10)
		pdf.SetTextColor(20, 20, 20)
		pdf.MultiCell(0, 6, tr(valor), "", "L", false)
	}
	sec := func(t string) {
		pdf.Ln(2)
		pdf.SetFont("Arial", "B", 11)
		pdf.SetTextColor(0, 47, 108)
		pdf.CellFormat(0, 7, tr(t), "B", 1, "L", false, 0, "")
		pdf.Ln(1)
	}

	insc := a.FechaInscripcion
	if !a.InscripcionDiaExacto && len(insc) >= 7 {
		insc = insc[:7]
	}
	sec("Datos personales")
	campo("Cédula", cedulaTxt(a.CedulaTipo, a.CedulaNumero))
	campo("Fecha de nacimiento", fmt.Sprintf("%s  (%d años)", a.FechaNacimiento, a.Edad))
	campo("Tipo de sangre", derefOr(a.TipoSangre))
	campo("Teléfono principal", derefOr(a.Telefono))
	if len(a.TelefonosContacto) > 0 {
		campo("Teléfonos de contacto", strings.Join(a.TelefonosContacto, ", "))
	}

	sec("Formación")
	campo("Escuela", a.EscuelaNombre)
	campo("Entrenador (maestro)", a.MaestroNombre)
	campo("Cinturón actual", cinturonTxt(a.CinturonColor, a.CinturonDan))
	campo("Fecha de inicio", insc)
	campo("Estado", a.Estado)

	sec("Ubicación")
	ubic := strings.Join(soloNoVacios(a.ParroquiaNom, a.MunicipioNom, a.CiudadNom, a.EstadoNom), ", ")
	campo("Ubicación", ubic)
	campo("Dirección", derefOr(a.DireccionDetalle))

	if r := a.Representante; r != nil {
		sec("Representante")
		campo("Nombre", strings.TrimSpace(derefOr(r.Nombres)+" "+derefOr(r.Apellidos)))
		campo("Cédula", cedulaTxt(r.CedulaTipo, r.CedulaNumero))
		campo("Teléfono", derefOr(r.Telefono))
		campo("Parentesco", derefOr(r.Parentesco))
	}

	pdf.SetY(-15)
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(120, 120, 120)
	pdf.CellFormat(0, 8, tr("Generado "+time.Now().Format("2006-01-02 15:04")+" — SIGAT"), "", 0, "C", false, 0, "")

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="ficha-atleta.pdf"`)
	if err := pdf.Output(w); err != nil {
		fmt.Printf("[ficha] error generando PDF: %v\n", err)
	}
}

func derefOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func soloNoVacios(vs ...string) []string {
	out := []string{}
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

func cedulaTxt(tipo, numero *string) string {
	if tipo != nil && numero != nil && *numero != "" {
		return *tipo + "-" + *numero
	}
	if numero != nil {
		return *numero
	}
	return "—"
}

func cinturonTxt(color string, dan *int) string {
	if color == "" {
		return "—"
	}
	if dan != nil {
		return fmt.Sprintf("%s %d° DAN", color, *dan)
	}
	return color
}

// recorta trunca un texto para que no desborde una celda de ancho w (mm).
func recorta(s string, w float64) string {
	max := int(w / 1.7) // aprox. caracteres que caben a 8pt
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max < 1 {
		return ""
	}
	return string(r[:max-1]) + "…"
}
