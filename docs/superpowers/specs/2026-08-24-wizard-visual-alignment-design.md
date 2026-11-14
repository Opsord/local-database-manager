# Spec: Alineación visual del wizard de creación de instancias

Fecha: 2026-08-24  
Estado: propuesto (diagnóstico + diseño; sin implementación en este cambio)  
Alcance: vista `ModeWizard` (`internal/app/view_wizard.go`) y helpers de superficie ya usados por el menú principal.

## 1. Problema

El menú principal (`view_main.go`) ya tiene chrome Night Owl estable: cada fila pinta `Background(BgSurface)`, los paneles tienen ancho/alto fijos y el texto no se sale de la caja.

El asistente de creación (`[n]` → `viewWizard`) reutiliza colores y `ActivePanelStyle`, pero **no** el contrato de layout del menú principal. El resultado es un modal flotante con agujeros negros, regla separadora corta, campos que parecen todos activos, valores crudos (`postgres` / `docker`) y wrap que rompe el borde.

Esto no es un tema de paleta. El commit `6b94720` (`fix(tui): fill Night Owl panel backgrounds…`) corrigió lista, detalles y footer con `surfaceLine` / `Background(BgSurface)` en cada estilo. El wizard recibió un parche parcial (gaps y `JoinHorizontal`) y se quedó a medias.

## 2. Evidencia (render TrueColor, 120×32)

Celdas sin `48;2` de fondo (aprox. “agujeros” de terminal default):

| Vista | `cellsWithoutBG` |
| --- | ---: |
| Main | 28 |
| Wizard (paso Engine) | 385 |
| Wizard (Review) | **957** |
| Help overlay | 523 |
| Action menu overlay | 505 |

Dump visible del review (el wrap de nombres largos es peor):

```text
┌────────────────────────────────────────────────────────────────────────┐
│ New Database Instance                                                  │
│ ────────────────────────────────────────────────────────────────────   │  ← regla corta
│ 1. Engine:     postgres                                                │  ← id crudo
│ 2. Runtime:    docker                                                  │
│ 3. Name:       > my_new_instance                                       │  ← prompt en todos
│ 4. Container:  > pg-this-is-a-very-long-container-name-that-will-      │
│ overflow                                                               │  ← wrap rompe la caja
│ 8. Password:   > postgres                                              │  ← texto plano
└────────────────────────────────────────────────────────────────────────┘
```

El menú principal, en el mismo tamaño, llena header + split + footer sin wrap.

## 3. Causas raíz (código)

1. **Filas sin `surfaceLine(innerWidth)`.** El wizard hace `JoinVertical` de fragmentos. Lip Gloss rellena el `Width` del panel *después* de un `ESC[0m` → el padding queda sin `BgSurface`. El main evita esto con `surfaceLine` / `listLineStyle.Width`.
2. **`styleTextInput` solo pinta foreground.** Prompt, texto, placeholder y cursor no setean `Background(BgSurface)`. `bubbles/textinput` hace `Inline(true)` y termina en reset. Comentario ya existente en `styles.go`: *“Lip Gloss emits SGR reset when only foreground is set, which punches black holes through the panel.”*
3. **`textinput.Width == 0`.** Sin ancho, un valor largo hace wrap y la siguiente línea nace pegada al borde izquierdo (`overflow` suelto).
4. **Focus sucio.** `Focus()` del siguiente input no hace `Blur()` del anterior. El `Prompt` default `"> "` se renderiza en todos los campos, focused o no.
5. **Separador `boxWidth-4`.** El inner real es `boxWidth - 2` (padding horizontal). La regla queda 2–4 celdas corta.
6. **Labels de display vs ids.** En el paso activo se muestra `Postgres` / `SQL Server`; al confirmar se pinta `w.engines[i]` / `w.runtimes[i]` (`postgres`, `docker`).
7. **Overlay sin chrome.** `renderOverlay` usa `lipgloss.Place` sin `WithWhitespaceBackground(BgDark)`. El wizard no reusa header/footer del main. Help y action menu comparten este overlay; el arreglo de `Place` es compartido, el resto del polish es del wizard.
8. **Password en `EchoNormal`.** Fallo visual y de higiene; el main nunca muestra el secreto en el panel de detalles.

