# Checklist de release (GitFlow)

`release/*` (o `hotfix/*`) → PR a `main` → tag `vX.Y.Z` → despliegue.

El **merge a `main`, el tag y el despliegue los aprueba el usuario**. Devops y el
tech-lead preparan todo lo anterior. Nadie despliega, crea tags ni hace merge sin
esa aprobación explícita.

> El CI (`.github/workflows/*.yml`) corre **solo después** de que se crea el tag
> `v*`. Si el CI falla con el tag ya creado, el tag está publicado y hay que
> sacar un `vX.Y.(Z+1)`. Por eso la corrida completa se hace **en local, sobre la
> rama `release/*`, antes de pedir el tag**.

## 1. Preparar la rama (devops + tech-lead)

- [ ] `git switch -c release/X.Y.Z origin/develop`, con todos los PRs del
      release ya fusionados en `develop` y su `/pr-review` limpio.
- [ ] Changelog: lista de PRs desde el último tag. Si es el primer release, desde
      `1326a43`, la raíz pública de `main`:
      `git log --merges --oneline <último-tag>..HEAD`.
- [ ] Versión: el número de `frontend/package.json` coincide con `X.Y.Z`.
- [ ] **Orden de las migraciones** (sección 2).
- [ ] Inventario de variables al día
      ([variables-de-entorno.md](variables-de-entorno.md)). Si hay variables
      nuevas, ya están cargadas en el gestor de secretos del entorno y
      documentadas. Nunca se pegan en el PR ni en mensajes.
- [ ] `npm audit --omit=dev --audit-level=high` en `frontend/` y `govulncheck`
      en `backend/` sin hallazgos (`make test` corre govulncheck). Un hallazgo
      alto o crítico bloquea el release.

## 2. Orden de las migraciones

goose aplica en orden de versión y **rechaza** una versión menor que la última
aplicada. Comprobado con goose v3.21.1 sobre Postgres 16:

```
goose run: error: found 1 missing migrations before current version 61:
	version 58: migrations/058_...sql
```

Como `entrypoint.sh` migra con `set -e`, ese error deja al backend sin arrancar.

- [ ] Ninguna migración nueva tiene una versión menor que la mayor que ya hay en
      `main` o en la base de producción:
      `ls backend/migrations | tail` en `main` frente a la rama de release.
- [ ] Sin versiones duplicadas:
      `ls backend/migrations | cut -d_ -f1 | sort | uniq -d` no imprime nada.
      Dos PRs abiertos pueden haber elegido el mismo número. Con varios PRs que
      agregan migraciones, se renumeran **antes** de fusionarlos, en el orden en
      que se fusionan.
- [ ] Las migraciones nuevas se crean con `make migrate/create name=…`, que
      numera en secuencia (`062_…`). Un `goose create` sin `-s` usa una marca
      de tiempo (`20261111…_x.sql`): una vez aplicada, toda migración numerada
      posterior quedaría fuera de orden. Ningún archivo nuevo puede llevar una
      marca de tiempo.
- [ ] **No se rellenan los huecos existentes** (028–033 y 053). En una base que
      ya pasó esas versiones, una migración nueva con ese número sería
      rechazada por estar fuera de orden.
- [ ] Ciclo completo en una base **desechable** con Postgres 16:
      `make test TEST_DATABASE_URL=...`, que hace `up`, `down-to 0` y `up` otra
      vez, igual que el CI.
- [ ] Ensayo sobre una **copia restaurada** del backup de producción, cuando ya
      exista producción: `goose up` contra la copia, y medir el tiempo y los
      bloqueos de las migraciones con `CREATE INDEX` sobre tablas grandes.
- [ ] Revisar las migraciones con un tratamiento especial:
  - **039** (`drop_challenges`) y **051** (`normalize_notificaciones_destinatario`):
    su `Down` es un no-op a propósito. No se pueden revertir.
  - **054** y **058**: `-- +goose NO TRANSACTION` (`CREATE INDEX CONCURRENTLY`).
    Si fallan a medias, queda un índice `INVALID`: `DROP INDEX CONCURRENTLY
    <nombre>` y volver a arrancar. Está en `docs/puesta-en-marcha.md` §5.
  - **060**: `UNIQUE NULLS NOT DISTINCT`, que exige Postgres ≥ 15.
  - **013**: `CREATE EXTENSION pg_trgm`. El usuario de la base necesita permiso
      para crearla, o que ya esté creada.

Estado de `develop` en este momento: 001…061 (54 archivos, sin 028–033 ni 053).
La 062 está en el PR #92.

## 3. Verificación en local, sobre `release/X.Y.Z`

