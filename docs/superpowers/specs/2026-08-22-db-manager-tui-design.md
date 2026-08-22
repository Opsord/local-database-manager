# Especificación de Diseño: Local Database Manager TUI (Go + Bubble Tea)

## 1. Visión General y Objetivos
Crear una herramienta CLI con interfaz de terminal interactiva (TUI) en Go, ultra ligera y de alto rendimiento, para gestionar de forma centralizada múltiples bases de datos locales (PostgreSQL, SQL Server, etc.) ejecutadas en contenedores Docker y Podman.

### Objetivos Clave
- **Consumo mínimo de recursos:** < 15 MB de RAM en reposo, 0% CPU en espera.
- **Binario único e independiente:** Compila a un ejecutable `.exe` sin dependencias de Node, Python o runtimes externos.
- **Carpeta Global de Instancias:** Todas las instancias se configuran como archivos independientes dentro de `instances/*.env`.
- **Configuración declarativa por variables:** Cada archivo `.env` declara su `ENGINE`, `RUNTIME`, `CONTAINER_NAME`, puertos y credenciales.
- **Control de estado visual:** Indicadores de estado (`🟢 RUNNING`, `🔴 STOPPED`) al inicio de cada elemento.
- **Nombres de contenedor personalizados:** Cada instancia define su `CONTAINER_NAME` y `COMPOSE_PROJECT_NAME`, evitando nombres por defecto basados en carpetas.
- **Asistente de Creación (`[n]`):** Wizard interactivo para generar nuevas instancias en `instances/` con detección automática de puertos libres.
- **Acciones rápidas:** Iniciar/detener con `Espacio`, copiar URI con `c`, streaming de logs con `l`, destruir volumen con `d`, editar con `e`.

---

## 2. Estructura de Directorios del Proyecto

```text
local-database-manager/
├── engines/                         # Plantillas maestras de compose y scripts
│   ├── postgres/
│   │   ├── docker-compose.yml
│   │   ├── podman-compose.yml
│   │   └── init/                    # Scripts SQL/sh de inicialización base
│   └── sql-server/
│       ├── docker-compose.yml
│       ├── docker-entrypoint.sh
│       └── init/
│
├── instances/                       # 📁 CARPETA GLOBAL DE INSTANCIAS (Opción A)
│   ├── super_calendar.env
│   ├── requerimientos.env
│   └── .template.env                # Plantilla de referencia
│
├── cmd/
│   └── db-manager/
│       └── main.go                  # Punto de entrada de la aplicación
│
├── internal/
│   ├── app/                         # Modelo principal de Bubble Tea y vistas
│   │   ├── tui.go                   # Inicialización y ciclo de vida de la TUI
│   │   ├── view_main.go             # Split view (lista + panel de detalles)
│   │   ├── view_wizard.go           # Formulario interactivo para crear instancias
│   │   ├── view_logs.go             # Viewport de streaming de logs
│   │   └── styles.go                # Temas y estilos con Lip Gloss
│   ├── core/                        # Lógica de dominio
│   │   ├── instance.go              # Estructura de Instancia y parser de .env
│   │   ├── scanner.go               # Escáner de instances/*.env y engines/
│   │   ├── runner.go                # Invocador de docker/podman compose CLI
│   │   ├── ports.go                 # Detección de puertos libres y en uso
│   │   └── clipboard.go             # Integración con portapapeles de Windows
│   └── config/                      # Rutas base y constantes
│
├── go.mod
├── go.sum
└── docs/
    └── superpowers/specs/2026-08-22-db-manager-tui-design.md
```

---

## 3. Modelo de Instancia y Formato de `.env`

### 3.1 Estructura de un archivo `instances/<nombre>.env`
Ejemplo PostgreSQL (`instances/super_calendar.env`):
```env
ENGINE=postgres              # postgres | sqlserver
RUNTIME=docker               # docker | podman

CONTAINER_NAME=pg-super-calendar
COMPOSE_PROJECT_NAME=pg-super-calendar

POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=super_calendar_db
POSTGRES_SCHEMA=public
POSTGRES_VOLUME=pgdata_super_calendar
```

Ejemplo SQL Server (`instances/requerimientos.env`):
```env
ENGINE=sqlserver
RUNTIME=docker

CONTAINER_NAME=sql-requerimientos
COMPOSE_PROJECT_NAME=sql-requerimientos

SQLSERVER_PORT=1433
SA_PASSWORD=SuperSecurePass123!
SQLSERVER_DB=requerimientos_db
SQLSERVER_VOLUME=sqlserver_requerimientos
```

### 3.2 Adaptación de Compose Files (`engines/`)
Los archivos `docker-compose.yml` y `podman-compose.yml` dentro de `engines/` se configuran para consumir estas variables:
```yaml
services:
  database:
    container_name: ${CONTAINER_NAME}
    image: postgres:18 # o mcr.microsoft.com/mssql/server
    restart: unless-stopped
    ports:
      - "${POSTGRES_PORT}:5432"
    environment:
      POSTGRES_DB: ${POSTGRES_DB}
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_SCHEMA: ${POSTGRES_SCHEMA}
    volumes:
      - ${POSTGRES_VOLUME}:/var/lib/postgresql
      - ./init:/docker-entrypoint-initdb.d:ro

volumes:
  pgdata:
    name: ${POSTGRES_VOLUME}
```

