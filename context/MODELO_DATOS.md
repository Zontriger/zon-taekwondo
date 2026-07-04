# Modelo de Datos — Sistema de Gestión de Atletas (Taekwondo, Edo. Miranda)

> **Cómo usar este archivo con Claude Code**
> Este documento explica el *por qué* del modelo. El esquema ejecutable está en
> `schema.sql` (SQLite), que es la **fuente de verdad** de las tablas. Ante
> cualquier duda de estructura, `schema.sql` manda; ante cualquier duda de
> criterio, mandan las "Decisiones de negocio" de abajo. No cambies estas reglas
> sin confirmarlo.

---

## Decisiones de negocio (ya cerradas)

Estas decisiones resuelven las ambigüedades del requerimiento original. Están
reflejadas en el esquema y deben respetarse.

1. **Usuario único = rol único, no cuenta única.** El único *tipo* de usuario que
   inicia sesión es el **entrenador**. Puede existir **más de un entrenador**, cada
   uno con su cuenta. No hay login para atletas ni representantes. Un entrenador
   puede marcarse como **administrador** (`es_admin`), lo que habilita acciones
   sensibles (eliminar atletas, backups, datos de menores).
2. **Permisos:** todos los entrenadores ven **toda la asociación** (no está
   restringido por escuela). Es una asociación estatal pequeña.
3. **Entrenador ↔ Escuela:** un entrenador pertenece a **una** escuela
   (uno-a-muchos). Una escuela tiene **un** municipio y **varios** entrenadores.
4. **Antigüedad y reactivación:** el estado activo/retirado se modela como
   **periodos** (`periodo_actividad`), no como un campo. Reactivar = abrir un nuevo
   periodo. La antigüedad ("fecha transcurrida") se calcula desde
   `atleta.fecha_inscripcion` (la primera inscripción), que **nunca se pierde** al
   retirar/reactivar.
5. **Cinturón como línea de tiempo:** el cinturón actual **no es un campo editable**;
   es la fila más reciente de `historial_cinturon`. Cambiar de cinta = agregar una
   fila nueva.
6. **DAN (1–9):** solo aplica cuando el cinturón es **negro**. Si no es negro, `dan`
   debe ir en NULL. (Regla validada a nivel de aplicación.)
7. **Representante:** solo para menores de 18. Se **conserva como histórico** aunque
   el atleta cumpla la mayoría de edad (no se borra).
8. **Fecha de inscripción con día opcional:** se guarda como fecha completa usando
   día `01` cuando el día real es desconocido, y `inscripcion_dia_exacto = 0` marca
   ese caso. Al mostrarla, si el día no es exacto, mostrar solo mes/año.
9. **Auditoría:** cada cambio relevante se registra en `auditoria` (quién, qué
   tabla, qué registro, cuándo).
10. **Cédula compuesta (tipo + número):** la cédula se compone de un **tipo**
    (`V`, `E` o `P`) y un **número**. La combinación *(tipo, número)* es **única**
    entre atletas. Ambos son NULL-ables (un menor puede no tener cédula). El
    representante también usa tipo+número, pero **sin** restricción de unicidad.
11. **Teléfonos:** el atleta tiene un **teléfono principal** (`atleta.telefono`,
    requerido a nivel app) y de **0 a 3 teléfonos de contacto** adicionales en la
    tabla `atleta_telefono_contacto`.

### Puntos de extensión (documentados, NO implementados en v1)
- **Historial de escuela del atleta:** hoy `atleta.escuela_id` guarda solo la escuela
  actual. Si en el futuro se quiere rastrear traslados entre escuelas, se agrega una
  tabla `historial_escuela` sin romper lo existente.
- **Entrenador en varias escuelas:** hoy es uno-a-muchos. Si algún entrenador da
  clases en varias sedes, se migra a una tabla puente `entrenador_escuela`.

---

## Entidades

| Entidad | Rol en el sistema |
|---|---|
| `estado`, `municipio` | Jerarquía geográfica reutilizable (dirección y ubicación de escuelas). |
| `cinturon` | Catálogo de colores + orden de progresión; marca cuál habilita DAN. |
| `escuela` | Sede; un municipio, varios entrenadores y atletas. |
| `entrenador` | Único rol de usuario. Tiene credenciales, cinturón/DAN, `es_admin` y estado. |
| `atleta` | Ficha central: datos personales, foto, dirección, escuela e inscripción. |
| `atleta_telefono_contacto` | 0..3 teléfonos de contacto adicionales del atleta. |
| `representante` | 1:1 opcional con el atleta (solo menores). |
| `historial_cinturon` | Línea de tiempo de grados; de aquí se deriva el cinturón actual. |
| `periodo_actividad` | Periodos activo/retirado; soporta reactivación y antigüedad. |
| `auditoria` | Trazabilidad de cambios por entrenador. |

---

## Valores derivados (no se almacenan; se calculan)

- **Edad** = hoy − `atleta.fecha_nacimiento`. Determina si se exige representante (< 18).
- **Cinturón actual** = fila de `historial_cinturon` con `fecha_cambio` máxima.
- **Estado actual** = ¿existe un `periodo_actividad` con `fecha_fin` NULL? → *activo*; si no → *retirado*.
- **Fecha de retiro** = `fecha_fin` del último periodo cerrado.
- **Antigüedad** = hoy − `atleta.fecha_inscripcion`.

---

## Reglas de integridad a validar en la aplicación

Estas no las puede garantizar SQLite por sí solo:

1. Si el cinturón (del atleta o del entrenador) **no** es negro → `dan` = NULL.
   Si **es** negro → `dan` obligatorio entre 1 y 9.
2. Si el atleta es **menor de 18** → exigir datos de `representante`. El **teléfono
   principal** del atleta es requerido siempre (en menores puede tomarse el del
   representante si no se indica otro).
3. Un atleta no debe tener **dos** `periodo_actividad` abiertos (con `fecha_fin` NULL)
   simultáneamente.
4. El primer `periodo_actividad.fecha_inicio` debe coincidir con
   `atleta.fecha_inscripcion`.
5. Los datos sensibles de menores (foto, cédula, teléfonos) y los **backups** que los
   contienen solo deben ser accesibles/descargables por el administrador.
6. **Cédula:** si se indica número, el tipo (`V`/`E`/`P`) es obligatorio y viceversa.
   La combinación *(tipo, número)* no puede repetirse entre atletas.
7. **Teléfonos de contacto:** máximo 3 por atleta.

---

## Notas para la generación de reportes PDF y búsqueda

- La **búsqueda por facetas** (escuela, cinturón, estado, municipio, rango de edad,
  rango de fechas de inscripción) se traduce en filtros sobre `atleta` + joins a
  `escuela`, `municipio` y a los valores derivados de cinturón/estado. Además del
  filtro simple existe un **filtro avanzado tipo Odoo**: un dominio booleano
  (coincidir *todas*/*cualquiera* de una lista de condiciones campo–operador–valor).
- Los reportes PDF (con o sin filtros) usan exactamente el mismo conjunto de filtros
  que la búsqueda, más un resumen agregado (totales, activos/retirados, distribución
  por cinturón).
