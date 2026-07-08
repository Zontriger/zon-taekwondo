package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/fs"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-pdf/fpdf"

	"zon-taekwondo/database"
)

var (
	logoAcademiaOnce sync.Once
	logoAcademiaPNG  []byte // PNG normalizado para fpdf (nil si no está disponible)
)

// logoAcademiaBytes devuelve el logo de la academia normalizado a un PNG que el
// parser de fpdf procesa de forma fiable: se decodifica el original y se re-
// codifica aplanado sobre blanco (fpdf falla con ciertos PNG con transparencia o
// chunks poco comunes). El resultado se cachea. Devuelve nil si no hay logo.
func (s *Server) logoAcademiaBytes() []byte {
	logoAcademiaOnce.Do(func() {
		raw, err := fs.ReadFile(s.frontend, "logo_academia.png")
		if err != nil {
			return
		}
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			return
		}
		b := img.Bounds()
		rgba := image.NewRGBA(b)
		draw.Draw(rgba, b, image.NewUniform(color.White), image.Point{}, draw.Src) // fondo blanco
		draw.Draw(rgba, b, img, b.Min, draw.Over)                                   // logo encima
		var buf bytes.Buffer
		if err := png.Encode(&buf, rgba); err != nil {
			return
		}
		logoAcademiaPNG = buf.Bytes()
	})
	return logoAcademiaPNG
}

// registrarLogoAcademia registra el logo bajo el nombre "logo_academia" y
// devuelve true si quedó disponible como membrete. Protegido con recover: un
// logo problemático nunca debe impedir la generación del PDF.
func (s *Server) registrarLogoAcademia(pdf *fpdf.Fpdf) (ok bool) {
	data := s.logoAcademiaBytes()
	if data == nil {
		return false
	}
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()
	pdf.RegisterImageOptionsReader("logo_academia", fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(data))
	return pdf.Ok()
}

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
	// Acciones en lote: PDF solo de los atletas seleccionados (ids coma-separados).
	if ids := q.Get("ids"); strings.TrimSpace(ids) != "" {
		for id := range idsSeleccionados(ids) {
			f.IDs = append(f.IDs, id)
		}
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

	// Encabezado con membrete: logo de la academia a la izquierda.
	conLogo := s.registrarLogoAcademia(pdf)
	xTexto := 10.0
	if conLogo {
		pdf.ImageOptions("logo_academia", 10, 10, 18, 18, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
		xTexto = 31
	}
	pdf.SetXY(xTexto, 11)
	pdf.SetFont("Arial", "B", 16)
	pdf.SetTextColor(0, 47, 108)
	pdf.CellFormat(0, 9, tr("Taekwondo Miranda — Reporte de Atletas"), "", 1, "L", false, 0, "")
	pdf.SetX(xTexto)
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(90, 90, 90)
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("Total de atletas: %d", total)), "", 1, "L", false, 0, "")
	pdf.SetY(30)
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
			key = fmt.Sprintf("%s %s DAN", key, roman(*a.CinturonDan))
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

func derefOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
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
		return fmt.Sprintf("%s %s DAN", color, roman(*dan))
	}
	return color
}

// roman convierte 1..9 a numeración romana (para mostrar el DAN).
func roman(n int) string {
	nums := []string{"", "I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX"}
	if n >= 1 && n <= 9 {
		return nums[n]
	}
	return fmt.Sprintf("%d", n)
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
