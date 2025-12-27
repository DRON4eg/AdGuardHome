//go:build !linux

package dnsforward

import (
	"context"
)

// createIpsets is a stub for non-Linux systems.
func (s *Server) createIpsets(_ context.Context, _ *IpsetCreateConfig) error {
	// IPSet is only supported on Linux.
	return nil
}

// validateIpsetsExist is a stub for non-Linux systems.
func (s *Server) validateIpsetsExist(_ context.Context, _ []string) (missing []string) {
	// IPSet is only supported on Linux.
	return nil
}
