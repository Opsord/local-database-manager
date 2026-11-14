# Especificación de diseño: `config.yml` de la aplicación

## 1. Objetivo

Centralizar knobs de runtime (intervalo de health check de Docker/Podman, y más adelante timeouts) en un YAML **siempre presente en el repositorio**. Los valores por defecto viven **solo en ese archivo**. El código Go guarda el resultado en variables/campos; no duplica literales como `5 * time.Second`.

## 2. Alcance (v1)

**Incluye**

- Archivo tracked `config.yml` en la raíz del proyecto (junto a `engines/` e `instances/`).
- Una clave leída: `engine_health_interval`.
- Carga al arrancar; la TUI usa `cfg.EngineHealthInterval` para el tick de health.
- Comentarios en el YAML con claves futuras **no leídas**.

**No incluye**

- UI para editar config.
- Recarga en caliente (`r` no relee `config.yml`).
- Flags CLI que pisen el YAML.
- Perfil por usuario (`~/.config/...`).
- Implementar las claves comentadas (`engine_health_timeout`, compose, mensajes de status).

## 3. Archivo

Ruta: `<projectRoot>/config.yml`  
`projectRoot` es el que ya resuelve `findProjectRoot()` (directorio con `engines/` e `instances/`).

El archivo se commitea. Contenido inicial:

```yaml
# Local Database Manager — app settings.
# Durations use Go syntax: 5s, 10s, 1m.

# How often to re-check Docker / Podman daemon health.
engine_health_interval: 5s

# --- not read yet ---
# engine_health_timeout: 3s
# compose_start_timeout: 45s
# status_message_timeout: 4s
```

Cualquier clave comentada o desconocida se ignora. Añadir soporte es un cambio de código aparte.

## 4. Modelo en Go

Paquete: `internal/config`.

```go
type Config struct {
    EngineHealthInterval time.Duration `yaml:"engine_health_interval"`
}
```

`time.Duration` no unmarshal-ea `"5s"` con yaml.v3 de forma nativa. Usar un tipo wrapper (o string + `time.ParseDuration`) para aceptar sintaxis Go (`5s`, `500ms`, `1m`).

Dependencia: `gopkg.in/yaml.v3`.

API:

- `Load(projectRoot string) (Config, error)` lee `filepath.Join(projectRoot, "config.yml")`.
- `main` llama `Load` **antes** de `app.NewApp` y pasa `Config` al constructor (o a un campo del modelo).
- Eliminar el `const engineHealthInterval = 5 * time.Second` de `internal/app/tui.go`.

No hay constante de default en código. El único “default” es el valor en el YAML del repo.

## 5. Errores (fail fast)

Fallar al arrancar con mensaje que incluya la ruta, **sin** fallback silencioso (un intervalo `0` dispararía health checks en loop):

| Situación | Resultado |
|-----------|-----------|
| Archivo ausente | error |
| YAML inválido | error |
| Falta `engine_health_interval` | error |
| Valor vacío, `0`, o negativo | error |
| `ParseDuration` falla | error |

`main` imprime el error y `os.Exit(1)`.

## 6. Datos y ciclo de vida

```
findProjectRoot()
  → config.Load(projectRoot)
  → app.NewApp(projectRoot, cfg)
  → tea.Tick(cfg.EngineHealthInterval, engineHealthTickMsg)
```

El intervalo se fija al arranque. Cambiar el YAML requiere reiniciar la TUI.

## 7. Pruebas

- `Load` con YAML válido → duration esperada.
- Falta archivo / YAML roto / clave ausente / `0s` / `-1s` / `"nope"` → error.
- El tick de health usa el valor cargado (test del modelo o del cmd de tick con un `Config` de test, sin depender de un default en código).

## 8. Documentación

Mencionar `config.yml` en `README.md` (una o dos líneas: ruta, clave v1, reinicio para aplicar cambios).
