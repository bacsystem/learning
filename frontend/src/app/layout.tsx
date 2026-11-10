import "./globals.css";

import type { Metadata } from "next";
import { DM_Sans, Figtree, Inter, JetBrains_Mono, Manrope, Plus_Jakarta_Sans } from "next/font/google";
import { NextIntlClientProvider } from "next-intl";
import { getLocale, getMessages } from "next-intl/server";
import { Suspense } from "react";

import Loading from "@/app/loading";
import { Providers } from "@/components/providers";
import { ScriptDiseno } from "@/components/script-diseno";
import { AuthProvider } from "@/lib/auth/auth-context";

// Inter para la interfaz y JetBrains Mono para identificadores (khipu). Las demás fuentes son preferencia del usuario
// (components/apariencia-opciones.ts) y no se precargan. Las variables van en <html> para que las lean los portales.
const sans = Inter({ variable: "--font-sans", subsets: ["latin"] });
const jakarta = Plus_Jakarta_Sans({ variable: "--fuente-jakarta", subsets: ["latin"], preload: false });
const manrope = Manrope({ variable: "--fuente-manrope", subsets: ["latin"], preload: false });
const dmSans = DM_Sans({ variable: "--fuente-dmsans", subsets: ["latin"], preload: false });
const figtree = Figtree({ variable: "--fuente-figtree", subsets: ["latin"], preload: false });
const mono = JetBrains_Mono({ variable: "--font-mono", weight: ["400", "500", "600"], subsets: ["latin"] });
const fuentes = [sans, jakarta, manrope, dmSans, figtree, mono].map((f) => f.variable).join(" ");

export const metadata: Metadata = {
  title: "Capacitate",
  description: "Capacitate",
};

export default async function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  const locale = await getLocale();
  const messages = await getMessages();

  return (
    <html lang={locale} className={fuentes} suppressHydrationWarning>
      <head>
        <ScriptDiseno />
      </head>
      <body className="antialiased">
        <NextIntlClientProvider messages={messages}>
          <Providers>
            <AuthProvider>
              <Suspense fallback={<Loading />}>{children}</Suspense>
            </AuthProvider>
          </Providers>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
