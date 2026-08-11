package core

import (
	"encoding/json"
	"testing"
)

func TestFirewallProfileRoundTrip(t *testing.T) {
	raw := []byte(`{
		"name": "US only",
		"global": {
			"policy": "none",
			"rules": [
				{
					"enable": true,
					"log": false,
					"name": "",
					"policy": "allow",
					"port_direction": "destination",
					"port_group": "custom",
					"ports": "5001,443",
					"protocol": "tcp",
					"source_ip": "192.168.1.0/255.255.255.0",
					"source_ip_group": "netmask"
				}
			]
		}
	}`)

	var p FirewallProfile
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Name != "US only" {
		t.Fatalf("name: got %q", p.Name)
	}
	g, ok := p.Adapters["global"]
	if !ok {
		t.Fatal("missing global adapter")
	}
	if g.Policy != "none" || len(g.Rules) != 1 {
		t.Fatalf("global: %+v", g)
	}
	if g.Rules[0].Ports != "5001,443" {
		t.Fatalf("ports: %q", g.Rules[0].Ports)
	}

	out, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var again FirewallProfile
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if again.Name != p.Name || len(again.Adapters["global"].Rules) != 1 {
		t.Fatalf("round-trip mismatch: %+v", again)
	}
}
