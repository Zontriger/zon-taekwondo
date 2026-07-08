package backend

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // registra JPEG para image.DecodeConfig
	"image/png"
	"io"
	"io/fs"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/go-pdf/fpdf"

	"zon-taekwondo/database"
)

var (
	firmaOnce sync.Once
	firmaPNG  []byte // firma del entrenador normalizada (nil si no está)
)

// firmaEntrenador devuelve la firma del entrenador (assets/firma_entrenador.png)
// re-codificada a un PNG limpio (conservando la transparencia) para que fpdf la
// procese de forma fiable. Cacheada; nil si no está disponible.
func (s *Server) firmaEntrenador() []byte {
	firmaOnce.Do(func() {
		raw, err := fs.ReadFile(s.frontend, "assets/firma_entrenador.png")
		if err != nil {
			return
		}
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			return
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return
		}
		firmaPNG = buf.Bytes()
	})
	return firmaPNG
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

// dibujarFotoAjustada coloca la imagen dentro del recuadro (bx,by,bw,bh)
// respetando su proporción (modo "contain": se ve completa, centrada, sin
// deformarse). Devuelve false si no se pudo dibujar.
func dibujarFotoAjustada(pdf *fpdf.Fpdf, path string, bx, by, bw, bh float64) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()
	dw, dh := bw-2, bh-2
	if f, err := os.Open(path); err == nil {
		cfg, _, err := image.DecodeConfig(f)
		f.Close()
		if err == nil && cfg.Width > 0 && cfg.Height > 0 {
			iw, ih := float64(cfg.Width), float64(cfg.Height)
			scale := math.Min(dw/iw, dh/ih)
			dw, dh = iw*scale, ih*scale
		}
	}
	x := bx + (bw-dw)/2
	y := by + (bh-dh)/2
	pdf.ImageOptions(path, x, y, dw, dh, false, fpdf.ImageOptions{ImageType: "", ReadDpi: true}, 0, "")
	return true
}

// fechaDMY convierte "YYYY-MM-DD" a "DD/MM/YYYY" (o "" si no aplica).
func fechaDMY(iso string) string {
	iso = strings.TrimSpace(iso)
	if len(iso) != 10 || iso[4] != '-' || iso[7] != '-' {
		return iso
	}
	return iso[8:10] + "/" + iso[5:7] + "/" + iso[0:4]
}

// GET /api/atletas/{id}/planilla.pdf → "Planilla del Atleta" oficial rellenada.
func (s *Server) handlePlanillaAtleta(w http.ResponseWriter, r *http.Request, id int64) {
	a, err := database.GetAtleta(s.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "atleta no encontrado")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	ses, _ := s.sessions.obtener(r)
	pdf := s.construirPlanilla(a, ses.EsAdmin)
	w.Header().Set("Content-Type", "application/pdf")
	setAttachment(w, "planilla-"+database.NombreArchivoAtleta(s.db, id)+".pdf")
	if err := pdf.Output(w); err != nil {
		fmt.Printf("[planilla] error generando PDF: %v\n", err)
	}
}

// GET /api/reportes/planilla-blanco.pdf → "Planilla del Atleta" en blanco, para
// imprimir y llenar a mano.
func (s *Server) handlePlanillaBlanco(w http.ResponseWriter, r *http.Request) {
	pdf := s.construirPlanilla(nil, false)
	w.Header().Set("Content-Type", "application/pdf")
	setAttachment(w, "planilla-en-blanco.pdf")
	if err := pdf.Output(w); err != nil {
		fmt.Printf("[planilla] error generando PDF en blanco: %v\n", err)
	}
}

