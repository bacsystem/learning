"use client";

import { FormularioLimites } from "./_componentes/formulario-limites";
import { TarjetaGasto } from "./_componentes/tarjeta-gasto";
import { useTranslations } from "next-intl";

import { BotonReintentar } from "@/components/app/boton-reintentar";
import { Alerta } from "@/components/feedback/alerta";
import { Skeleton } from "@/components/feedback/skeleton";
import { useToast } from "@/components/feedback/toast";
import { useCarga } from "@/hooks/use-carga";
import { useAuth } from "@/lib/auth/auth-context";
import { CONTENEDOR_PAGINA } from "@/lib/estilos";
import { iaApi } from "@/lib/serums-ia/ia/api";

/** Panel de IA del admin (SIA-E6-02): gasto del mes y límites; el postulante no tiene pantalla en esta historia (D21). */
export default function IaPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();
  const gasto = useCarga(token ? "ia-gasto" : null, () => iaApi.gasto(token ?? ""));
  const configuracion = useCarga(token ? "ia-configuracion" : null, () => iaApi.configuracion(token ?? ""));

  return (
    <div className={CONTENEDOR_PAGINA}>
      <div>
        <h1 className="text-titulo-pagina text-foreground">{t("serumsIa.ia.titulo")}</h1>
        <p className="text-control text-muted-foreground">{t("serumsIa.ia.descripcion")}</p>
      </div>

      {gasto.error ? (
        <Alerta tono="error" accion={<BotonReintentar onReintentar={gasto.recargar} />}>
          {t("serumsIa.ia.gasto.errorCarga")}
        </Alerta>
      ) : gasto.datos ? (
        <TarjetaGasto gasto={gasto.datos} />
      ) : (
        <Skeleton className="h-96 w-full rounded-xl" />
      )}

      {configuracion.error ? (
        <Alerta tono="error" accion={<BotonReintentar onReintentar={configuracion.recargar} />}>
          {t("serumsIa.ia.configuracion.errorCarga")}
        </Alerta>
      ) : configuracion.datos && token ? (
        <FormularioLimites
          // Tras guardar se monta de nuevo con lo que devolvió el servidor, que es lo que manda.
          key={configuracion.datos.updated_at}
          configuracion={configuracion.datos}
          onGuardar={async (cambio) => {
            const guardada = await iaApi.guardarConfiguracion(token, cambio);
            configuracion.reemplazar(guardada);
            // El presupuesto pudo cambiar el estado de la alerta: se vuelve a leer el gasto.
            void gasto.refrescar();
            toast.ok(t("serumsIa.ia.configuracion.guardada"));
          }}
        />
      ) : (
        <Skeleton className="h-96 w-full rounded-xl" />
      )}
    </div>
  );
}
