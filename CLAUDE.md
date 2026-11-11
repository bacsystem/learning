# CLAUDE.md

Guía para Claude Code (claude.ai/code) al trabajar en este repositorio.

## Qué es

Capacitate: plataforma de práctica y simulacros para los exámenes de ascenso, con panel de administración.

## Estructura del monorepo

- `frontend/` — Next.js 16 (App Router) + React 19 + TypeScript, construido con el **khipu design system** (ver `frontend/README.md`).
- `backend/` — API en Go 1.27 (`internal/<dominio>`, comandos en `cmd/`: `server`, `migrate`, `seed`, …) sobre PostgreSQL; migraciones en `backend/migrations/`.
- `docker-compose.yml` — base de datos, backend y frontend juntos.
- `docs/` — guía de puesta en marcha (`docs/puesta-en-marcha.md`).
- `assets/` — datos y documentos fuente (bancos de preguntas, padrones). **Nunca se versionan** (están en `.gitignore`): contienen datos personales.

## Comandos

Desde la raíz: `make dev`, `make test`, `make lint`, `make build`.

Frontend (`cd frontend`):

```bash
npm run dev        # servidor de desarrollo (localhost:3000)
npm run build      # build de producción
npm run lint       # ESLint
npm run lint:fix   # ESLint con autocorrección
npx tsc --noEmit   # chequeo de tipos
```

El frontend no tiene runner de tests todavía; la verificación es `tsc`, `lint` y `build`.

## Frontend: arquitectura

- **Rutas:** `src/app/(dashboard)/` es el área privada (guard de sesión y de rol admin en su `layout.tsx`, Shell de khipu con `SidebarApp` y `BarraSuperior`); `src/app/auth/` son las páginas públicas de acceso; `src/app/page.tsx` es la landing.
- **Componentes de khipu:** `src/components/{ui,nav,datos,feedback,formularios,navegacion,patrones}` y las recetas de `src/lib/{estilos,formato,iconos,…}.ts` se copian del repo khipu con su CLI. No se editan a mano.
- **Capa propia de la app:** `src/components/app/` (sidebar, barra superior, notificaciones, recursos, idioma, tour, «Acerca de», y `TablaListado`: el de khipu con sus textos traducidos). La navegación está en `src/components/app/navegacion.ts`.
- **Estilos:** Tailwind v4 con los tokens de khipu en `src/styles/{tokens,theme,shadcn}.css`. Colores y tipografía siempre desde tokens o recetas, nunca valores fijos. Las clases se unen con `cn` de `@/lib/utils`.
- **Iconos:** Phosphor (`@phosphor-icons/react`; en Server Components, `@phosphor-icons/react/dist/ssr`).
- **i18n:** next-intl con el locale en la cookie `${NEXT_PUBLIC_STORAGE_PREFIX}-locale` (por defecto `capacitate-locale`). Idiomas `es` y `en`; mensajes en `src/i18n/messages/{es,en}.json` bajo el namespace `dashboard`. Toda clave debe existir en ambos archivos.
- **Datos:** clientes de la API en `src/lib/<dominio>/api.ts` (`NEXT_PUBLIC_API_URL`); sesión en `src/lib/auth/auth-context.tsx`.
- **Formularios:** formik + yup. **Tour guiado:** driver.js en `src/components/app/tour-app.tsx` (los pasos apuntan a ids `tour-*` y `data-tour`).

## Reglas

- **Design system obligatorio:** [khipu-design-system](https://github.com/bacsystem/khipu-design-system) es la referencia de system design. Todo elemento o componente de UI que se implemente usa sus componentes, tokens y recetas (copiados con su CLI); se consulta el repo khipu antes de implementar. No se crean componentes propios que dupliquen uno de khipu ni estilos fuera de sus tokens.
- **Componentes del design system:** si hace falta un componente que khipu no tiene, o cambiar uno de los suyos, primero se abre un PR en [khipu-design-system](https://github.com/bacsystem/khipu-design-system), se revisa con `/pr-review` corrigiendo todos los hallazgos, se avisa al usuario de los PRs abiertos, y recién después se usa en este proyecto.
- GitFlow: `feature/*` y `fix/*` salen de `develop` y vuelven por PR a `develop`; a `main` solo llegan `release/*` y `hotfix/*`, con tag `vX.Y.Z`. Commits Conventional Commits en inglés.
- GitHub Actions solo corre con tags `v*` (producción). Antes de abrir un PR a `develop` se corre en local todo lo que corre el CI (ver `.github/workflows/`) y se reportan los resultados en el PR.
- **Equipo de agentes:** `setup-team.sh` es la fuente de verdad de `.claude/agents/` y `.claude/settings.json`. Todo cambio a esos archivos actualiza el script en el mismo PR y se verifica con `bash setup-team.sh --check`.
- Nunca subir `assets/`, binarios de `backend/` ni archivos `.env`.

## Equipo de agentes
Roles (definidos en .claude/agents/): po, architect, tech-lead (sesión principal), backend-dev, frontend-dev, qa.
Flujo: po escribe historias en docs/stories/ -> architect revisa su impacto, define el contrato en docs/api/ y los ADR en docs/adr/ -> tech-lead reparte tareas ->
backend-dev (backend/) y frontend-dev (frontend/) trabajan en paralelo, cada uno dueño de sus archivos ->
qa valida contra criterios de aceptación -> cada developer crea su PR a develop -> tech-lead lo revisa con el skill /pr-review ->
si hay hallazgos, los corrige el developer autor y el tech-lead vuelve a revisar, hasta que quede limpio.
Ninguna tarea se cierra sin ese review limpio. El merge requiere aprobación del usuario.
Comunicación siempre activa: cada integrante avisa al empezar, al terminar, al bloquearse y cuando su cambio afecta a otro.
Respetar las convenciones de este archivo (GitFlow, Conventional Commits, i18n, tokens de khipu). Sin push ni deploy sin aprobación del usuario.