// construirPlanilla dibuja el formato oficial "Planilla del Atleta" (Club de
// Taekwondo Elite Oro Carrizal). Si a es nil, genera el formulario en blanco.
// Reproduce fielmente la retícula del original: encabezado institucional,
// recuadro de FOTO, tablas de datos, información médica y firmas.
func (s *Server) construirPlanilla(a *database.Atleta, esAdmin bool) *fpdf.Fpdf {
	pdf := fpdf.New("P", "mm", "Letter", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetMargins(10, 10, 10)
	pdf.SetAutoPageBreak(false, 10)
	pdf.AddPage()
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetLineWidth(0.2)

	const (
		LX = 10.0
		RX = 206.0
		W  = RX - LX
	)

	// ---------- Valores (vacíos en la planilla en blanco) ----------
	var (
		nombreAtleta, cedulaAtleta, edadStr             string
		nacDia, nacMes, nacAnio                         string
		sexoM, sexoF                                    bool
		fIngreso, horario, telefono, email, direccion   string
		estatura, peso, imc, fc, tCamisa, tPantalon     string
		instituto, institutoDir                         string
		repNombre, repCed, repTrabajo, repDirTrab       string
		repTel, repEmail                                string
		medEnf, medAler, medOper                        *bool
		medEnfDet, medAlerDet, medOperDet, medEmergencia string
	)
	if a != nil {
		nombreAtleta = strings.TrimSpace(a.Nombres + " " + a.Apellidos)
		cedulaAtleta = cedTexto(a.CedulaTipo, a.CedulaNumero)
		edadStr = fmt.Sprintf("%d", a.Edad)
		if len(a.FechaNacimiento) == 10 {
			nacAnio, nacMes, nacDia = a.FechaNacimiento[0:4], a.FechaNacimiento[5:7], a.FechaNacimiento[8:10]
		}
		if a.Sexo != nil {
			sexoM = strings.EqualFold(*a.Sexo, "M")
			sexoF = strings.EqualFold(*a.Sexo, "F")
		}
		fIngreso = fechaDMY(a.FechaInscripcion)
		horario = derefOr(a.Horario)
		telefono = derefOr(a.Telefono)
		email = derefOr(a.Email)
		direccion = derefOr(a.DireccionDetalle)
		estatura, peso, imc, fc = derefOr(a.Estatura), derefOr(a.Peso), derefOr(a.IMC), derefOr(a.FC)
		tCamisa, tPantalon = derefOr(a.TallaCamisa), derefOr(a.TallaPantalon)
		instituto, institutoDir = derefOr(a.Instituto), derefOr(a.InstitutoDireccion)
		medEnf, medAler, medOper = a.MedEnfermedad, a.MedAlergia, a.MedOperado
		medEnfDet, medAlerDet, medOperDet = derefOr(a.MedEnfermedadDet), derefOr(a.MedAlergiaDet), derefOr(a.MedOperadoDet)
		medEmergencia = derefOr(a.MedEmergencia)
		if a.Representante != nil {
			r := a.Representante
			repNombre = strings.TrimSpace(derefOr(r.Nombres) + " " + derefOr(r.Apellidos))
			repCed = cedTexto(r.CedulaTipo, r.CedulaNumero)
			repTrabajo, repDirTrab = derefOr(r.LugarTrabajo), derefOr(r.DireccionTrabajo)
			repTel, repEmail = derefOr(r.Telefono), derefOr(r.Email)
		}
	}

	// ---------- Helpers de dibujo (coordenadas absolutas) ----------
	rect := func(x, y, w, h float64) { pdf.Rect(x, y, w, h, "D") }
	line := func(x1, y1, x2, y2 float64) { pdf.Line(x1, y1, x2, y2) }
	fit := func(s string, maxW float64) string {
		if pdf.GetStringWidth(tr(s)) <= maxW {
			return s
		}
		rr := []rune(s)
		for len(rr) > 0 {
			rr = rr[:len(rr)-1]
			if pdf.GetStringWidth(tr(string(rr)+"…")) <= maxW {
				return string(rr) + "…"
			}
		}
		return ""
	}
	lbl := func(x, y float64, s string) { pdf.SetFont("Arial", "B", 8); pdf.Text(x, y, tr(s)) }
	labelValue := func(x, y, cellW, baseY float64, label, val string) {
		pdf.SetFont("Arial", "B", 8)
		pdf.Text(x+1.8, baseY, tr(label))
		lw := pdf.GetStringWidth(tr(label))
		pdf.SetFont("Arial", "", 9)
		pdf.Text(x+1.8+lw+1.5, baseY, tr(fit(val, cellW-(lw+5))))
	}
	centerIn := func(x, w, y float64, s string) {
		sw := pdf.GetStringWidth(tr(s))
		pdf.Text(x+(w-sw)/2, y, tr(s))
	}
	checkbox := func(x, y float64, on bool) {
		pdf.Rect(x, y, 3.6, 3.6, "D")
		if on {
			pdf.SetFont("Arial", "B", 10)
			pdf.Text(x+0.55, y+3.15, "X")
		}
	}
	banda := func(y float64, s string) float64 {
		pdf.SetFont("Arial", "B", 10)
		centerIn(LX, W, y+4.5, s)
		return y + 6.5
	}
	// filaFull: celda de ancho completo con etiqueta + valor. Devuelve la nueva y.
	filaFull := func(y, h float64, label, val string) float64 {
		rect(LX, y, W, h)
		labelValue(LX, y, W, y+h/2+1.4, label, val)
		return y + h
	}

	// ============================ ENCABEZADO ============================
	// Texto institucional centrado en toda la página.
	pdf.SetFont("Arial", "", 9)
	centerIn(LX, W, 16, "ALCALDIA DEL MUNICIPIO CARRIZAL")
	centerIn(LX, W, 21, "INSTITUTO MUNICIPAL DE DEPORTE Y")
	centerIn(LX, W, 26, "RECREACION DE CARRIZAL")
	pdf.SetFont("Arial", "B", 10)
	centerIn(LX, W, 31.5, "CLUB DE TAEKWONDO ELITE ORO CARRIZAL")
	pdf.SetFont("Arial", "BI", 21)
	centerIn(LX, W, 45, tr("Planilla del Atleta"))

	// Recuadro de FOTO (arriba a la derecha, compacto). La foto respeta su
	// proporción (se ajusta sin deformarse dentro del recuadro).
	fw, fh := 30.0, 36.0
	fx, fy := RX-fw, 12.0
	rect(fx, fy, fw, fh)
	fotoPuesta := false
	if a != nil && a.FotoPath != nil && *a.FotoPath != "" && (esAdmin || !a.EsMenor) {
		if _, e := os.Stat(*a.FotoPath); e == nil {
			fotoPuesta = dibujarFotoAjustada(pdf, *a.FotoPath, fx, fy, fw, fh)
		}
	}
	if !fotoPuesta {
		pdf.SetFont("Arial", "B", 11)
		centerIn(fx, fw, fy+fh/2+2, "FOTO")
	}

	pdf.SetFont("Arial", "", 10)
	pdf.Text(LX+2, 53, tr("Fecha ingreso: "+subrayado(fIngreso, 26)))
	pdf.Text(LX+2, 59, tr("Horario: "+subrayado(horario, 34)))

	// ======================== DATOS DEL ATLETA ========================
	y := banda(62, "DATOS DEL ATLETA")

	// NOMBRE Y APELLIDO | C.I
	ciW := 46.0
	rect(LX, y, W-ciW, 8)
	rect(LX+W-ciW, y, ciW, 8)
	labelValue(LX, y, W-ciW, y+5.2, "NOMBRE Y APELLIDO:", nombreAtleta)
	labelValue(LX+W-ciW, y, ciW, y+5.2, "C.I", cedulaAtleta)
	y += 8

	// EDAD | SEXO | FECHA DE NACIMIENTO | TELEFONO / Email
	bh := 15.0
	cEdad, cSexo, cNac := 20.0, 28.0, 52.0
	xSexo, xNac, xTel := LX+cEdad, LX+cEdad+cSexo, LX+cEdad+cSexo+cNac
	cTel := RX - xTel
	rect(LX, y, W, bh)
	line(xSexo, y, xSexo, y+bh)
	line(xNac, y, xNac, y+bh)
	line(xTel, y, xTel, y+bh)
	line(LX, y+6, xTel, y+6)
	line(xTel, y+7.5, RX, y+7.5)
	lbl(LX+1.8, y+4, "EDAD")
	lbl(xSexo+1.8, y+4, "SEXO")
	lbl(xNac+1.8, y+4, "FECHA DE NACIMIENTO")
	pdf.SetFont("Arial", "", 10)
	centerIn(LX, cEdad, y+12, edadStr)
	lbl(xSexo+2, y+12, "M")
	checkbox(xSexo+7, y+9, sexoM)
	lbl(xSexo+15, y+12, "F")
	checkbox(xSexo+20, y+9, sexoF)
	pdf.SetFont("Arial", "", 10)
	pdf.Text(xNac+4, y+12, tr(fmt.Sprintf("%s  /  %s  /  %s", pad2s(nacDia), pad2s(nacMes), pad4s(nacAnio))))
	labelValue(xTel, y, cTel, y+5, "TELEFONO:", telefono)
	labelValue(xTel, y, cTel, y+12.5, "Email:", email)
	y += bh

	// DIRECCIÓN DE RESIDENCIA
	y = filaFull(y, 11, "DIRECCION DE RESIDENCIA:", direccion)

	// Medidas físicas (6 columnas).
	fh6 := 13.0
	widths := []float64{31, 25, 36, 34, 36, W - (31 + 25 + 36 + 34 + 36)}
	mlabels := []string{"ESTATURA", "PESO", "I.M.C (ADULTOS)", "F.C.(ADULTOS)", "TALLA CAMISA", "TALLA PANTALON"}
	mvals := []string{estatura, peso, imc, fc, tCamisa, tPantalon}
	rect(LX, y, W, fh6)
	line(LX, y+6, RX, y+6)
	cx := LX
	for i, wd := range widths {
		if i > 0 {
			line(cx, y, cx, y+fh6)
		}
		pdf.SetFont("Arial", "B", 7.5)
		centerIn(cx, wd, y+4, mlabels[i])
		pdf.SetFont("Arial", "", 9)
		centerIn(cx, wd, y+11, fit(mvals[i], wd-2))
		cx += wd
	}
	y += fh6

	// INSTITUTO
	y = filaFull(y, 9, "INSTITUTO DONDE ESTUDIA:", instituto)
	y = filaFull(y, 11, "DIRECCION DE LA INSTITUCION EDUCATIVA:", institutoDir)

	// ===================== DATOS DEL REPRESENTANTE =====================
	y = banda(y+1, "DATOS DEL REPRESENTANTE")
	rect(LX, y, W-ciW, 8)
	rect(LX+W-ciW, y, ciW, 8)
	labelValue(LX, y, W-ciW, y+5.2, "NOMBRE Y APELLIDO:", repNombre)
	labelValue(LX+W-ciW, y, ciW, y+5.2, "C.I", repCed)
	y += 8
	y = filaFull(y, 9, "LUGAR DE TRABAJO:", repTrabajo)
	y = filaFull(y, 9, "DIRECCION DE TRABAJO:", repDirTrab)
	half := W / 2
	rect(LX, y, half, 9)
	rect(LX+half, y, W-half, 9)
	labelValue(LX, y, half, y+5.7, "TELEFONO:", repTel)
	labelValue(LX+half, y, W-half, y+5.7, "Email:", repEmail)
	y += 9

	// ======================== INFORMACIÓN MÉDICA ========================
	y = banda(y+1, "INFORMACION MEDICA")
	filaMed := func(y float64, pregunta string, estado *bool, detalle string) float64 {
		hh := 10.0
		siX, noX, espX := LX+92.0, LX+112.0, LX+132.0
		rect(LX, y, W, hh)
		line(siX, y, siX, y+hh)
		line(noX, y, noX, y+hh)
		line(espX, y, espX, y+hh)
		multilinea(pdf, tr, LX+2, y, siX-LX-3, hh, pregunta)
		by := y + hh/2 + 1.4
		lbl(siX+2, by, "SI")
		checkbox(siX+9, y+hh/2-1.8, estado != nil && *estado)
		lbl(noX+2, by, "NO")
		checkbox(noX+9, y+hh/2-1.8, estado != nil && !*estado)
		labelValue(espX, y, RX-espX, by, "ESPECIFIQUE:", detalle)
		return y + hh
	}
	y = filaMed(y, "¿PADECE DE ALGUNA ENFERMEDAD?", medEnf, medEnfDet)
	y = filaMed(y, "¿ES ALERGICO A ALGUN MEDICAMENTO?", medAler, medAlerDet)
	y = filaMed(y, "¿HA SIDO OPERADO?", medOper, medOperDet)
	y = filaFull(y, 12, "EN CASO DE EMERGENCIA LLAMAR A:", medEmergencia)

	// ============================ FIRMAS ============================
	y += 16
	fw2 := 66.0
	line(LX+18, y, LX+18+fw2, y)
	line(RX-18-fw2, y, RX-18, y)
	pdf.SetFont("Arial", "", 9)
	centerIn(LX+18, fw2, y+5, "Firma del Representante")
	centerIn(RX-18-fw2, fw2, y+5, "Firma del Atleta")
	y += 20
	// Firma del entrenador (imagen embebida) centrada sobre su línea.
	if img := s.firmaEntrenador(); img != nil {
		func() {
			defer func() { _ = recover() }()
			pdf.RegisterImageOptionsReader("firma_ent", fpdf.ImageOptions{ImageType: "PNG"}, bytesReader(img))
			if pdf.Ok() {
				sw := 27.0
				sh := sw * 495.0 / 553.0
				// Centrada sobre la línea, ligeramente a la izquierda y más abajo.
				pdf.ImageOptions("firma_ent", LX+W/2-sw/2-5, y-sh+6, sw, sh, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
			}
		}()
	}
	line(LX+(W-fw2)/2, y, LX+(W+fw2)/2, y)
	centerIn(LX+(W-fw2)/2, fw2, y+5, "Firma del Entrenador")

	return pdf
}

// ---- utilidades de la planilla ----

// subrayado añade una línea de guiones bajos de ancho aproximado n caracteres,
// tras un valor (para simular el espacio a llenar en la planilla en blanco).
func subrayado(val string, n int) string {
	val = strings.TrimSpace(val)
	if val != "" {
		return val
	}
	return strings.Repeat("_", n)
}

func pad2s(s string) string {
	if s == "" {
		return "__"
	}
	return s
}
func pad4s(s string) string {
	if s == "" {
		return "____"
	}
	return s
}

// cedTexto formatea la cédula (tipo-número) o "" si no hay.
func cedTexto(tipo, numero *string) string {
	if numero != nil && *numero != "" {
		if tipo != nil && *tipo != "" {
			return *tipo + "-" + *numero
		}
		return *numero
	}
	return ""
}

// multilinea escribe un texto en negrita 8pt ajustado al ancho w dentro de una
// celda de alto h, centrado verticalmente (1 o 2 líneas).
func multilinea(pdf *fpdf.Fpdf, tr func(string) string, x, y, w, h float64, s string) {
	pdf.SetFont("Arial", "B", 8)
	if pdf.GetStringWidth(tr(s)) <= w {
		pdf.Text(x, y+h/2+1.4, tr(s))
		return
	}
	// Partir en dos líneas por palabras.
	palabras := strings.Fields(s)
	l1, l2 := "", ""
	for _, p := range palabras {
		if l2 == "" && pdf.GetStringWidth(tr(l1+" "+p)) <= w {
			if l1 == "" {
				l1 = p
			} else {
				l1 += " " + p
			}
		} else {
			if l2 == "" {
				l2 = p
			} else {
				l2 += " " + p
			}
		}
	}
	pdf.Text(x, y+h/2-1, tr(l1))
	pdf.Text(x, y+h/2+3.4, tr(l2))
}