Con las mismas versiones que el CI (ver [paridad-ci-local.md](paridad-ci-local.md)):
Go según `backend/go.mod` (toolchain go1.27.4), Node 22.14.0 (`frontend/.nvmrc`)
y Postgres 16.

- [ ] `make lint`
- [ ] `make test TEST_DATABASE_URL=postgres://...` (base desechable)
- [ ] `make build`
- [ ] `docker build backend/` y
      `docker build --build-arg NEXT_PUBLIC_API_URL=<api de prod> frontend/`
- [ ] Los resultados se pegan en el PR `release/X.Y.Z → main`.

## 4. Merge y tag (lo aprueba el usuario)

- [ ] PR `release/X.Y.Z → main` con el changelog, el orden de las migraciones y
      las verificaciones del paso 3. El merge es «Create a merge commit», nunca
      squash.
- [ ] Con la aprobación del usuario: merge, tag anotado
      `git tag -a vX.Y.Z -m "vX.Y.Z"` sobre el merge en `main`, y
      `git push origin vX.Y.Z`.
- [ ] El CI del tag pasa (`backend` y `frontend`). Si falla, no se despliega.
- [ ] Merge de vuelta `main → develop`, también con merge commit.

## 5. Despliegue en Railway (lo aprueba el usuario)

La configuración de cada servicio está en [railway.md](railway.md). El autodeploy
está desactivado: el despliegue se lanza a mano cuando el CI del tag pasa.

- [ ] **Backup manual del volumen de Postgres** (pestaña Backups) justo antes
      de desplegar, porque el pre-deploy del backend migra. Con PITR activo,
      anotar además la hora exacta: es el punto de restauración.
- [ ] Variables nuevas del release ya cargadas en el servicio (selladas si
      son secretos) y con su copia en el gestor de secretos.
- [ ] Railway construye desde `main`, desde su `Dockerfile`. Las imágenes de
      Docker Hub (`make docker/release DOCKER_TAG=vX.Y.Z`) no se usan en
      Railway: solo si el usuario las pide.
- [ ] **Backend primero**: «Deploy» del commit del tag. El Pre-deploy Command
      corre `cmd/migrate`. Si falla, el despliegue se detiene y el anterior
      sigue sirviendo. Revisar los logs del pre-deploy.
- [ ] La migración usa conexión **directa** por la red privada, sin pooler: el
      advisory lock es de sesión.
- [ ] Luego el frontend, con `NEXT_PUBLIC_API_URL` = el dominio público del
      backend. Al terminar,
      `GET /health` responde 200 y los logs no muestran errores de
      `config load failed` ni de migración.
- [ ] Prueba rápida: login, listado de un dominio y envío de un correo de
      prueba (SMTP).

## 6. Cómo revertir

| Situación | Qué hacer |
|---|---|
| El release **no trae migraciones** | En Railway: **Rollback** al despliegue anterior. Restaura la imagen, la configuración y las variables, siempre que la imagen siga retenida: 72 h en Hobby, 120 h en Pro ([plans](https://docs.railway.com/pricing/plans)). Pasado ese plazo, redeploy del commit anterior |
| El release trae migraciones **aditivas** (tablas, columnas o índices nuevos) que el código anterior ignora | Volver a la imagen anterior **sin revertir la base**. Comprobado: con la base en la versión N y una imagen que solo llega a N−1, `goose up` responde `no migrations to run` y la imagen anterior arranca. Las migraciones se revierten después, si hace falta, con un hotfix |
| La migración **cambia o borra datos o columnas** que usa el código anterior | **Corregir hacia adelante** con un `hotfix/*`. Si eso no es viable, restaurar el backup del paso 5 o hacer PITR a la hora anotada (crea un servicio hermano: hay que apuntar `DATABASE_URL` a él). Se pierde lo escrito desde entonces, y eso lo decide el usuario. Ver [railway.md](railway.md) §5 |
| Revertir el esquema a mano | Solo como último recurso y con la aprobación del usuario: `cmd/migrate` con `down-to <versión>` contra producción, después de otro backup. El `down-to 0` del CI prueba que los `Down` **ejecutan**, no que **conserven datos**. 039 y 051 no se revierten |

Por eso cada migración se escribe de forma que la imagen anterior siga
funcionando con el esquema nuevo («expand/contract»): primero se agrega, y en un
release posterior se quita lo viejo.

## 7. Después

- [ ] Changelog publicado en la release de GitHub del tag.
- [ ] Issues del release cerradas y validadas.
- [ ] Dejar escrito en el PR o en `docs/ops/` cualquier ajuste manual que se
      haya hecho. Nada queda configurado a mano sin documentar.
