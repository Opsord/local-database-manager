# Design: Postgres Version Selection + Automatic Volumes

**Date:** 2026-08-29  
**Status:** Approved for implementation planning  
**Scope:** Select Postgres major version in create/edit wizard; derive volume names automatically (read-only preview).

## Goal

Let users pick a curated Postgres major version per instance, and stop asking them to invent volume names. Volume names are derived from instance parameters so container config and persistent storage stay aligned and major upgrades do not accidentally reuse incompatible PGDATA.

## Non-goals

- SQL Server image/version selection
- Custom image tags (`alpine`, patch pins, private registries)
- Automatic data migration / `pg_upgrade` between majors
- Automatic cleanup of orphan volumes from previous majors (beyond the existing Purge of the *current* volume)

## Decisions

| Topic | Choice |
|-------|--------|
| Version UX | Curated list `14`–`18`, ←/→, default `18` |
| When selectable | Create **and** Edit |
| Storage of version | `POSTGRES_VERSION` in instance `.env` |
| Compose image | `postgres:${POSTGRES_VERSION}` |
| Volume UX | Derived automatically; shown as read-only preview (not a focusable step) |
| Volume naming (Postgres) | `pgdata_<instance_name>_<version>` |
| Volume naming (SQL Server) | `sqlserver_<instance_name>` (same auto-derive pattern; no version) |
| Major change behavior | New derived volume name; old volume left until manual Purge / engine cleanup |
| Existing instances | Missing `POSTGRES_VERSION` → treat as `18`; volume rewritten to derived form on next Edit **Save** |

## Data model

### Instance `.env` (Postgres)

```env
ENGINE=postgres
RUNTIME=docker
CONTAINER_NAME=pg-shop
COMPOSE_PROJECT_NAME=pg-shop
MEMORY_LIMIT=512M
POSTGRES_VERSION=16
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=...
POSTGRES_DB=shop
POSTGRES_SCHEMA=public
POSTGRES_VOLUME=pgdata_shop_16
```

### Compose templates

`engines/postgres/docker-compose.yml` and `podman-compose.yml`:

```yaml
image: postgres:${POSTGRES_VERSION}
```

Volume block continues to use `name: ${POSTGRES_VOLUME}`.

### Template

`instances/.env.template` documents `POSTGRES_VERSION` and that volume names are managed by the app.

### Core model

- `DatabaseInstance` gains `Version string` (empty for SQL Server).
- Parser: read `POSTGRES_VERSION`; if empty and engine is postgres, default display/runtime value to `18`.
- URI / connection behavior unchanged.

## Wizard UX

### Step order (Postgres)

1. Engine  
2. Runtime  
3. **Version** (←/→ among `14|15|16|17|18`)  
4. Name  
5. Container  
6. Port  
7. Database  
8. Password  
9. Memory  
10. Review  

SQL Server skips Version. Volume is **not** an editable step for either engine.

### Volume preview

- Shown as a muted, non-focusable row (e.g. after Database and again on Review).
- Recalculated live when Name or Version (Postgres) changes.
- Formulas:
  - Postgres: `pgdata_<sanitized_name>_<version>`
  - SQL Server: `sqlserver_<sanitized_name>`
- On Save, always write the derived volume into the `.env` (ignore any previous manual value).

### Create defaults

- Version: `18`
- Other autofill (container, port, db, password, memory) unchanged aside from volume derivation.

### Edit

- Preload Version from instance (default `18` if missing).
- Same editable fields as create, including Version.
- Volume preview updates live; Save persists derived volume + version.

## Edit warnings and restart

When Save causes the derived volume string to differ from the previously stored `POSTGRES_VOLUME` / `SQLSERVER_VOLUME`:

- Status message (English), e.g.  
  `Volume will change to pgdata_shop_18. Previous volume pgdata_shop_16 is kept until you Purge it.`

If the instance was running, keep the existing post-edit restart confirm (`y`/`n`). New config includes the new image tag and volume name; a fresh start initializes empty data on the new volume.

No automatic migration path is offered.

## Details panel and help

- Details panel shows `Version:` for Postgres instances.
- `Volume:` continues to show the effective volume name.
- README / help shortcuts: briefly note version selection and that volumes are auto-named.

## Purge

Unchanged: `d` / Action Menu purge runs `down -v` for the **current** compose project volume from the `.env`. Orphan volumes from prior majors are out of scope for auto-delete; the volume-change status message documents that.

## Compatibility / migration notes

| Case | Behavior |
|------|----------|
| Old `.env` without `POSTGRES_VERSION` | Read as `18`; compose must receive `POSTGRES_VERSION=18` on start (inject default when building env for compose if missing, and/or rewrite on next Save) |
| Old custom `POSTGRES_VOLUME` | Kept until next Edit Save; then replaced by derived name |
| Hardcoded `postgres:18` in compose today | Replaced by `${POSTGRES_VERSION}` |

Ensure Start/Up always passes a concrete `POSTGRES_VERSION` so compose never resolves an empty image tag.

## Testing (acceptance)

- Create Postgres: choose version 16 → `.env` has `POSTGRES_VERSION=16` and `POSTGRES_VOLUME=pgdata_<name>_16`; compose uses `postgres:16`.
- Create SQL Server: no version step; volume `sqlserver_<name>`.
- Edit Postgres: change 16→18 → volume preview/`POSTGRES_VOLUME` become `…_18`; status warns previous volume kept; restart confirm still works if running.
- Missing version on load → UI shows `18`.
- Volume step is not focusable; ←/→ on Version cycles curated majors only.
- Unit tests for derive helpers and wizard save content; compose templates contain `${POSTGRES_VERSION}`.

## Implementation sketch (for planning)

1. Compose + `.env.template` + `DatabaseInstance` / parser defaults.  
2. Derive helpers + remove editable volume step; add Version step.  
3. Save/write paths + edit volume-change status.  
4. Details panel + README.  
5. Tests and regression for create/edit wizards.
