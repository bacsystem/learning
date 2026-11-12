package questions

import (
	"reflect"
	"testing"
)

func ids(v ...int64) *[]int64 { return &v }

func TestValidarPasajeIDs(t *testing.T) {
	veinte := make([]int64, 20)
	veintiuno := make([]int64, 21)
	for i := range veintiuno {
		veintiuno[i] = int64(100 - i)
		if i < 20 {
			veinte[i] = int64(100 - i)
		}
	}
	tests := []struct {
		name string
		req  ReemplazarCitasRequest
		want string
		ord  []int64
	}{
		{"ausente", ReemplazarCitasRequest{}, CodigoValidacion, nil},
		{"vacía quita todas", ReemplazarCitasRequest{PasajeIDs: ids()}, "", []int64{}},
		{"id cero", ReemplazarCitasRequest{PasajeIDs: ids(1, 0)}, CodigoValidacion, nil},
		{"id negativo", ReemplazarCitasRequest{PasajeIDs: ids(-3)}, CodigoValidacion, nil},
		{"repetidos", ReemplazarCitasRequest{PasajeIDs: ids(4, 4)}, CodigoValidacion, nil},
		{"ordena", ReemplazarCitasRequest{PasajeIDs: ids(9, 3, 5)}, "", []int64{3, 5, 9}},
		{"20 valen", ReemplazarCitasRequest{PasajeIDs: &veinte}, "", nil},
		{"21 dan limite_citas", ReemplazarCitasRequest{PasajeIDs: &veintiuno}, CodigoLimiteCitas, nil},
		{"repetidos antes que el límite", ReemplazarCitasRequest{PasajeIDs: ptrSlice(append(veintiuno[:20:20], veintiuno[0]))}, CodigoValidacion, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validarPasajeIDs(tt.req)
			if c := codigoRegla(t, err); c != tt.want {
				t.Fatalf("código = %q, want %q", c, tt.want)
			}
			if tt.ord != nil && !reflect.DeepEqual(got, tt.ord) {
				t.Errorf("ids = %v, want %v", got, tt.ord)
			}
		})
	}
}

func ptrSlice(s []int64) *[]int64 { return &s }

// Steps 6–8 of E3-07 §4.2.
func TestDecidirCitas(t *testing.T) {
	activa := pasajeInfo{FuenteID: 1, FuenteActiva: true}
	retirada := pasajeInfo{FuenteID: 2}
	info := map[int64]pasajeInfo{10: activa, 11: activa, 20: retirada, 21: retirada}
	cita := func(pasaje int64) citaFila {
		return citaFila{PasajeID: pasaje, FuenteID: info[pasaje].FuenteID, FuenteActiva: info[pasaje].FuenteActiva}
	}

	tests := []struct {
		name     string
		estado   string
		actuales []citaFila
		pedidos  []int64
		want     string
		agregar  []int64
		quitar   []int64
	}{
		{"pasaje inexistente", EstadoBorrador, nil, []int64{10, 99}, CodigoReferenciaInvalida, nil, nil},
		{"cita nueva a fuente retirada", EstadoBorrador, nil, []int64{20}, CodigoFuenteNoActiva, nil, nil},
		{"conservar una retirada y sumar una activa (revalidar tras retiro)", EstadoEnRevision, []citaFila{cita(20)}, []int64{20, 10}, "", []int64{10}, nil},
		{"conservar una retirada sin cambios", EstadoEnRevision, []citaFila{cita(20)}, []int64{20}, "", nil, nil},
		{"lista idéntica", EstadoAprobada, []citaFila{cita(10), cita(11)}, []int64{11, 10}, "", nil, nil},
		{"agregar a una aprobada", EstadoAprobada, []citaFila{cita(10)}, []int64{10, 11}, "", []int64{11}, nil},
		{"quitar la última cita de una aprobada", EstadoAprobada, []citaFila{cita(10)}, []int64{}, CodigoUltimaCitaActiva, nil, nil},
		{"quitar una de dos en una aprobada", EstadoAprobada, []citaFila{cita(10), cita(11)}, []int64{11}, "", nil, []int64{10}},
		{"sustituir A por B activa en una aprobada", EstadoAprobada, []citaFila{cita(10)}, []int64{11}, "", []int64{11}, []int64{10}},
		{"quitar la activa dejando solo una retirada", EstadoAprobada, []citaFila{cita(10), cita(20)}, []int64{20}, CodigoUltimaCitaActiva, nil, nil},
		{"quitar todas en borrador", EstadoBorrador, []citaFila{cita(10)}, []int64{}, "", nil, []int64{10}},
		{"quitar todas en activo heredado", EstadoActivo, []citaFila{cita(10)}, []int64{}, "", nil, []int64{10}},
		{"quitar todas en rechazada", EstadoRechazada, []citaFila{cita(10)}, []int64{}, "", nil, []int64{10}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := decidirCitas(preguntaBloqueada{Estado: tt.estado, Citas: tt.actuales}, tt.pedidos, info)
			if c := codigoRegla(t, err); c != tt.want {
				t.Fatalf("código = %q, want %q (%v)", c, tt.want, err)
			}
			if tt.want != "" {
				return
			}
			if !reflect.DeepEqual(plan.Agregar, tt.agregar) || !reflect.DeepEqual(plan.Quitar, tt.quitar) {
				t.Errorf("agregar/quitar = %v/%v, want %v/%v", plan.Agregar, plan.Quitar, tt.agregar, tt.quitar)
			}
			if sin := len(tt.agregar)+len(tt.quitar) == 0; plan.sinCambios() != sin {
				t.Errorf("sinCambios = %v, want %v", plan.sinCambios(), sin)
			}
		})
	}
}

func TestDecidirCitas_Detalle(t *testing.T) {
	info := map[int64]pasajeInfo{10: {FuenteID: 1}, 11: {FuenteID: 1}}
	_, err := decidirCitas(preguntaBloqueada{Estado: EstadoBorrador}, []int64{10, 11, 99, 98}, info)
	re, _ := err.(*ReglaError)
	if re == nil || !reflect.DeepEqual(re.Detalle["pasaje_ids"], []int64{99, 98}) {
		t.Errorf("faltantes: %+v", err)
	}
	_, err = decidirCitas(preguntaBloqueada{Estado: EstadoBorrador}, []int64{10, 11}, info)
	re, _ = err.(*ReglaError)
	if re == nil || re.Codigo != CodigoFuenteNoActiva || !reflect.DeepEqual(re.Detalle["pasaje_ids"], []int64{10, 11}) {
		t.Errorf("no activas: %+v", err)
	}
}
