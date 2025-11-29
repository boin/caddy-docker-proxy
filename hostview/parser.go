package hostview

import "encoding/json"

// HostInfo represents a host and its associated routes
type HostInfo struct {
	Host   string      `json:"host"`
	Routes []RouteInfo `json:"routes"`
}

// RouteInfo represents route information for a host
type RouteInfo struct {
	ServerName  string   `json:"serverName"`
	ListenAddrs []string `json:"listenAddrs"`
	Route       any      `json:"route"`
}

// ParseHostList parses Caddy JSON config and extracts host list
// This is a Go port of caddyConfigParser.js parseHostList function
func ParseHostList(configJSON []byte) []HostInfo {
	if len(configJSON) == 0 {
		return nil
	}

	var config map[string]any
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil
	}

	hostMap := make(map[string][]RouteInfo)

	apps, ok := config["apps"].(map[string]any)
	if !ok {
		return nil
	}

	httpApp, ok := apps["http"].(map[string]any)
	if !ok {
		return nil
	}

	servers, ok := httpApp["servers"].(map[string]any)
	if !ok {
		return nil
	}

	for serverName, serverAny := range servers {
		server, ok := serverAny.(map[string]any)
		if !ok {
			continue
		}

		// Extract listen addresses
		var listenAddrs []string
		if listen, ok := server["listen"].([]any); ok {
			for _, l := range listen {
				if addr, ok := l.(string); ok {
					listenAddrs = append(listenAddrs, addr)
				}
			}
		}

		// Extract routes
		routes, ok := server["routes"].([]any)
		if !ok {
			continue
		}

		for _, routeAny := range routes {
			route, ok := routeAny.(map[string]any)
			if !ok {
				continue
			}

			// Extract match rules
			matches, ok := route["match"].([]any)
			if !ok {
				continue
			}

			for _, matchAny := range matches {
				match, ok := matchAny.(map[string]any)
				if !ok {
					continue
				}

				// Extract hosts from match
				hosts, ok := match["host"].([]any)
				if !ok {
					continue
				}

				for _, hostAny := range hosts {
					host, ok := hostAny.(string)
					if !ok || host == "" {
						continue
					}

					hostMap[host] = append(hostMap[host], RouteInfo{
						ServerName:  serverName,
						ListenAddrs: listenAddrs,
						Route:       route,
					})
				}
			}
		}
	}

	// Convert map to slice
	result := make([]HostInfo, 0, len(hostMap))
	for host, routes := range hostMap {
		result = append(result, HostInfo{Host: host, Routes: routes})
	}

	return result
}
