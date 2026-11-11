"use client";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useState } from "react";

import { Campo } from "@/components/app/campo";
import { DialogContent } from "@/components/app/dialogo";
import { type ColumnaListado, TablaListado } from "@/components/app/tabla-listado";
import { Alerta } from "@/components/feedback/alerta";
import { useToast } from "@/components/feedback/toast";
import { Entrada } from "@/components/formularios/campo";
import { SelectCampo } from "@/components/formularios/select-campo";
import { MenuAcciones } from "@/components/navegacion/menu-acciones";
import { BotonAsync } from "@/components/patrones/boton-async";
import { Aviso, PieDialogo } from "@/components/patrones/cabecera-dialogo";
import { Chip, PillEstado } from "@/components/patrones/pill-estado";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Dialog, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { GrupoBotones } from "@/components/ui/grupo-botones";
import { adminPersonasApi, PersonaListItem, UpdatePersonaRequest } from "@/lib/auth/auth-api";
import { useAuth } from "@/lib/auth/auth-context";
import { catalogsApi, Especialidad, Grado } from "@/lib/catalogs/api";
import { ACCION_PRINCIPAL, BOTON_PRIMARIO, BOTON_SECUNDARIO, CONTENEDOR_PAGINA } from "@/lib/estilos";
import { PencilSimpleIcon, UploadSimpleIcon, UserCircleIcon, WarningCircleIcon, XIcon } from "@phosphor-icons/react";

type TabEstado = "todos" | "activo" | "inactivo";

const ESTADO_TABS: TabEstado[] = ["todos", "activo", "inactivo"];

const EMPTY_FORM: UpdatePersonaRequest = {
  nombres: "",
  apellido_paterno: "",
  apellido_materno: "",
  email: "",
  telefono: "",
  grado_id: 0,
  especialidad_id: 0,
  institucion_id: 1,
  estado: "activo",
};

