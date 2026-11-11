import prettierConfig from "eslint-config-prettier";
import prettierPlugin from "eslint-plugin-prettier";
import reactPlugin from "eslint-plugin-react";
import reactHooksPlugin from "eslint-plugin-react-hooks";
import simpleImportSort from "eslint-plugin-simple-import-sort";
import globals from "globals";
import tseslint from "typescript-eslint";

import js from "@eslint/js";
import nextPlugin from "@next/eslint-plugin-next";

export default [
  // Global ignores — must be a standalone object with only `ignores` to apply globally
  {
    ignores: [
      ".next/**",
      "coverage/**",
      "node_modules/**",
      "public/**",
      "next-env.d.ts",
      "next-eslint-plugin-next.d.ts",
      "package-lock.json",
    ],
  },
  // Base JS & environment settings
  {
    files: ["**/*.{js,jsx,ts,tsx}"],
    languageOptions: {
      parser: tseslint.parser,
      parserOptions: {
        ecmaVersion: "latest",
        sourceType: "module",
        ecmaFeatures: { jsx: true },
        project: "./tsconfig.json",
      },
      globals: globals.browser,
    },
    linterOptions: {
      reportUnusedDisableDirectives: true,
    },
    settings: {
      react: { version: "detect" },
      next: { rootDir: ["./src"] },
    },
    ignores: [
      ".cache",
      ".next",
      ".wrangler",
      "bun.lock",
      "next-env.d.ts",
      "next-eslint-plugin-next.d.ts",
      "next.config.ts",
      "node_modules",
      "package-lock.json",
      "public",
      "yarn.lock",
    ],
  },
  // Recommended rules
  js.configs.recommended,
  ...tseslint.configs.recommended,
  prettierConfig,
  // Plugins and custom rules
  {
    plugins: {
      prettier: prettierPlugin,
      "simple-import-sort": simpleImportSort,
      "@next/next": nextPlugin,
      react: reactPlugin,
      "react-hooks": reactHooksPlugin,
    },
    rules: {
      "prefer-const": "warn",
      "no-var": "error",
      "no-unused-vars": "off",
      "@typescript-eslint/no-unused-vars": [
        "error",
        {
          caughtErrors: "none",
        },
      ],
      "@typescript-eslint/no-explicit-any": "off",
      "object-shorthand": "warn",
      "quote-props": ["warn", "as-needed"],
      "@typescript-eslint/array-type": [
        "warn",
        {
          default: "array",
        },
      ],
      "@typescript-eslint/consistent-type-assertions": [
        "warn",
        {
          assertionStyle: "as",
          objectLiteralTypeAssertions: "allow",
        },
      ],
      "react/jsx-fragments": ["warn", "syntax"],
      "react/jsx-filename-extension": [
        "warn",
        {
          extensions: ["ts", "tsx"],
        },
      ],
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "warn",
      "react/react-in-jsx-scope": "off",
      "react/prop-types": "off",
      "no-html-link-for-pages": "off",
      "simple-import-sort/imports": [
        "warn",
        {
          groups: [["^\\u0000"], ["^"], ["^@/*"]],
        },
      ],
      "prettier/prettier": "warn",
      "@next/next/google-font-display": "error",
    },
  },
  // Las pantallas usan los envoltorios de components/app, que pasan los textos traducidos a khipu (PR #82).
  {
    files: ["src/app/**/*.{ts,tsx}"],
    rules: {
      "no-restricted-imports": [
        "warn",
        {
          paths: [
            ["@/components/ui/dialog", "DialogContent", "@/components/app/dialogo"],
            ["@/components/ui/sheet", "SheetContent", "@/components/app/dialogo"],
            ["@/components/patrones/pie-tabla", "PieTabla", "@/components/app/pie-tabla"],
            ["@/components/datos/tabla-datos", "TablaDatos", "@/components/app/tabla-datos"],
            ["@/components/datos/tabla-listado", "TablaListado", "@/components/app/tabla-listado"],
            ["@/components/formularios/campo", "Campo", "@/components/app/campo"],
          ].map(([name, importName, app]) => ({
            name,
            importNames: [importName],
            message: `Importa ${importName} desde "${app}": pasa los textos traducidos.`,
          })),
        },
      ],
    },
  },
];
