package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/cbaciliod/learning/internal/fuentes"
)

const (
	// tituloFuenteDemo names the only fuente the seed creates. It is not an
	// official one: oficial_verificada stays false.
	tituloFuenteDemo  = "Fuente demo (no oficial)"
	nombreArchivoDemo = "fuente-demo.md"
)

// pasajesDemo is synthetic text, written for the demo: it never comes from
// assets/ (those files hold personal data). The questions cite these passages.
var pasajesDemo = []fuentes.PasajeNuevo{
	{Orden: 1, LineaInicio: 1, LineaFin: 3, Seccion: textoPtr("1. Alcance"),
		Texto: "Este documento sintético existe solo para la demostración. Describe un procedimiento ficticio de práctica clínica.", Palabras: 16},
	{Orden: 2, LineaInicio: 5, LineaFin: 8, Seccion: textoPtr("2. Definiciones"),
		Texto: "Para la demostración, la opción correcta de cada pregunta es la primera alternativa. No tiene valor clínico.", Palabras: 17},
	{Orden: 3, LineaInicio: 10, LineaFin: 12, Seccion: textoPtr("3. Disposiciones finales"),
		Texto: "Los datos de la demostración son inventados y pueden borrarse en cualquier momento.", Palabras: 12},
}

func textoPtr(s string) *string { return &s }

// contenidoFuenteDemo is the file of the demo fuente, built from its passages.
func contenidoFuenteDemo() []byte {
	var b []byte
	for _, p := range pasajesDemo {
		b = append(b, []byte("# "+*p.Seccion+"\n\n"+p.Texto+"\n\n")...)
	}
	return b
}

// fuenteDemo returns the passages of the demo fuente (by orden), creating and
// publishing it when missing and finishing a publication left half done.
func (s *sembrador) fuenteDemo(ctx context.Context) ([]int64, error) {
	var id int64
	var estado string
	err := s.db.QueryRowContext(ctx, `SELECT id, estado FROM fuentes WHERE titulo = $1`, tituloFuenteDemo).Scan(&id, &estado)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		id, estado = 0, ""
	case err != nil:
		return nil, fmt.Errorf("buscar la fuente demo: %w", err)
	}

	src := fuentes.NewPgSource(s.db)
	if id == 0 {
		if id, _, err = src.GuardarFuente(ctx, fuentes.NuevaFuente{Titulo: tituloFuenteDemo, Tipo: "guia",
			NombreArchivo: nombreArchivoDemo, Contenido: contenidoFuenteDemo()}, nil); err != nil {
			return nil, fmt.Errorf("crear la fuente demo: %w", err)
		}
		estado = fuentes.EstadoProcesando
		s.creado["fuentes"]++
	}
	if estado == fuentes.EstadoProcesando {
		// A running server's worker can publish the same version meanwhile: then
		// ours is refused, and the fuente being activa is all the seed needs.
		if err := src.PublicarVersion(ctx, id, 1, pasajesDemo, nil); err != nil {
			if estado, rerr := s.estadoFuente(ctx, id); rerr != nil || estado != fuentes.EstadoActiva {
				return nil, fmt.Errorf("publicar la fuente demo: %w", err)
			}
		}
	} else if estado != fuentes.EstadoActiva {
		return nil, fmt.Errorf("la fuente demo está %s; bórrala o restáurala para sembrar de nuevo", estado)
	}

	return s.pasajesFuente(ctx, id)
}

func (s *sembrador) estadoFuente(ctx context.Context, id int64) (string, error) {
	var estado string
	err := s.db.QueryRowContext(ctx, `SELECT estado FROM fuentes WHERE id = $1`, id).Scan(&estado)
	return estado, err
}

// pasajesFuente returns the passage ids of the fuente, by version and orden.
func (s *sembrador) pasajesFuente(ctx context.Context, id int64) ([]int64, error) {
	var pasajes []int64
	if err := s.db.SelectContext(ctx, &pasajes, `SELECT pa.id FROM pasajes pa
		JOIN fuente_versiones fv ON fv.id = pa.fuente_version_id
		WHERE fv.fuente_id = $1 ORDER BY fv.numero, pa.orden`, id); err != nil {
		return nil, fmt.Errorf("pasajes de la fuente demo: %w", err)
	}
	if len(pasajes) == 0 {
		return nil, errors.New("la fuente demo no tiene pasajes")
	}
	return pasajes, nil
}
