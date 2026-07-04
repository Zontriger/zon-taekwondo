package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
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

	pdf := fpdf.New("L", "mm", "A4", "")
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
	}{{"Apellidos y nombres", 78}, {"Cédula", 32}, {"Edad", 16}, {"Escuela", 62}, {"Cinturón", 52}, {"Estado", 37}}
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
