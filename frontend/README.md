# Capacitate — frontend

Aplicación web de Capacitate: práctica y simulacros de examen de ascenso, con panel de administración.

## Stack

- **Next.js 16** (App Router, salida `standalone`) y **React 19**, en TypeScript.
- **[khipu design system](https://github.com/bacsystem/khipu-design-system)**: componentes, tokens y recetas de estilo copiados al proyecto.
- **[Base UI](https://base-ui.com)** (`@base-ui/react`) como primitivas accesibles de los componentes de khipu.
- **[Phosphor Icons](https://phosphoricons.com)** (`@phosphor-icons/react`; en Server Components, `@phosphor-icons/react/dist/ssr`).
- **Tailwind CSS v4** con los tokens de khipu (`src/styles/`), sin archivo `tailwind.config`.
- **next-intl** (es / en), **formik** + **yup** para formularios, **driver.js** para el recorrido guiado y **motion** para animaciones.

## Estructura

```
src/
  app/                  rutas (App Router)
    (dashboard)/        área privada: Shell de khipu (sidebar + barra superior)
    auth/               inicio de sesión, registro, recuperación y documentos legales
    page.tsx            landing pública
    globals.css         entrada de estilos (importa src/styles)
  components/
    ui, nav, datos, feedback, formularios, navegacion, patrones
                        componentes de khipu (no se editan a mano)
    app/                capa propia: sidebar, barra superior, notificaciones, tour,
                        TablaListado (el de khipu `datos/tabla-listado`, con textos traducidos)
  lib/
    estilos.ts, formato.ts, iconos.ts, …   recetas y utilidades de khipu
    <dominio>/api.ts                       clientes de la API del backend
  i18n/                 configuración de next-intl y mensajes (messages/es.json, en.json)
  styles/               tokens, tema y capa de compatibilidad shadcn
```

## Convenciones de interfaz

- Color solo por token (`bg-card`, `text-muted-foreground`, `border-border`, `bg-destructive-suave`…); nada de hex ni de la paleta de Tailwind.
- Tipografía con la escala con nombre: `text-etiqueta`, `text-control`, `text-cuerpo`, `text-titulo`, `text-titulo-pagina`, `text-metrica`.
- En el área privada la miga de la barra superior es el `h1`: las páginas no repiten su título.
- Listados: `TablaListado` (paginación en cliente) o `Table` + `PieTabla` (paginación en servidor).
- Confirmaciones de acción con `useToast()`; errores de página con `Alerta`.
- Clases combinadas siempre con `cn` de `@/lib/utils`.

## Desarrollo

Requiere Node 22 (ver `.nvmrc`).

```bash
npm install
npm run dev        # http://localhost:3000
npm run lint       # ESLint + Prettier
npm run build      # build de producción
```

Variables de entorno:

| Variable                     | Uso                                                                                                                              |
| ---------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `NEXT_PUBLIC_API_URL`        | URL base de la API del backend (en `.env.local`).                                                                                |
| `NEXT_PUBLIC_STORAGE_PREFIX` | Prefijo de la cookie de idioma.                                                                                                  |
| `NEXT_PUBLIC_CONTACT_EMAIL`  | Correo público de contacto (política de privacidad, términos y pie de la landing). Por defecto `contact@bacsystemsolutions.com`. |

Las variables `NEXT_PUBLIC_*` se incrustan en el bundle al construir.

## Docker

```bash
docker build --build-arg NEXT_PUBLIC_API_URL=https://api.ejemplo.com -t capacitate-frontend .
```

El `Dockerfile` hace un build multi-etapa y sirve la salida `standalone` de Next.js en el puerto 3000.

## Pendientes

- **Revisión legal:** los textos de `/auth/terms-and-conditions` y `/auth/privacy-policy` son una base redactada durante la migración a khipu (ley aplicable: Perú; privacidad según la Ley N.º 29733). Deben revisarse con asesoría legal antes de publicarse.
- **Retención de IP y user-agent:** se guardan por sesión y en la auditoría. Hay que definir cuánto tiempo se conservan e incluirlo en la política de privacidad, junto a la revisión legal.
