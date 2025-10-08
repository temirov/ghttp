// Package serverdetails provides utilities for describing server configuration
// details in a user-friendly format.
package serverdetails

import (
	"fmt"
	"net"
	"strings"
)

const (
	bindAddressEmptyValue            = ""
	ipv4AddressAnyValue              = "0.0.0.0"
	ipv4AddressLoopbackValue         = "127.0.0.1"
	loggingDisplayHostLocalhostValue = "localhost"
)

// ServingAddressFormatter normalizes listening addresses for presentation in
// logs so that users can click the reported URL directly.
type ServingAddressFormatter struct{}

// NewServingAddressFormatter constructs a ServingAddressFormatter.
func NewServingAddressFormatter() ServingAddressFormatter {
	return ServingAddressFormatter{}
}

// FormatHostAndPortForLogging returns the host and port combination to display
// in logs. Any empty, wildcard, or loopback bind addresses are mapped to the
// more user-friendly "localhost" value.
func (formatter ServingAddressFormatter) FormatHostAndPortForLogging(bindAddress string, port string) string {
	sanitizedHost := strings.TrimSpace(bindAddress)
	switch sanitizedHost {
	case bindAddressEmptyValue, ipv4AddressAnyValue, ipv4AddressLoopbackValue:
		sanitizedHost = loggingDisplayHostLocalhostValue
	}
	return net.JoinHostPort(sanitizedHost, port)
}

// FormatURLForLogging returns a full URL with scheme for logging output.
func (formatter ServingAddressFormatter) FormatURLForLogging(scheme string, bindAddress string, port string) string {
	normalizedScheme := strings.TrimSuffix(strings.TrimSpace(scheme), "://")
	return fmt.Sprintf("%s://%s", normalizedScheme, formatter.FormatHostAndPortForLogging(bindAddress, port))
}
