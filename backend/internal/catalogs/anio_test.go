package catalogs

import "testing"

// H140: a proceso's anio has the same upper bound (2099) as the payment
// configuration (admin: PUT /admin/ingresos/config/{anio}, 2000-2099), so no
// proceso can be created for a year whose price cannot be configured.
func TestCreateProcesoRequest_AnioRange(t *testing.T) {
	for anio, ok := range map[int]bool{2023: false, 2024: true, 2050: true, 2099: true, 2100: false, 2101: false} {
		req := CreateProcesoRequest{InstitucionID: 1, Nombre: "Proceso", Tipo: "ascenso", Anio: anio}
		if err := validate.Struct(req); (err == nil) != ok {
			t.Errorf("anio %d: valid=%v, want %v (err %v)", anio, err == nil, ok, err)
		}
	}
}
