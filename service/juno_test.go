package service

import "testing"

func TestAdvertiseAddress(t *testing.T) {
	cases := map[string]string{
		":9180":             "192.168.0.10:9180",
		"0.0.0.0:9180":      "192.168.0.10:9180",
		"[::]:9180":         "192.168.0.10:9180",
		"10.0.0.5:9180":     "10.0.0.5:9180",
		"127.0.0.1:9180":    "127.0.0.1:9180",
		"juno.example:9180": "juno.example:9180",
		"invalid":           "invalid",
	}
	for listen, want := range cases {
		if got := advertiseAddress(listen, "192.168.0.10"); got != want {
			t.Errorf("advertiseAddress(%q) = %q, want %q", listen, got, want)
		}
	}
}