### 3.3 Invocación de Comandos
Cuando la TUI ejecuta una acción sobre una instancia, ejecuta:
* **Start:**
  `docker compose -p <COMPOSE_PROJECT_NAME> -f engines/<ENGINE>/docker-compose.yml --env-file instances/<NAME>.env up -d`
* **Stop:**
  `docker compose -p <COMPOSE_PROJECT_NAME> -f engines/<ENGINE>/docker-compose.yml --env-file instances/<NAME>.env down`
* **Down -v (Purge):**
  `docker compose -p <COMPOSE_PROJECT_NAME> -f engines/<ENGINE>/docker-compose.yml --env-file instances/<NAME>.env down -v`
* **Logs:**
  `docker compose -p <COMPOSE_PROJECT_NAME> -f engines/<ENGINE>/docker-compose.yml --env-file instances/<NAME>.env logs --tail=100 -f`

---

## 4. Diseño de la Interfaz de Usuario (TUI)

### 4.1 Vista Principal (Split View)
```text
┌─── Instancias de BD ─────────────────────────┐┌─── Detalles / Conexión ────────────────────────┐
│ > 🟢 [Docker] Postgres : super_calendar      ││ Motor:          PostgreSQL 18 (Docker)         │
│   🔴 [Podman] Postgres : test_local          ││ Contenedor:     pg-super-calendar              │
│   🟢 [Docker] SQL Server : requerimientos    ││ Estado:         🟢 RUNNING                     │
│                                              ││ Puerto Host:    5432                           │
│                                              ││ Base de datos:  super_calendar_db              │
│                                              ││ Usuario:        postgres                       │
│                                              ││ Esquema:        public                         │
│                                              ││ Volumen:        pgdata_super_calendar          │
│                                              ││ Archivo:        instances/super_calendar.env   │
│                                              ││                                                │
│                                              ││ Connection String ([c] para copiar):           │
│                                              ││ postgresql://postgres:***@localhost:5432/db    │
│                                              ││                                                │
│                                              ││ Comando CLI:                                   │
│                                              ││ psql -h localhost -p 5432 -U postgres          │
└──────────────────────────────────────────────┘└────────────────────────────────────────────────┘
┌─── Atajos de Teclado ──────────────────────────────────────────────────────────────────────────┐
│ [↑/↓] Navegar  [Espacio] Start/Stop  [c] Copiar URI  [l] Logs  [n] Nueva Instancia  [d] Down -v│
│ [e] Editar .env  [r] Recargar lista  [q] Salir                                                 │
└────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### 4.2 Asistente de Creación de `.env` (`[n]`)
1. **Paso 1 - Motor:** Selector `[Postgres]` o `[SQL Server]`.
2. **Paso 2 - Runtime:** Selector `[Docker]` o `[Podman]`.
3. **Paso 3 - Nombre:** Input para el nombre de la instancia (ej. `tienda_app`).
4. **Paso 4 - Nombre Contenedor:** Sugiere automáticamente `pg-tienda-app` o `sql-tienda-app`.
5. **Paso 5 - Puerto:** Escanea todos los `.env` existentes y puertos locales ocupados, sugiriendo el primer puerto libre (ej. `5434`).
6. **Paso 6 - Base de datos y Volumen:** Sugeridos automáticamente a partir del nombre (`tienda_app_db` y `pgdata_tienda_app`).
7. **Paso 7 - Guardar:** Genera `instances/tienda_app.env` y vuelve a la lista con la nueva instancia seleccionada.

---

## 5. Tabla de Atajos de Teclado

| Tecla | Acción | Descripción |
| :--- | :--- | :--- |
| `↑` / `↓` o `k` / `j` | Navegar | Cambia la instancia seleccionada |
| `Espacio` / `Enter` | Start / Stop | Ejecuta `up -d` o `down` según el estado actual |
| `c` | Copiar URI | Copia la URI de conexión (`postgresql://...`) al portapapeles de Windows |
| `l` | Ver Logs | Abre el visor de streaming de logs a pantalla completa |
| `n` | Nueva Instancia | Abre el asistente interactivo para crear una nueva instancia |
| `e` | Editar `.env` | Abre el archivo `instances/<name>.env` en el editor por defecto (`code` o `notepad`) |
| `d` | Down -v (Purge) | Destruye contenedor y volumen de datos con confirmación `¿Estás seguro? (y/N)` |
| `r` | Recargar | Re-escanea la carpeta `instances/` y actualiza estados de contenedores |
| `q` / `Ctrl+C` | Salir | Cierra la TUI |

---

## 6. Plan de Migración de `db-instances/`
Para una transición fluida desde la estructura anterior:
1. Crear la carpeta `engines/` y mover allí los `docker-compose.yml`, `podman-compose.yml` e `init/`.
2. Crear la carpeta `instances/`.
3. Migrar `.env.super_calendar` y `.env.requerimientos` a `instances/super_calendar.env` e `instances/requerimientos.env` agregando las claves `ENGINE`, `RUNTIME`, `CONTAINER_NAME` y `COMPOSE_PROJECT_NAME`.
4. Eliminar o archivar la carpeta anterior `db-instances/` una vez verificado.
