// Package backend defines types shared across backend implementations for testctr.
package backend

// ContainerInfo represents minimal container inspection data returned by a Backend.
// This struct is designed to be compatible with the information typically obtained
// from `docker inspect`.
type ContainerInfo struct {
	// NetworkSettings contains network-related information, primarily port mappings and IP addresses.
	NetworkSettings struct {
		// Ports is a map of exposed container ports (e.g., "80/tcp") to their host bindings.
		// Each container port can have multiple host bindings, though typically it's one.
		Ports map[string][]PortBinding `json:"Ports"`
		// InternalIP is the IP address of the container within its primary Docker network.
		// This might not always be available or relevant for all backends or network configurations.
		InternalIP string
	} `json:"NetworkSettings"`
	// State holds information about the container's current operational state.
	State struct {
		Running  bool   `json:"Running"`  // True if the container is currently running.
		Status   string `json:"Status"`   // Human-readable status (e.g., "running", "exited", "created").
		ExitCode int    `json:"ExitCode"` // Exit code of the container if it has stopped.
	} `json:"State"`
	// ID is the full unique identifier of the container.
	ID string `json:"Id"`
	// Name is the human-readable name of the container (often prefixed with a slash).
	Name string `json:"Name"`
	// Created is the timestamp when the container was created, in RFC3339Nano format.
	Created string `json:"Created"`
	// Config holds parts of the container's configuration, like Labels.
	Config struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

// PortBinding represents a single host port binding for a container port.
type PortBinding struct {
	HostIP   string `json:"HostIp"`   // Host IP address the port is bound to.
	HostPort string `json:"HostPort"` // Host port number.
}
