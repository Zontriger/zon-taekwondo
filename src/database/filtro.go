package database

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Filtro avanzado tipo Odoo: un dominio booleano (all/any) de condiciones
// campo–operador–valor. Todo se traduce a SQL PARAMETRIZADO usando una lista
// blanca de campos y operadores; nunca se interpola valor del usuario en SQL.

type fieldKind int

const (
	kText fieldKind = iota
	kNumber
	kDate
	kSelect // basado en id (escuela, municipio, cinturón)
	kEstado // enum derivado activo/retirado
)

type fieldSpec struct {
	Label   string
	Kind    fieldKind
	Expr    string // expresión SQL sobre el alias 'a' (atleta)
	Options string // catálogo de opciones para kSelect: escuelas|municipios|cinturones
}

// Lista blanca de campos filtrables del atleta.
var atletaFields = map[string]fieldSpec{
	"nombres":           {"Nombres", kText, "a.nombres", ""},
	"apellidos":         {"Apellidos", kText, "a.apellidos", ""},
	"cedula":            {"Cédula (número)", kText, "a.cedula_numero", ""},
	"telefono":          {"Teléfono principal", kText, "a.telefono", ""},
	"edad":              {"Edad", kNumber, "CAST((julianday('now') - julianday(a.fecha_nacimiento))/365.25 AS INTEGER)", ""},
	"fecha_inscripcion": {"Fecha de inscripción", kDate, "a.fecha_inscripcion", ""},
	"fecha_registro":    {"Fecha de registro", kDate, "date(a.creado_en)", ""},
	"escuela":           {"Escuela", kSelect, "a.escuela_id", "escuelas"},
	"maestro":           {"Entrenador", kSelect, "a.maestro_id", "maestros"},
	"tipo_sangre":       {"Tipo de sangre", kText, "a.tipo_sangre", ""},
	"estado":            {"Estado", kSelect, "a.estado_id", "estados"},
	"ciudad":            {"Ciudad", kSelect, "a.ciudad_id", "ciudades"},
	"municipio":         {"Municipio", kSelect, "a.municipio_id", "municipios"},
	"parroquia":         {"Parroquia", kSelect, "a.parroquia_id", "parroquias"},
	"cinturon":          {"Cinturón actual", kSelect, "(SELECT h.cinturon_id FROM historial_cinturon h WHERE h.atleta_id=a.id ORDER BY h.fecha_cambio DESC, h.id DESC LIMIT 1)", "cinturones"},
	"estado_actividad":  {"Estado (actividad)", kEstado, "", "estado"},
}

// Operadores SQL comparables (número y fecha comparten los mismos).
var cmpOps = map[string]string{"eq": "=", "neq": "<>", "gt": ">", "lt": "<", "gte": ">=", "lte": "<="}

const periodoAbierto = "EXISTS(SELECT 1 FROM periodo_actividad p WHERE p.atleta_id=a.id AND p.fecha_fin IS NULL)"

// buildFiltro combina la búsqueda rápida (texto) con el dominio avanzado.
func buildFiltro(f AtletaFiltro) (string, []any, error) {
	var clauses []string
	var args []any

	if t := strings.TrimSpace(f.Texto); t != "" {
		clauses = append(clauses, "(a.nombres LIKE ? OR a.apellidos LIKE ? OR a.cedula_numero LIKE ?)")
		like := "%" + t + "%"
		args = append(args, like, like, like)
	}

	if f.Dominio != nil && len(f.Dominio.Condiciones) > 0 {
		joiner := " AND "
		if f.Dominio.Match == "any" {
			joiner = " OR "
		}
		var parts []string
		for _, c := range f.Dominio.Condiciones {
			s, a, err := condSQL(c)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, s)
			args = append(args, a...)
		}
		clauses = append(clauses, "("+strings.Join(parts, joiner)+")")
	}

	if len(clauses) == 0 {
		return "", args, nil
	}
	return "WHERE " + strings.Join(clauses, " AND "), args, nil
}

