package serverdetails_test

import (
	"testing"

	"github.com/temirov/ghhtp/internal/serverdetails"
)

const (
	testNameEmptyBindAddress              = "empty bind address becomes localhost"
	testNameWildcardBindAddress           = "wildcard bind address becomes localhost"
	testNameLoopbackBindAddress           = "loopback bind address becomes localhost"
	testNameExternalBindAddressPreserved  = "external bind address is preserved"
	testNameHostnameWithWhitespaceTrimmed = "hostname with whitespace is trimmed"
	bindAddressEmptyValue                 = ""
	bindAddressWildcardValue              = "0.0.0.0"
	bindAddressLoopbackValue              = "127.0.0.1"
	bindAddressExternalValue              = "192.168.10.50"
	bindAddressHostnameWithWhitespace     = "  example.com  "
	bindAddressIpvSixValue                = "2001:db8::1"
	portValue                             = "8000"
	expectedLocalhostDisplay              = "localhost:8000"
	expectedExternalDisplay               = "192.168.10.50:8000"
	expectedHostnameDisplay               = "example.com:8000"
)

func TestServingAddressFormatter_FormatHostAndPortForLogging(t *testing.T) {
	formatter := serverdetails.NewServingAddressFormatter()

	testCases := []struct {
		name        string
		bindAddress string
		expected    string
	}{
		{
			name:        testNameEmptyBindAddress,
			bindAddress: bindAddressEmptyValue,
			expected:    expectedLocalhostDisplay,
		},
		{
			name:        testNameWildcardBindAddress,
			bindAddress: bindAddressWildcardValue,
			expected:    expectedLocalhostDisplay,
		},
		{
			name:        testNameLoopbackBindAddress,
			bindAddress: bindAddressLoopbackValue,
			expected:    expectedLocalhostDisplay,
		},
		{
			name:        testNameExternalBindAddressPreserved,
			bindAddress: bindAddressExternalValue,
			expected:    expectedExternalDisplay,
		},
		{
			name:        testNameHostnameWithWhitespaceTrimmed,
			bindAddress: bindAddressHostnameWithWhitespace,
			expected:    expectedHostnameDisplay,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			actual := formatter.FormatHostAndPortForLogging(testCase.bindAddress, portValue)
			if actual != testCase.expected {
				t.Fatalf("formatted address mismatch: expected %s, got %s", testCase.expected, actual)
			}
		})
	}
}

func TestServingAddressFormatter_FormatHostAndPortForLoggingUsesNetJoinHostPort(t *testing.T) {
	formatter := serverdetails.NewServingAddressFormatter()
	expectedAddress := "[2001:db8::1]:8000"

	actualAddress := formatter.FormatHostAndPortForLogging(bindAddressIpvSixValue, portValue)
	if actualAddress != expectedAddress {
		t.Fatalf("expected IPv6 address to remain bracketed: expected %s, got %s", expectedAddress, actualAddress)
	}
}
