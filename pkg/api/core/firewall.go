package core

import (
	"encoding/json"
	"fmt"
)

// Firewall settings (enable + active profile).
// SYNO.Core.Security.Firewall

type Firewall struct {
	EnableFirewall bool   `json:"enable_firewall" url:"enable_firewall"`
	ProfileName    string `json:"profile_name"    url:"profile_name,quoted"`
}

type FirewallGetResponse Firewall

type FirewallSetRequest struct {
	EnableFirewall bool   `url:"enable_firewall"`
	ProfileName    string `url:"profile_name,quoted"`
}

// Firewall conf (port check).
// SYNO.Core.Security.Firewall.Conf

type FirewallConf struct {
	EnablePortCheck bool `json:"enable_port_check" url:"enable_port_check"`
}

type FirewallConfGetResponse FirewallConf

type FirewallConfSetRequest struct {
	EnablePortCheck bool `url:"enable_port_check"`
}

// Firewall adapters.
// SYNO.Core.Security.Firewall.Adapter

type FirewallAdapterListResponse struct {
	AdapterNames []string `json:"adapter_names"`
}

// Firewall profile list.
// SYNO.Core.Security.Firewall.Profile

type FirewallProfileListResponse struct {
	ProfileNames []string `json:"profile_names"`
}

// FirewallRule is one rule inside an adapter block (Profile.get / Profile.set shape).
type FirewallRule struct {
	Enable        bool   `json:"enable"`
	Log           bool   `json:"log"`
	Name          string `json:"name"`
	Policy        string `json:"policy"`
	PortDirection string `json:"port_direction"`
	PortGroup     string `json:"port_group"`
	Ports         string `json:"ports"`
	Protocol      string `json:"protocol"`
	SourceIP      string `json:"source_ip"`
	SourceIPGroup string `json:"source_ip_group"`
	// Optional geoip / app-list fields when present on the wire.
	SetType string `json:"set_type,omitempty"`
	Src     string `json:"src,omitempty"`
}

// FirewallAdapterRules is the per-adapter policy + ordered rules list.
type FirewallAdapterRules struct {
	Policy string         `json:"policy"`
	Rules  []FirewallRule `json:"rules"`
}

// FirewallProfile is the full profile document.
// DSM stores adapters as top-level keys alongside "name" (e.g. "global", "eth0").
type FirewallProfile struct {
	Name     string                           `json:"name"`
	Adapters map[string]FirewallAdapterRules `json:"-"`
}

// UnmarshalJSON flattens DSM's top-level adapter keys into Adapters.
func (p *FirewallProfile) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if n, ok := raw["name"]; ok {
		if err := json.Unmarshal(n, &p.Name); err != nil {
			return fmt.Errorf("profile name: %w", err)
		}
		delete(raw, "name")
	}
	p.Adapters = make(map[string]FirewallAdapterRules, len(raw))
	for k, v := range raw {
		var block FirewallAdapterRules
		if err := json.Unmarshal(v, &block); err != nil {
			return fmt.Errorf("adapter %q: %w", k, err)
		}
		if block.Rules == nil {
			block.Rules = []FirewallRule{}
		}
		p.Adapters[k] = block
	}
	return nil
}

// MarshalJSON writes name + each adapter as top-level keys (DSM Profile.set shape).
func (p FirewallProfile) MarshalJSON() ([]byte, error) {
	raw := make(map[string]any, len(p.Adapters)+1)
	raw["name"] = p.Name
	for k, v := range p.Adapters {
		if v.Rules == nil {
			v.Rules = []FirewallRule{}
		}
		raw[k] = v
	}
	return json.Marshal(raw)
}

// FirewallProfileGetRequest fetches one profile by name.
type FirewallProfileGetRequest struct {
	Name string `url:"name,quoted"`
}

// FirewallProfileSetRequest saves a profile (not live until Apply).
type FirewallProfileSetRequest struct {
	Profile         FirewallProfile `url:"profile,json"`
	ProfileApplying bool            `url:"profile_applying"`
}

// FirewallProfileDeleteRequest removes a profile by name.
type FirewallProfileDeleteRequest struct {
	Name string `url:"name,quoted"`
}

// Profile.Apply two-phase commit.
// SYNO.Core.Security.Firewall.Profile.Apply

type FirewallProfileApplyStartRequest struct {
	Name            string `url:"name,quoted"`
	ProfileApplying bool   `url:"profile_applying"`
}

type FirewallProfileApplyStartResponse struct {
	TaskID string `json:"task_id"`
}

type FirewallProfileApplyStatusRequest struct {
	TaskID string `url:"task_id"`
}

type FirewallProfileApplyStatusResponse struct {
	Finish bool `json:"finish"`
	// Nested result when finish is true (shape varies; finish is the gate).
	Data json.RawMessage `json:"data,omitempty"`
}

// Rules load (read-only convenience; Profile.get is preferred for manage).
// SYNO.Core.Security.Firewall.Rules method=load

type FirewallRulesLoadRequest struct {
	Adapter string `url:"adapter,quoted"`
}

type FirewallRulesLoadResponse struct {
	Policy string `json:"policy"`
	Total  int    `json:"total"`
	// Wire schema differs from Profile rules; leave opaque for diagnostics.
	Rules json.RawMessage `json:"rules"`
}
