package config

import (
	"net"
	"time"
)

// Options are the options for generator
type Options struct {
	CaddyfilePath          string
	EnvFile                string
	DockerSockets          []string
	DockerCertsPath        []string
	DockerAPIsVersion      []string
	LabelPrefix            string
	ControlledServersLabel string
	ProxyServiceTasks      bool
	ProcessCaddyfile       bool
	ScanStoppedContainers  bool
	PollingInterval        time.Duration
	EventThrottleInterval  time.Duration
	Mode                   Mode
	Secret                 string
	ControllerNetwork      *net.IPNet
	IngressNetworks        []string
	HostStatusURL          string // URL path for host status page, e.g. /caddy/hosts. Leave empty to disable.
	HostStatusTemplate     string // Path to host status HTML template file
}

// Mode represents how this instance should run
type Mode int

const (
	// Controller runs only controller
	Controller Mode = 1
	// Server runs only server
	Server Mode = 2
	// Standalone runs controller and server in a single instance
	Standalone Mode = Controller | Server
)
