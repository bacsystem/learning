"use client";

import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { useTransition } from "react";

import { Tooltip } from "@/components/feedback/tooltip";
import { Button } from "@/components/ui/button";
import {
  Menu,
  MenuContent,
  MenuGroup,
  MenuGroupLabel,
  MenuRadioGroup,
  MenuRadioItem,
  MenuTrigger,
} from "@/components/ui/menu";
import type { LocaleOption } from "@/constants";
import { setClientLocale } from "@/i18n/locale";
import { TranslateIcon } from "@phosphor-icons/react";

const IDIOMAS: LocaleOption[] = ["es", "en"];

/** Selector de idioma (es/en) de la barra superior: guarda la cookie y refresca para que el servidor vuelva a traducir. */
export function BarraIdioma() {
  const t = useTranslations("dashboard");
  const locale = useLocale();
  const router = useRouter();
  const [pendiente, startTransition] = useTransition();

  const cambiar = (valor: LocaleOption) => {
    if (valor === locale) return;
    startTransition(async () => {
      await setClientLocale(valor);
      router.refresh();
    });
  };

  return (
    <Menu>
      <Tooltip texto={t("user-language")} lado="bottom">
        <MenuTrigger
          disabled={pendiente}
          render={
            <Button
              variant="ghost"
              size="default"
              aria-label={`${t("user-language")}: ${t(locale)}`}
              className="gap-1 px-2"
            />
          }
        >
          <TranslateIcon className="size-[18px]" />
          {/* tipografia-ok: el código de idioma es un literal en mayúsculas */}
          <span className="text-etiqueta text-muted-foreground font-semibold">{locale.toUpperCase()}</span>
        </MenuTrigger>
      </Tooltip>
      <MenuContent side="bottom" align="end" className="min-w-44">
        <MenuGroup>
          <MenuGroupLabel>{t("user-language")}</MenuGroupLabel>
          <MenuRadioGroup value={locale} onValueChange={(v) => cambiar(v as LocaleOption)}>
            {IDIOMAS.map((l) => (
              <MenuRadioItem key={l} value={l}>
                {t(l)}
              </MenuRadioItem>
            ))}
          </MenuRadioGroup>
        </MenuGroup>
      </MenuContent>
    </Menu>
  );
}
