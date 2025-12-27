//go:build linux

package dnsforward

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AdguardTeam/golibs/logutil/slogutil"
	"github.com/digineo/go-ipset/v2"
	"github.com/ti-mo/netfilter"
)

// protoFamilyToString converts netfilter.ProtoFamily to a human-readable string.
func protoFamilyToString(family netfilter.ProtoFamily) string {
	switch family {
	case netfilter.ProtoIPv4:
		return "inet"
	case netfilter.ProtoIPv6:
		return "inet6"
	default:
		return fmt.Sprintf("unknown(%d)", family)
	}
}

// createIpsets creates ipsets defined in the configuration if they don't exist.
// It skips ipsets that already exist and logs the action.
func (s *Server) createIpsets(ctx context.Context, config *IpsetCreateConfig) error {
	if config == nil || !config.Enabled || len(config.Sets) == 0 {
		return nil
	}

	s.logger.InfoContext(ctx, "creating ipsets if missing", "count", len(config.Sets))

	for _, setConfig := range config.Sets {
		err := s.createSingleIpset(ctx, setConfig)
		if err != nil {
			s.logger.ErrorContext(
				ctx,
				"failed to create ipset",
				"name", setConfig.Name,
				slogutil.KeyError, err,
			)
			// Continue with next ipset instead of failing completely
			continue
		}
	}

	return nil
}

// validateIpsetsExist checks that all ipsets with given names exist in the
// system.  Returns a list of missing ipset names.  Distinguishes between
// "ipset not found" errors and system errors (permissions, netlink issues).
func (s *Server) validateIpsetsExist(ctx context.Context, names []string) (missing []string) {
	if len(names) == 0 {
		return nil
	}

	// Use IPv4 connection for header queries (family doesn't matter for existence check).
	conn, err := ipset.Dial(netfilter.ProtoIPv4, nil)
	if err != nil {
		s.logger.WarnContext(ctx, "cannot connect to netfilter for validation",
			slogutil.KeyError, err)
		// If we can't connect, skip validation - updateConfig will catch the error.
		return nil
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			s.logger.WarnContext(ctx, "closing ipset connection",
				slogutil.KeyError, closeErr)
		}
	}()

	for _, name := range names {
		_, err = conn.Header(name)
		if err != nil {
			// Check if this is "ipset not found" vs system error.
			// Netlink returns "The set with the given name does not exist" for missing ipsets.
			if isIpsetNotFoundError(err) {
				missing = append(missing, name)
			} else {
				// System error (permissions, netlink issues) - log and skip.
				s.logger.WarnContext(ctx, "cannot check ipset existence",
					"name", name,
					slogutil.KeyError, err)
			}
		}
	}

	return missing
}

// isIpsetNotFoundError returns true if the error indicates that an ipset
// does not exist, as opposed to a system error (permissions, netlink issues).
func isIpsetNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Netlink returns this message when ipset doesn't exist.
	return strings.Contains(errStr, "does not exist") ||
		strings.Contains(errStr, "name does not exist")
}

// createSingleIpset creates a single ipset if it doesn't exist.
func (s *Server) createSingleIpset(ctx context.Context, config IpsetSetConfig) error {
	// Determine protocol family
	var family netfilter.ProtoFamily
	switch config.Family {
	case "inet", "ipv4":
		family = netfilter.ProtoIPv4
	case "inet6", "ipv6":
		family = netfilter.ProtoIPv6
	default:
		return fmt.Errorf("unknown family %q, expected inet or inet6", config.Family)
	}

	// Connect to netfilter
	conn, err := ipset.Dial(family, nil)
	if err != nil {
		return fmt.Errorf("dialing netfilter: %w", err)
	}
	defer func() {
		closeErr := conn.Close()
		if closeErr != nil {
			s.logger.WarnContext(
				ctx,
				"closing ipset connection",
				slogutil.KeyError, closeErr,
			)
		}
	}()

	// Check if ipset already exists.
	headerPolicy, err := conn.Header(config.Name)
	if err == nil {
		s.logger.InfoContext(ctx, "ipset already exists, skipping creation",
			"name", config.Name)

		// Validate only if we got a valid header response.
		if headerPolicy == nil {
			s.logger.WarnContext(ctx, "ipset exists but header is empty",
				"name", config.Name)

			return nil
		}

		// Validate type.
		if headerPolicy.TypeName != nil {
			existingType := headerPolicy.TypeName.Get()
			if existingType != config.Type {
				s.logger.WarnContext(ctx, "existing ipset type mismatch",
					"name", config.Name,
					"expected", config.Type,
					"actual", existingType)
			}
		}

		// Validate family.
		if headerPolicy.Family != nil {
			existingFamily := netfilter.ProtoFamily(headerPolicy.Family.Value)
			if existingFamily != family {
				expectedStr := config.Family
				actualStr := protoFamilyToString(existingFamily)
				s.logger.WarnContext(ctx, "existing ipset family mismatch",
					"name", config.Name,
					"expected", expectedStr,
					"actual", actualStr)
			}
		}

		return nil
	}

	// Create the ipset
	s.logger.InfoContext(
		ctx,
		"creating ipset",
		"name", config.Name,
		"type", config.Type,
		"family", config.Family,
		"timeout", config.Timeout,
	)

	// Determine ipset type revision (typically 0 for basic types)
	var revision uint8 = 0

	// Prepare create options
	var opts []ipset.CreateDataOption

	// Add timeout if specified
	if config.Timeout > 0 {
		opts = append(opts, ipset.CreateDataTimeout(time.Duration(config.Timeout)*time.Second))
	}

	err = conn.Create(config.Name, config.Type, revision, family, opts...)
	if err != nil {
		return fmt.Errorf("creating ipset %q: %w", config.Name, err)
	}

	s.logger.InfoContext(
		ctx,
		"successfully created ipset",
		"name", config.Name,
		"type", config.Type,
	)

	return nil
}
