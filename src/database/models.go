package database

// Structs de dominio. Los campos opcionales usan puntero para distinguir
// "ausente/NULL" de "valor cero" al serializar a JSON.

type Entrenador struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Nombres   string `json:"nombres"`
	Apellidos string `json:"apellidos"`
	EsAdmin   bool   `json:"es_admin"`
	Estado    string `json:"estado"`
}

type Cinturon struct {
	ID     int64  `json:"id"`
	Color  string `json:"color"`
	Orden  int    `json:"orden"`
	EsNegro bool  `json:"es_negro"`
}

type Estado struct {
	ID     int64  `json:"id"`
	Nombre string `json:"nombre"`
}

type Ciudad struct {
	ID       int64  `json:"id"`
	EstadoID int64  `json:"estado_id"`
	Nombre   string `json:"nombre"`
}

type Municipio struct {
	ID       int64  `json:"id"`
	EstadoID int64  `json:"estado_id"`
	CiudadID *int64 `json:"ciudad_id"`
	Nombre   string `json:"nombre"`
}

type Parroquia struct {
	ID          int64  `json:"id"`
	EstadoID    int64  `json:"estado_id"`
	MunicipioID int64  `json:"municipio_id"`
	Nombre      string `json:"nombre"`
}

type Escuela struct {
	ID           int64   `json:"id"`
	Nombre       string  `json:"nombre"`
	MunicipioID  int64   `json:"municipio_id"`
	MunicipioNom string  `json:"municipio_nombre,omitempty"`
	Direccion    *string `json:"direccion"`
	Activa       bool    `json:"activa"`
}

type Representante struct {
	CedulaTipo   *string `json:"cedula_tipo"`
	CedulaNumero *string `json:"cedula_numero"`
	Nombres      *string `json:"nombres"`
	Apellidos    *string `json:"apellidos"`
	Telefono     *string `json:"telefono"`
	Parentesco   *string `json:"parentesco"`
}

// HistorialCinturon es una fila de la línea de tiempo de grados.
type HistorialCinturon struct {
	ID           int64   `json:"id"`
	CinturonID   int64   `json:"cinturon_id"`
	Color        string  `json:"color,omitempty"`
	Dan          *int    `json:"dan"`
	FechaCambio  string  `json:"fecha_cambio"`
	RegistradoPor *int64 `json:"registrado_por"`
}

// Periodo de actividad; FechaFin nil = actualmente activo.
type Periodo struct {
	ID           int64   `json:"id"`
	FechaInicio  string  `json:"fecha_inicio"`
	FechaFin     *string `json:"fecha_fin"`
	MotivoRetiro *string `json:"motivo_retiro"`
}

// Atleta es la ficha central. Los valores derivados (edad, cinturón actual,
// estado, antigüedad) se calculan y adjuntan al leer, no se almacenan.
type Atleta struct {
	ID                   int64    `json:"id"`
	FotoPath             *string  `json:"foto_path"`
	Nombres              string   `json:"nombres"`
	Apellidos            string   `json:"apellidos"`
	CedulaTipo           *string  `json:"cedula_tipo"`
	CedulaNumero         *string  `json:"cedula_numero"`
	FechaNacimiento      string   `json:"fecha_nacimiento"`
	Telefono             *string  `json:"telefono"`
	TelefonosContacto    []string `json:"telefonos_contacto"`
	EstadoID             *int64   `json:"estado_id"`
	CiudadID             *int64   `json:"ciudad_id"`
	MunicipioID          *int64   `json:"municipio_id"`
	ParroquiaID          *int64   `json:"parroquia_id"`
	DireccionDetalle     *string `json:"direccion_detalle"`
	EscuelaID            *int64  `json:"escuela_id"`
	FechaInscripcion     string  `json:"fecha_inscripcion"`
	InscripcionDiaExacto bool    `json:"inscripcion_dia_exacto"`

	// Derivados / enriquecidos para la UI.
	Edad          int     `json:"edad"`
	EsMenor       bool    `json:"es_menor"`
	EscuelaNombre string  `json:"escuela_nombre,omitempty"`
	EstadoNom     string  `json:"estado_nombre,omitempty"`
	CiudadNom     string  `json:"ciudad_nombre,omitempty"`
	MunicipioNom  string  `json:"municipio_nombre,omitempty"`
	ParroquiaNom  string  `json:"parroquia_nombre,omitempty"`
	CinturonColor string  `json:"cinturon_color,omitempty"`
	CinturonDan   *int    `json:"cinturon_dan,omitempty"`
	Estado        string  `json:"estado"` // activo / retirado

	// Relaciones cargadas solo en el detalle.
	Representante *Representante       `json:"representante,omitempty"`
	Cinturones    []HistorialCinturon `json:"cinturones,omitempty"`
	Periodos      []Periodo           `json:"periodos,omitempty"`
}

// Condicion es una comparación individual del filtro avanzado.
type Condicion struct {
	Field string `json:"field"` // clave de campo (lista blanca)
	Op    string `json:"op"`    // operador (lista blanca por tipo)
	Value string `json:"value"` // valor a comparar
}

// Dominio es un filtro booleano tipo Odoo: coincidir TODAS ("all") o
// CUALQUIERA ("any") de una lista de condiciones.
type Dominio struct {
	Match      string      `json:"match"` // "all" | "any"
	Condiciones []Condicion `json:"conditions"`
}

// AtletaFiltro reúne la búsqueda rápida, el filtro avanzado y la paginación.
type AtletaFiltro struct {
	Texto   string   // búsqueda rápida (nombre, apellido o cédula)
	Dominio *Dominio // filtro avanzado (opcional)
	Limit   int
	Offset  int
}
