package apispec

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fuenteConRutas wraps registration statements in a compilable-looking file.
func fuenteConRutas(consts, cuerpo string) string {
	return "package x\n" + consts + "\nfunc Mount() {\n" + cuerpo + "\n}\n"
}

func TestEscanearFuente_FormasConocidas(t *testing.T) {
	tests := []struct {
		name   string
		consts string
		cuerpo string
		want   map[string]rol
	}{
		{"handler func público", "", `mux.HandleFunc("GET /a", h.A)`, map[string]rol{"GET /a": rolPublico}},
		{"HandlerFunc sin middleware", "", `mux.Handle("GET /a", http.HandlerFunc(h.A))`, map[string]rol{"GET /a": rolPublico}},
		{"autenticado", "", `mux.Handle("GET /a", authn(http.HandlerFunc(h.A)))`, map[string]rol{"GET /a": rolAutenticado}},
		{"admin con authn", "", `mux.Handle("POST /a", authn(admin(http.HandlerFunc(h.A))))`, map[string]rol{"POST /a": rolAdmin}},
		{"admin con nombres largos", "", `mux.Handle("PUT /a/{id}", requireAuth(requireAdmin(http.HandlerFunc(h.A))))`, map[string]rol{"PUT /a/{id}": rolAdmin}},
		{"admin con paquete", "", `mux.Handle("GET /a", authn(auth.RequireAdmin(http.HandlerFunc(h.A))))`, map[string]rol{"GET /a": rolAdmin}},
		{"auth como función", "", `mux.Handle("DELETE /a", auth(http.HandlerFunc(h.A)))`, map[string]rol{"DELETE /a": rolAutenticado}},
		{"rate limit público", "", `mux.Handle("POST /a", authRL.Limit(http.HandlerFunc(h.A)))`, map[string]rol{"POST /a": rolPublico}},
		{"rate limit y auth", "", `mux.Handle("PATCH /a", rl.Limit(auth(http.HandlerFunc(h.A))))`, map[string]rol{"PATCH /a": rolAutenticado}},
		{"fábrica de handler", "", `mux.HandleFunc("GET /health", healthHandler(database, version))`, map[string]rol{"GET /health": rolPublico}},
		{"admin sin auth sigue siendo admin", "", `mux.Handle("GET /a", admin(http.HandlerFunc(h.A)))`, map[string]rol{"GET /a": rolAdmin}},
		{"constante", `const RutaA = "GET /a/{id}"`, `mux.Handle(RutaA, authn(admin(http.HandlerFunc(h.A))))`, map[string]rol{"GET /a/{id}": rolAdmin}},
		{"constantes en bloque", "const (\n\tRutaA = \"GET /a\"\n\tRutaB = \"POST /b\"\n)", "mux.Handle(RutaA, authn(http.HandlerFunc(h.A)))\nmux.Handle(RutaB, authn(http.HandlerFunc(h.B)))",
			map[string]rol{"GET /a": rolAutenticado, "POST /b": rolAutenticado}},
		{"espacios extra en el patrón", "", `mux.Handle("GET   /a", h.A)`, map[string]rol{"GET /a": rolPublico}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rutas, err := escanearFuente("x/routes.go", fuenteConRutas(tt.consts, tt.cuerpo))
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]rol{}
			for _, r := range rutas {
				got[r.Clave] = r.Rol
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// A registration form the scanner cannot read must fail, never be skipped.
func TestEscanearFuente_FormaDesconocidaFalla(t *testing.T) {
	tests := []struct {
		name    string
		consts  string
		cuerpo  string
		wantErr string
	}{
		{"middleware desconocido", "", `mux.Handle("GET /a", cors(http.HandlerFunc(h.A)))`, `llamada a "cors" desconocida`},
		{"middleware desconocido anidado", "", `mux.Handle("GET /a", authn(audit(http.HandlerFunc(h.A))))`, `llamada a "audit" desconocida`},
		{"handler literal", "", `mux.HandleFunc("GET /a", func(w http.ResponseWriter, r *http.Request) {})`, "forma de handler desconocida"},
		{"middleware construido en línea", "", `mux.Handle("GET /a", auth.RequireAuth(s, c)(http.HandlerFunc(h.A)))`, "función desconocida"},
		{"patrón calculado", "", `mux.Handle(prefijo+"/a", h.A)`, "forma de patrón desconocida"},
		{"patrón de un campo", "", `mux.Handle(r.Patron, h.A)`, "forma de patrón desconocida"},
		{"constante de otro archivo", "", `mux.Handle(otro.RutaA, h.A)`, "forma de patrón desconocida"},
		{"constante sin resolver", "", `mux.Handle(RutaFalta, h.A)`, "no es una constante de texto"},
		{"patrón sin método", "", `mux.Handle("/a", h.A)`, "se esperaba"},
		{"método inválido", "", `mux.Handle("FETCH /a", h.A)`, "se esperaba"},
		{"patrón con host", "", `mux.Handle("GET example.com/a", h.A)`, "se esperaba"},
		{"un solo argumento", "", `mux.Handle("GET /a")`, "1 argumentos"},
		{"tabla recorrida en bucle", "", "for _, r := range tabla {\n\tmux.Handle(r.Patron, r.H)\n}", "forma de patrón desconocida"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := escanearFuente("x/routes.go", fuenteConRutas(tt.consts, tt.cuerpo))
			if err == nil {
				t.Fatal("se esperaba un error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q no contiene %q", err, tt.wantErr)
			}
			if !strings.Contains(err.Error(), "x/routes.go:") {
				t.Errorf("el error no lleva archivo:línea: %q", err)
			}
		})
	}
}

func TestEscanearFuente_SinRegistrosNoEsError(t *testing.T) {
	rutas, err := escanearFuente("x.go", "package x\nfunc F() { println(1) }\n")
	if err != nil || len(rutas) != 0 {
		t.Fatalf("got %v, %v", rutas, err)
	}
}

func rutasDe(claves ...string) []rutaCodigo {
	var out []rutaCodigo
	for _, c := range claves {
		out = append(out, rutaCodigo{Clave: c, Rol: rolAdmin, Pos: "x/routes.go:1"})
	}
	return out
}

func opAdmin(clave string) opSpec { return opSpec{Clave: clave, Rol: "admin", Tags: []string{"x"}} }

func TestComparar(t *testing.T) {
	codigo := map[string][]rutaCodigo{"a": rutasDe("GET /a"), "b": rutasDe("GET /b")}
	tests := []struct {
		name         string
		spec         infoSpec
		documentados []string
		wantErr      string // substring of the only expected error; "" for none
		wantAviso    bool
	}{
		{"todo coincide", infoSpec{Ops: map[string]opSpec{"GET /a": opAdmin("GET /a"), "GET /b": opAdmin("GET /b")}}, []string{"a", "b"}, "", false},
		{"falta una ruta en paquete documentado", infoSpec{Ops: map[string]opSpec{"GET /a": opAdmin("GET /a")}}, []string{"a", "b"}, "GET /b", false},
		{"falta una ruta en paquete pendiente solo avisa", infoSpec{Ops: map[string]opSpec{"GET /a": opAdmin("GET /a")}}, []string{"a"}, "", true},
		{"operación del spec que no existe", infoSpec{Ops: map[string]opSpec{"GET /a": opAdmin("GET /a"), "GET /b": opAdmin("GET /b"), "GET /c": opAdmin("GET /c")}}, []string{"a", "b"}, "GET /c está en el spec pero no en el código", false},
		{"planificada ausente del código se permite", infoSpec{Ops: map[string]opSpec{
			"GET /a": opAdmin("GET /a"), "GET /b": opAdmin("GET /b"),
			"GET /c": {Clave: "GET /c", Rol: "admin", Tags: []string{"x"}, Estado: "planificado"}}}, []string{"a", "b"}, "", false},
		{"planificada que ya existe", infoSpec{Ops: map[string]opSpec{"GET /a": {Clave: "GET /a", Rol: "admin", Tags: []string{"x"}, Estado: "planificado"}, "GET /b": opAdmin("GET /b")}}, []string{"a", "b"}, "quite x-estado", false},
		{"x-estado desconocido", infoSpec{Ops: map[string]opSpec{"GET /a": {Clave: "GET /a", Rol: "admin", Tags: []string{"x"}, Estado: "borrador"}, "GET /b": opAdmin("GET /b")}}, []string{"a", "b"}, "no permitido", false},
		{"rol distinto", infoSpec{Ops: map[string]opSpec{"GET /a": {Clave: "GET /a", Rol: "autenticado", Tags: []string{"x"}}, "GET /b": opAdmin("GET /b")}}, []string{"a", "b"}, `"autenticado" en el spec y "admin" en el código`, false},
		{"sin x-rol", infoSpec{Ops: map[string]opSpec{"GET /a": {Clave: "GET /a", Tags: []string{"x"}}, "GET /b": opAdmin("GET /b")}}, []string{"a", "b"}, "x-rol", false},
		{"sin tag", infoSpec{Ops: map[string]opSpec{"GET /a": {Clave: "GET /a", Rol: "admin"}, "GET /b": opAdmin("GET /b")}}, []string{"a", "b"}, "exactamente un tag", false},
		{"dos tags", infoSpec{Ops: map[string]opSpec{"GET /a": {Clave: "GET /a", Rol: "admin", Tags: []string{"x", "y"}}, "GET /b": opAdmin("GET /b")}}, []string{"a", "b"}, "exactamente un tag", false},
		{"clave huérfana", infoSpec{Ops: map[string]opSpec{"GET /a": opAdmin("GET /a"), "GET /b": opAdmin("GET /b")}, Huerfanas: []string{"a.yaml: /z"}}, []string{"a", "b"}, "a.yaml: /z no está referenciada", false},
		{"paquete documentado inexistente", infoSpec{Ops: map[string]opSpec{"GET /a": opAdmin("GET /a"), "GET /b": opAdmin("GET /b")}}, []string{"a", "b", "zzz"}, `"zzz" no es un paquete`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errores, avisos := comparar(codigo, tt.spec, tt.documentados)
			if tt.wantErr == "" && len(errores) != 0 {
				t.Fatalf("errores inesperados: %v", errores)
			}
			if tt.wantErr != "" && (len(errores) != 1 || !strings.Contains(errores[0], tt.wantErr)) {
				t.Fatalf("errores %v; se esperaba uno con %q", errores, tt.wantErr)
			}
			if (len(avisos) > 0) != tt.wantAviso {
				t.Fatalf("avisos %v, wantAviso %v", avisos, tt.wantAviso)
			}
		})
	}
}

func TestFichasDePuntero(t *testing.T) {
	got, err := fichasDePuntero("/paths/~1a~1{id}~0x")
	if err != nil || !reflect.DeepEqual(got, []string{"paths", "/a/{id}~x"}) {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err := fichasDePuntero("paths/x"); err == nil {
		t.Fatal("un puntero sin «/» inicial debe fallar")
	}
}

// escribir creates files under dir.
func escribir(t *testing.T, dir string, archivos map[string]string) {
	t.Helper()
	for nombre, contenido := range archivos {
		ruta := filepath.Join(dir, nombre)
		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, []byte(contenido), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLeerSpec_SigueRefYLeeOperaciones(t *testing.T) {
	dir := t.TempDir()
	escribir(t, dir, map[string]string{
		"openapi.yaml": `
paths:
  /a/{id}:
    $ref: './paquetes/p.yaml#/paths/~1a~1{id}'
  /b:
    $ref: './paquetes/p.yaml#/paths/~1b'
`,
		"paquetes/p.yaml": `
paths:
  /a/{id}:
    get:
      tags: [p]
      x-rol: admin
    delete:
      tags: [p]
      x-rol: autenticado
      x-estado: planificado
  /b:
    post:
      tags: [p, q]
      x-rol: publico
`,
	})
	info, err := leerSpec(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]opSpec{
		"GET /a/{id}":    {Clave: "GET /a/{id}", Rol: "admin", Tags: []string{"p"}},
		"DELETE /a/{id}": {Clave: "DELETE /a/{id}", Rol: "autenticado", Estado: "planificado", Tags: []string{"p"}},
		"POST /b":        {Clave: "POST /b", Rol: "publico", Tags: []string{"p", "q"}},
	}
	if !reflect.DeepEqual(info.Ops, want) {
		t.Fatalf("got %+v\nwant %+v", info.Ops, want)
	}
	if len(info.Huerfanas) != 0 {
		t.Fatalf("huérfanas inesperadas: %v", info.Huerfanas)
	}
}

func TestLeerSpec_DetectaHuerfanasYRefsRotas(t *testing.T) {
	t.Run("clave de paquete sin referencia", func(t *testing.T) {
		dir := t.TempDir()
		escribir(t, dir, map[string]string{
			"openapi.yaml":    "paths:\n  /a:\n    $ref: './paquetes/p.yaml#/paths/~1a'\n",
			"paquetes/p.yaml": "paths:\n  /a:\n    get: {tags: [p], x-rol: admin}\n  /sobra:\n    get: {tags: [p], x-rol: admin}\n",
		})
		info, err := leerSpec(dir)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(info.Huerfanas, []string{"p.yaml: /sobra"}) {
			t.Fatalf("huérfanas: %v", info.Huerfanas)
		}
	})
	t.Run("ref a un puntero inexistente", func(t *testing.T) {
		dir := t.TempDir()
		escribir(t, dir, map[string]string{
			"openapi.yaml":    "paths:\n  /a:\n    $ref: './paquetes/p.yaml#/paths/~1nada'\n",
			"paquetes/p.yaml": "paths: {}\n",
		})
		if _, err := leerSpec(dir); err == nil {
			t.Fatal("una $ref rota debe fallar")
		}
	})
	t.Run("ref a un archivo inexistente", func(t *testing.T) {
		dir := t.TempDir()
		escribir(t, dir, map[string]string{"openapi.yaml": "paths:\n  /a:\n    $ref: './paquetes/nada.yaml#/paths/~1a'\n"})
		if _, err := leerSpec(dir); err == nil {
			t.Fatal("una $ref a un archivo inexistente debe fallar")
		}
	})
}

// escanearRepo fails on a routes.go it cannot read, on a file with no routes
// and on a registration outside the known files.
func TestEscanearRepo(t *testing.T) {
	build := func(t *testing.T, archivos map[string]string) (map[string][]rutaCodigo, error) {
		t.Helper()
		dir := t.TempDir()
		escribir(t, dir, archivos)
		return escanearRepo(dir)
	}
	main := "package main\nfunc run() { mux.HandleFunc(\"GET /health\", healthHandler(d, v)) }\n"
	ok := fuenteConRutas("", `mux.Handle("GET /a", authn(http.HandlerFunc(h.A)))`)

	t.Run("lee main y routes", func(t *testing.T) {
		got, err := build(t, map[string]string{"cmd/server/main.go": main, "internal/a/routes.go": ok})
		if err != nil || len(got["server"]) != 1 || len(got["a"]) != 1 || got["a"][0].Rol != rolAutenticado {
			t.Fatalf("got %v, %v", got, err)
		}
	})
	t.Run("routes.go sin rutas", func(t *testing.T) {
		_, err := build(t, map[string]string{"cmd/server/main.go": main, "internal/a/routes.go": "package a\n"})
		if err == nil || !strings.Contains(err.Error(), "no registra ninguna ruta") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("registro en otro archivo", func(t *testing.T) {
		_, err := build(t, map[string]string{
			"cmd/server/main.go": main, "internal/a/routes.go": ok,
			"internal/a/extra.go": fuenteConRutas("", `mux.Handle("GET /z", h.Z)`),
		})
		if err == nil || !strings.Contains(err.Error(), "fuera de routes.go") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("ruta duplicada", func(t *testing.T) {
		_, err := build(t, map[string]string{
			"cmd/server/main.go": "package main\nfunc run() { mux.HandleFunc(\"GET /a\", h.A) }\n", "internal/a/routes.go": ok,
		})
		if err == nil || !strings.Contains(err.Error(), "ya registrada") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("forma desconocida en routes.go", func(t *testing.T) {
		_, err := build(t, map[string]string{
			"cmd/server/main.go":   main,
			"internal/a/routes.go": fuenteConRutas("", `mux.Handle("GET /a", cors(h.A))`),
		})
		if err == nil || !strings.Contains(err.Error(), "internal/a/routes.go:") {
			t.Fatalf("got %v", err)
		}
	})
}
