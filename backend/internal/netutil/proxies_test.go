package netutil

import (
	"strings"
	"testing"
)

func TestParseTrustedProxies(t *testing.T) {
	cases := []struct {
		name, list string
		want       []string
		wantErr    bool
	}{
		{"empty", "", nil, false},
		{"empty entries skipped", " , ,", nil, false},
		{"IPv4 host gets /32", "10.1.1.1", []string{"10.1.1.1/32"}, false},
		{"IPv6 host gets /128", "2001:db8::1", []string{"2001:db8::1/128"}, false},
		{"CIDR with host bits keeps the network", "10.0.0.1/8", []string{"10.0.0.0/8"}, false},
		{"list with spaces", " 10.0.0.0/8 , 192.168.0.0/16 ", []string{"10.0.0.0/8", "192.168.0.0/16"}, false},
		{"invalid IP", "10.0.0.256", nil, true},
		{"invalid mask", "10.0.0.0/40", nil, true},
		{"hostname", "proxy", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			nets, err := ParseTrustedProxies(c.list)
			if c.wantErr {
				if err == nil {
					t.Fatalf("want error, got %v", nets)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTrustedProxies: %v", err)
			}
			got := make([]string, len(nets))
			for i, n := range nets {
				got[i] = n.String()
			}
			if strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}
