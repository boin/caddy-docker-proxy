package hostview

import (
	"encoding/json"
	"testing"
)

func TestParseHostList(t *testing.T) {
	// Sample Caddy config JSON
	configJSON := []byte(`{
		"apps": {
			"http": {
				"servers": {
					"srv0": {
						"listen": [":80", ":443"],
						"routes": [
							{
								"match": [{"host": ["example.com"]}],
								"handle": [{"handler": "reverse_proxy", "upstreams": [{"dial": "localhost:8080"}]}]
							},
							{
								"match": [{"host": ["api.example.com"]}],
								"handle": [{"handler": "reverse_proxy", "upstreams": [{"dial": "localhost:3000"}]}]
							}
						]
					},
					"srv1": {
						"listen": [":8080"],
						"routes": [
							{
								"match": [{"host": ["internal.example.com"]}],
								"handle": [{"handler": "file_server"}]
							}
						]
					}
				}
			}
		}
	}`)

	hosts := ParseHostList(configJSON)

	if len(hosts) != 3 {
		t.Errorf("Expected 3 hosts, got %d", len(hosts))
	}

	// Check that all expected hosts are present
	hostMap := make(map[string]HostInfo)
	for _, h := range hosts {
		hostMap[h.Host] = h
	}

	if _, ok := hostMap["example.com"]; !ok {
		t.Error("Expected host 'example.com' not found")
	}

	if _, ok := hostMap["api.example.com"]; !ok {
		t.Error("Expected host 'api.example.com' not found")
	}

	if _, ok := hostMap["internal.example.com"]; !ok {
		t.Error("Expected host 'internal.example.com' not found")
	}

	// Check routes for example.com
	exampleHost := hostMap["example.com"]
	if len(exampleHost.Routes) != 1 {
		t.Errorf("Expected 1 route for example.com, got %d", len(exampleHost.Routes))
	}

	if exampleHost.Routes[0].ServerName != "srv0" {
		t.Errorf("Expected server name 'srv0', got '%s'", exampleHost.Routes[0].ServerName)
	}

	if len(exampleHost.Routes[0].ListenAddrs) != 2 {
		t.Errorf("Expected 2 listen addresses, got %d", len(exampleHost.Routes[0].ListenAddrs))
	}
}

func TestParseHostListEmpty(t *testing.T) {
	hosts := ParseHostList(nil)
	if hosts != nil {
		t.Error("Expected nil for nil input")
	}

	hosts = ParseHostList([]byte{})
	if hosts != nil {
		t.Error("Expected nil for empty input")
	}

	hosts = ParseHostList([]byte("{}"))
	if hosts != nil {
		t.Error("Expected nil for empty config")
	}
}

func TestParseHostListInvalidJSON(t *testing.T) {
	hosts := ParseHostList([]byte("invalid json"))
	if hosts != nil {
		t.Error("Expected nil for invalid JSON")
	}
}

func TestHostInfoJSON(t *testing.T) {
	host := HostInfo{
		Host: "test.com",
		Routes: []RouteInfo{
			{
				ServerName:  "srv0",
				ListenAddrs: []string{":80"},
				Route:       map[string]any{"handler": "test"},
			},
		},
	}

	data, err := json.Marshal(host)
	if err != nil {
		t.Fatalf("Failed to marshal HostInfo: %v", err)
	}

	var decoded HostInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal HostInfo: %v", err)
	}

	if decoded.Host != "test.com" {
		t.Errorf("Expected host 'test.com', got '%s'", decoded.Host)
	}

	if len(decoded.Routes) != 1 {
		t.Errorf("Expected 1 route, got %d", len(decoded.Routes))
	}
}
