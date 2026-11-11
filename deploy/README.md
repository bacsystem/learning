# deploy/

Una carpeta por destino de despliegue. Todos usan **las mismas imágenes**
(`backend/Dockerfile` y `frontend/Dockerfile`), y la configuración entra
**solo por variables de entorno**. Nada específico de una nube vive en el
código ni en las imágenes (ADR 0004, Decisión 10).

| Carpeta | Qué es |
|---|---|
| [`local/`](local/README.md) | Preproducción local para qa: las imágenes de producción con `ENV=preprod`, Postgres 16 con TLS, Mailpit y datos sintéticos |
| [`railway/`](railway/README.md) | Producción en Railway: guía y plantilla de variables. No hay archivos de configuración: se configura en el panel (`docs/ops/railway.md`) |
| `verify-image.sh` | Verificación de contenido de las imágenes públicas (`make docker/verify`) |

Para llevar la app a otra nube se agrega `deploy/<destino>/` con su guía y sus
variables: las imágenes no cambian.

Reglas:

- **Ningún secreto real** en el repositorio. Los valores de
  `local/preprod.vars` son falsos (`NOT-FOR-PRODUCTION`) y solo protegen datos
  sintéticos.
- **Ningún dato real**: ni copias de producción ni nada de `assets/`.
- Las imágenes se despliegan **por tag inmutable** (`vX.Y.Z`) o por digest,
  nunca `:latest`. El frontend lleva un tag por entorno (`vX.Y.Z-<entorno>`),
  porque `NEXT_PUBLIC_API_URL` se incrusta al construir.
