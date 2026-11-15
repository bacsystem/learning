package locale

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ES},
		{"spaces only", "   ", ES},
		{"es", "es", ES},
		{"en", "en", EN},
		{"upper case and spaces", " EN ", EN},
		{"mixed case es", "Es", ES},
		{"regional variant", "en-US", ES},
		{"underscore variant", "EN_us", ES},
		{"unsupported", "fr", ES},
		{"long garbage", string(make([]byte, 4096)), ES},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Normalize(tt.in); got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