// condSQL traduce UNA condición a SQL parametrizado seguro.
func condSQL(c Condicion) (string, []any, error) {
	spec, ok := atletaFields[c.Field]
	if !ok {
		return "", nil, fmt.Errorf("campo no permitido: %q", c.Field)
	}
	v := strings.TrimSpace(c.Value)

	switch spec.Kind {
	case kEstado:
		activo := c.Value == "activo"
		if c.Value != "activo" && c.Value != "retirado" {
			return "", nil, fmt.Errorf("estado inválido")
		}
		if c.Op == "neq" {
			activo = !activo
		}
		if activo {
			return periodoAbierto, nil, nil
		}
		return "NOT " + periodoAbierto, nil, nil

	case kSelect:
		switch c.Op {
		case "set":
			return spec.Expr + " IS NOT NULL", nil, nil
		case "unset":
			return spec.Expr + " IS NULL", nil, nil
		case "eq", "neq":
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return "", nil, fmt.Errorf("valor inválido para %s", spec.Label)
			}
			if c.Op == "neq" {
				return "(" + spec.Expr + " IS NULL OR " + spec.Expr + " <> ?)", []any{id}, nil
			}
			return spec.Expr + " = ?", []any{id}, nil
		}
		return "", nil, opErr(spec.Label, c.Op)

	case kNumber:
		n, err := strconv.Atoi(v)
		if err != nil {
			return "", nil, fmt.Errorf("valor numérico inválido para %s", spec.Label)
		}
		op, ok := cmpOps[c.Op]
		if !ok {
			return "", nil, opErr(spec.Label, c.Op)
		}
		return fmt.Sprintf("%s %s ?", spec.Expr, op), []any{n}, nil

	case kDate:
		if _, err := time.Parse("2006-01-02", v); err != nil {
			return "", nil, fmt.Errorf("fecha inválida para %s (AAAA-MM-DD)", spec.Label)
		}
		op, ok := cmpOps[c.Op]
		if !ok {
			return "", nil, opErr(spec.Label, c.Op)
		}
		return fmt.Sprintf("%s %s ?", spec.Expr, op), []any{v}, nil

	case kText:
		switch c.Op {
		case "contains":
			return spec.Expr + " LIKE ?", []any{"%" + v + "%"}, nil
		case "ncontains":
			return "(" + spec.Expr + " IS NULL OR " + spec.Expr + " NOT LIKE ?)", []any{"%" + v + "%"}, nil
		case "starts":
			return spec.Expr + " LIKE ?", []any{v + "%"}, nil
		case "eq":
			return spec.Expr + " = ?", []any{v}, nil
		case "neq":
			return "(" + spec.Expr + " IS NULL OR " + spec.Expr + " <> ?)", []any{v}, nil
		}
		return "", nil, opErr(spec.Label, c.Op)
	}
	return "", nil, fmt.Errorf("tipo de campo no soportado")
}

func opErr(label, op string) error {
	return fmt.Errorf("operador %q no válido para %s", op, label)
}

// ---- Metadatos para el constructor de filtros del frontend ----------------

// FilterField describe un campo filtrable y sus operadores para la UI.
type FilterField struct {
	Key     string          `json:"key"`
	Label   string          `json:"label"`
	Type    string          `json:"type"`    // text|number|date|select|estado
	Options string          `json:"options"` // catálogo para select
	Ops     []OperatorLabel `json:"ops"`
}

type OperatorLabel struct {
	Op    string `json:"op"`
	Label string `json:"label"`
}

var opsPorTipo = map[fieldKind][]OperatorLabel{
	kText: {
		{"contains", "contiene"}, {"ncontains", "no contiene"},
		{"eq", "es igual a"}, {"neq", "no es igual a"}, {"starts", "empieza por"},
	},
	kNumber: {
		{"eq", "es igual a"}, {"neq", "no es igual a"},
		{"gt", "mayor que"}, {"lt", "menor que"}, {"gte", "mayor o igual"}, {"lte", "menor o igual"},
	},
	kDate: {
		{"eq", "es igual a"}, {"neq", "no es igual a"},
		{"gt", "después de"}, {"lt", "antes de"}, {"gte", "desde"}, {"lte", "hasta"},
	},
	kSelect: {
		{"eq", "es igual a"}, {"neq", "no es igual a"},
		{"set", "está definido"}, {"unset", "no está definido"},
	},
	kEstado: {
		{"eq", "es igual a"}, {"neq", "no es igual a"},
	},
}

var kindName = map[fieldKind]string{kText: "text", kNumber: "number", kDate: "date", kSelect: "select", kEstado: "estado"}

// FilterFields devuelve los campos filtrables (orden estable) para la UI.
func FilterFields() []FilterField {
	orden := []string{"nombres", "apellidos", "cedula", "telefono", "edad", "tipo_sangre",
		"fecha_inscripcion", "fecha_registro", "escuela", "maestro",
		"estado", "ciudad", "municipio", "parroquia", "cinturon", "estado_actividad"}
	out := make([]FilterField, 0, len(orden))
	for _, k := range orden {
		s := atletaFields[k]
		out = append(out, FilterField{
			Key: k, Label: s.Label, Type: kindName[s.Kind], Options: s.Options, Ops: opsPorTipo[s.Kind],
		})
	}
	return out
}