Fuera de este spec (YAGNI): rediseñar el flujo a un formulario de una sola página, split preview, o navegación atrás. Hoy no se puede editar un paso anterior; se deja como está salvo que haga falta para no mentir visualmente (campos completados se ven como valores, no como inputs).

## 4. Enfoques

### A. Reusar el contrato del main (recomendado)

Aplicar al wizard lo que ya funciona: `surfaceLine`, `panelTitle`/`panelSeparator`, `styleTextInput` con fondo, `Width` en inputs, `Place(..., WithWhitespaceBackground(BgDark))`.

- Pros: diff corto, mismos helpers, tests de ancho ya existen.
- Cons: el wizard sigue siendo un modal centrado, no un split.

### B. Rehacer el wizard como pantalla full-chrome

Header + panel de formulario + panel preview + footer de atajos, igual que el main.

- Pros: máxima consistencia.
- Cons: reescritura de layout; no hace falta para cerrar los agujeros.

### C. Formulario único (todos los campos a la vez)

Un `huh`/`bubbles` form en lugar de 10 pasos.

- Pros: menos estados.
- Cons: cambia UX y no ataca el bug de superficie.

**Decisión:** A. El main ya es la referencia visual; el wizard debe usar las mismas primitivas.

## 5. Diseño (enfoque A)

### 5.1 Overlay compartido

`renderOverlay` debe pintar el whitespace del `Place` con `BgDark`:

```go
lipgloss.Place(m.width-2, m.height, lipgloss.Center, lipgloss.Center, modal,
    lipgloss.WithWhitespaceBackground(BgDark))
```

Esto cubre wizard, help y action menu. No cambia su contenido.

### 5.2 Caja del wizard

- Ancho: el actual (`min(72, width-12)`, piso 50) se mantiene.
- Inner: `panelInnerWidth(boxWidth)` — misma función que el main.
- Cada fila (título, separador, campos, hint) se envuelve en `surfaceLine(inner, …)`.
- Separador: `SeparatorStyle.Width(inner).Render(strings.Repeat("─", inner))` (o extraer `panelSeparator` si se reintroduce; hoy no está en `helpers.go`).

### 5.3 Campos

- Pasos Engine/Runtime **activos**: chips `[Postgres]` / `SQL Server` como ahora.
- Pasos Engine/Runtime **completos**: `ValueHighlightStyle` con **display names** (`Postgres`, `SQL Server`, `Docker`, `Podman`), no ids.
- Pasos de texto **activos**: un `textinput` con `Prompt=""`, `Width` = inner − label(14) − gap(1), estilos con `Background(BgSurface)`.
- Pasos de texto **completos** y Review: valor estático (`ValueStyle` / `ValueHighlightStyle`), no `input.View()`. Visualmente honestos: no parecen editables.
- Password: `EchoMode = textinput.EchoPassword` desde `newWizardModel`.
- Al cambiar de paso: `Blur()` todos los inputs y `Focus()` solo el actual.

### 5.4 Copy (inglés, AGENTS.md)

Sin cambios de idioma. Opcional mínimo: el hint de review se queda. No añadir “Step 3 of 10” en esta iteración.

### 5.5 Tests

Nuevo `internal/app/view_wizard_test.go`:

- Render 120×32 y 80×24 en `StepReview`: ninguna línea con `lipgloss.Width > termWidth`; el contenido del modal no hace wrap de un valor largo (se trunca o cabe en `textinput.Width`).
- Review muestra `Postgres` y `Docker`, nunca `postgres` / `docker` como label visible.
- Password no aparece en claro en `View()` (el valor `postgres` no está en el dump stripped).
- Prompt `>` no aparece en campos completados.
- Forzar `termenv.TrueColor`: `cellsWithoutBG` del wizard review queda en el mismo orden de magnitud que main (objetivo: &lt; 80 en 120×32, hoy 957). Helper de conteo en el test, no en producción.

No se piden tests de TUI interactiva ni golden ANSI frágiles más allá de stripped text + métrica de fondo.

## 6. No objetivos

- No rediseñar pasos, validación, ni persistencia de `.env`.
- No convertir el wizard en split view.
- No reestilar help/action más allá del `Place` compartido.
- No añadir navegación atrás.

## 7. Criterio de hecho

El wizard, en 80×24 y 120×32, se lee como el mismo producto que el main: caja Night Owl llena, regla a ancho, un solo campo activo, labels humanos, password oculto, valores largos contenidos. Los tests de `./internal/app` pasan.
