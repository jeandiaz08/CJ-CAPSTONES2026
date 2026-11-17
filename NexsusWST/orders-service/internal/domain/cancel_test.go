package domain

import "testing"

// H-09-T07 (SCRUM-237): el cliente solo cancela antes de la preparación.
func TestClientCanCancel(t *testing.T) {
	cases := []struct {
		o    Order
		want bool
	}{
		{Order{Status: StatusPending}, true},
		{Order{Status: StatusValidating}, true},
		{Order{Status: StatusValidated}, true},
		{Order{Status: StatusVoided}, true},
		{Order{Status: StatusIncident, PrevStatus: StatusPending}, true},
		{Order{Status: StatusValidated, WaveID: "OLA-AM"}, false}, // ya asignada a ola en Invas
		{Order{Status: StatusPicking, WaveID: "OLA-AM"}, false},
		{Order{Status: StatusPicked, WaveID: "OLA-AM"}, false},
		{Order{Status: StatusPacking, WaveID: "OLA-AM"}, false},
		{Order{Status: StatusReadyPickup, WaveID: "OLA-AM"}, false},
		{Order{Status: StatusIncident, PrevStatus: StatusPicking}, false},
		{Order{Status: StatusIncident, WaveID: "OLA-AM"}, false},
	}
	for _, c := range cases {
		if got := c.o.ClientCanCancel(); got != c.want {
			t.Errorf("%s (previo %s, ola %q): %v, quería %v", c.o.Status, c.o.PrevStatus, c.o.WaveID, got, c.want)
		}
	}
}