export default function PersonasPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();
  /** Etiqueta traducida de un estado; un valor desconocido se muestra tal cual. */
  const etiquetaEstado = (estado: string) =>
    ESTADO_TABS.includes(estado as TabEstado) ? t(`admin-personas-estado-${estado}`) : estado;
  const opcionesEstado = [
    { value: "activo", label: etiquetaEstado("activo") },
    { value: "inactivo", label: etiquetaEstado("inactivo") },
  ];

  const [tab, setTab] = useState<TabEstado>("todos");
  const [allPersonas, setAllPersonas] = useState<PersonaListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Edit dialog
  const [editing, setEditing] = useState<PersonaListItem | null>(null);
  const [form, setForm] = useState<UpdatePersonaRequest>(EMPTY_FORM);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  // Catalogs
  const [grados, setGrados] = useState<Grado[]>([]);
  const [especialidades, setEspecialidades] = useState<Especialidad[]>([]);

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    setError(null);
    try {
      setAllPersonas(await adminPersonasApi.list(token));
    } catch {
      setError(t("admin-personas-error-cargar"));
    } finally {
      setLoading(false);
    }
  }, [token, t]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (!token) return;
    catalogsApi
      .listGrados(token)
      .then(setGrados)
      .catch(() => {});
    catalogsApi
      .listEspecialidades(token)
      .then(setEspecialidades)
      .catch(() => {});
  }, [token]);

  const rows = useMemo(
    () => (tab === "todos" ? allPersonas : allPersonas.filter((p) => p.estado === tab)),
    [allPersonas, tab],
  );

  const openEdit = (p: PersonaListItem) => {
    setEditing(p);
    setForm({
      nombres: p.nombres,
      apellido_paterno: p.apellido_paterno,
      apellido_materno: p.apellido_materno,
      email: p.email,
      telefono: p.telefono,
      grado_id: p.grado_id,
      especialidad_id: p.especialidad_id,
      institucion_id: p.institucion_id,
      estado: p.estado,
    });
    setSaveError(null);
  };

  const closeEdit = () => {
    setEditing(null);
    setSaveError(null);
  };

  const handleSave = async () => {
    if (!token || !editing) return;
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await adminPersonasApi.update(token, editing.id, form);
      setAllPersonas((prev) => prev.map((p) => (p.id === updated.id ? updated : p)));
      toast.ok(t("admin-personas-actualizada"), updated.nombre_completo);
      closeEdit();
    } catch (e: unknown) {
      setSaveError((e as { message?: string }).message || t("comun-error-guardar"));
    } finally {
      setSaving(false);
    }
  };

  const filteredEspecialidades = useMemo(
    () =>
      form.institucion_id ? especialidades.filter((e) => e.institucion_id === form.institucion_id) : especialidades,
    [especialidades, form.institucion_id],
  );

  const columns = useMemo<ColumnaListado<PersonaListItem>[]>(
    () => [
      {
        id: "nombre_completo",
        titulo: t("admin-usuarios-col-nombre"),
        ordenable: true,
        ocultable: false,
        render: (p) => (
          <div className="flex min-w-48 items-center gap-2.5">
            <Avatar nombre={p.nombre_completo} tono="accent" />
            <span className="text-foreground truncate font-medium">{p.nombre_completo}</span>
          </div>
        ),
      },
      { id: "dni", titulo: t("admin-usuarios-dni"), ordenable: true, className: "font-mono tabular-nums" },
      { id: "grado", titulo: t("perfil-grado"), ordenable: true },
      { id: "especialidad", titulo: t("perfil-especialidad"), ordenable: true, ocultaInicial: true },
      { id: "email", titulo: t("admin-usuarios-col-email"), ordenable: true, ocultaInicial: true },
      { id: "telefono", titulo: t("admin-usuarios-telefono"), ocultaInicial: true, className: "tabular-nums" },
      {
        id: "tiene_usuario",
        titulo: t("admin-personas-cuenta"),
        ordenable: true,
        valor: (p) => (p.tiene_usuario ? 1 : 0),
        texto: (p) => (p.tiene_usuario ? t("admin-personas-si") : t("admin-personas-no")),
        render: (p) =>
          p.tiene_usuario ? (
            <PillEstado tono="ok">{t("admin-personas-si")}</PillEstado>
          ) : (
            <PillEstado tono="neutro">{t("admin-personas-no")}</PillEstado>
          ),
      },
      {
        id: "estado",
        titulo: t("perfil-estado"),
        ordenable: true,
        texto: (p) => etiquetaEstado(p.estado),
        render: (p) => (
          <PillEstado tono={p.estado === "activo" ? "ok" : "neutro"}>{etiquetaEstado(p.estado)}</PillEstado>
        ),
      },
      {
        id: "actions",
        titulo: "",
        alinear: "derecha",
        exportable: false,
        ocultable: false,
        ancho: "3.5rem",
        render: (p) => (
          <MenuAcciones
            etiqueta={t("admin-usuarios-acciones-para", { nombre: p.nombre_completo })}
            acciones={[{ label: t("admin-editar-persona"), icon: PencilSimpleIcon, onClick: () => openEdit(p) }]}
          />
        ),
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t],
  );

  const contar = (v: TabEstado) =>
    v === "todos" ? allPersonas.length : allPersonas.filter((p) => p.estado === v).length;

  return (
    <div className={CONTENEDOR_PAGINA}>
      <p className="text-cuerpo text-muted-foreground">
        {t.rich("admin-personas-resumen", {
          n: allPersonas.length,
          b: (c) => <span className="text-foreground font-medium tabular-nums">{c}</span>,
        })}
      </p>

      {error ? (
        <Alerta
          tono="error"
          accion={
            <Button variant="ghost" size="icon-sm" aria-label={t("comun-cerrar")} onClick={() => setError(null)}>
              <XIcon weight="bold" className="size-4" />
            </Button>
          }
        >
          {error}
        </Alerta>
      ) : null}

      <TablaListado
        titulo={t("menu-admin-personas")}
        columnas={columns}
        filas={rows}
        clave={(p) => String(p.id)}
        cargando={loading}
        unidad={t("admin-personas-unidad")}
        placeholderBusqueda={t("admin-personas-buscar")}
        vacio={tab === "todos" ? t("admin-personas-vacio") : t("admin-personas-vacio-estado")}
        acciones={
          <Link href="/admin/personas/import" className={ACCION_PRINCIPAL}>
            <UploadSimpleIcon className="size-4" />
            {t("admin-preguntas-importar-csv")}
          </Link>
        }
        filtros={
          <GrupoBotones
            etiqueta={t("admin-usuarios-filtrar-estado")}
            valor={tab}
            onCambio={setTab}
            opciones={ESTADO_TABS.map((v) => ({
              valor: v,
              etiqueta: (
                <>
                  {etiquetaEstado(v)}
                  <span className="bg-secondary text-etiqueta text-muted-foreground rounded-full px-1.5 font-semibold tabular-nums">
                    {contar(v)}
                  </span>
                </>
              ),
            }))}
          />
        }
      />

      {/* ── Edit Dialog ── */}
      <Dialog
        open={!!editing}
        onOpenChange={(open) => {
          if (!open && !saving) closeEdit();
        }}
      >
        <DialogContent className="gap-0 p-0 sm:max-w-xl">
          {editing ? (
            <div className="px-5 pt-5 pr-14 pb-3">
              <div className="flex items-center gap-3">
                <Avatar nombre={editing.nombre_completo} tamano="lg" tono="accent" />
                <div className="min-w-0">
                  <DialogTitle className="truncate">{editing.nombre_completo}</DialogTitle>
                  <DialogDescription>
                    {t("admin-usuarios-dni")} <span className="font-mono">{editing.dni}</span> · {editing.institucion}
                  </DialogDescription>
                </div>
              </div>
              <div className="mt-3 flex flex-wrap gap-1.5">
                <Chip>{editing.grado}</Chip>
                <Chip>{editing.especialidad}</Chip>
                {editing.tiene_usuario ? (
                  <PillEstado tono="ok">
                    <UserCircleIcon aria-hidden className="size-3.5" />
                    {t("admin-personas-tiene-cuenta")}
                  </PillEstado>
                ) : null}
              </div>
            </div>
          ) : null}

          <form
            id="form-persona"
            className="grid grid-cols-1 gap-4 px-5 pt-2 pb-2 sm:grid-cols-2"
            onSubmit={(e) => {
              e.preventDefault();
              if (!saving) handleSave();
            }}
          >
            {saveError ? (
              <div className="sm:col-span-2">
                <Aviso tono="peligro" icon={WarningCircleIcon}>
                  {saveError}
                </Aviso>
              </div>
            ) : null}

            <Campo id="persona-nombres" etiqueta={t("perfil-nombres")} className="sm:col-span-2">
              <Entrada
                id="persona-nombres"
                required
                value={form.nombres}
                onChange={(e) => setForm((f) => ({ ...f, nombres: e.target.value }))}
              />
            </Campo>
            <Campo id="persona-ap-paterno" etiqueta={t("perfil-apellido-paterno")}>
              <Entrada
                id="persona-ap-paterno"
                required
                value={form.apellido_paterno}
                onChange={(e) => setForm((f) => ({ ...f, apellido_paterno: e.target.value }))}
              />
            </Campo>
            <Campo id="persona-ap-materno" etiqueta={t("perfil-apellido-materno")} opcional>
              <Entrada
                id="persona-ap-materno"
                value={form.apellido_materno}
                onChange={(e) => setForm((f) => ({ ...f, apellido_materno: e.target.value }))}
              />
            </Campo>
            <Campo id="persona-email" etiqueta={t("admin-usuarios-col-email")}>
              <Entrada
                id="persona-email"
                type="email"
                required
                value={form.email}
                onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))}
              />
            </Campo>
            <Campo id="persona-telefono" etiqueta={t("admin-usuarios-telefono")} opcional>
              <Entrada
                id="persona-telefono"
                type="tel"
                className="tabular-nums"
                value={form.telefono}
                onChange={(e) => setForm((f) => ({ ...f, telefono: e.target.value }))}
              />
            </Campo>
            <Campo id="persona-grado" etiqueta={t("perfil-grado")}>
              <SelectCampo
                id="persona-grado"
                required
                placeholder={t("admin-personas-elegir-grado")}
                opciones={grados.map((g) => ({ value: String(g.id), label: g.nombre }))}
                valor={form.grado_id ? String(form.grado_id) : ""}
                onCambio={(v) => setForm((f) => ({ ...f, grado_id: Number(v) }))}
              />
            </Campo>
            <Campo id="persona-especialidad" etiqueta={t("perfil-especialidad")}>
              <SelectCampo
                id="persona-especialidad"
                required
                placeholder={t("admin-personas-elegir-especialidad")}
                opciones={filteredEspecialidades.map((e) => ({ value: String(e.id), label: e.nombre }))}
                valor={form.especialidad_id ? String(form.especialidad_id) : ""}
                onCambio={(v) => setForm((f) => ({ ...f, especialidad_id: Number(v) }))}
              />
            </Campo>
            <Campo id="persona-estado" etiqueta={t("perfil-estado")}>
              <SelectCampo
                id="persona-estado"
                required
                opciones={opcionesEstado}
                valor={form.estado}
                onCambio={(v) => setForm((f) => ({ ...f, estado: v }))}
              />
            </Campo>
          </form>

          <PieDialogo>
            <button type="button" onClick={closeEdit} disabled={saving} className={BOTON_SECUNDARIO}>
              {t("comun-cancelar")}
            </button>
            <BotonAsync
              pendiente={saving}
              textoPendiente={t("comun-guardando")}
              type="submit"
              form="form-persona"
              className={BOTON_PRIMARIO}
            >
              {t("comun-guardar-cambios")}
            </BotonAsync>
          </PieDialogo>
        </DialogContent>
      </Dialog>
    </div>
  );
}
