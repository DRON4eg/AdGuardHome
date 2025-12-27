package dnsforward

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/AdguardTeam/AdGuardHome/internal/agh"
	"github.com/AdguardTeam/AdGuardHome/internal/aghhttp"
	"github.com/AdguardTeam/AdGuardHome/internal/aghnet"
	"github.com/AdguardTeam/AdGuardHome/internal/aghtest"
	"github.com/AdguardTeam/AdGuardHome/internal/filtering"
	"github.com/AdguardTeam/dnsproxy/upstream"
	"github.com/AdguardTeam/golibs/httphdr"
	"github.com/AdguardTeam/golibs/netutil"
	"github.com/AdguardTeam/golibs/testutil"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO(e.burkov):  Use the better approach to testdata with a separate
// directory for each test, and a separate file for each subtest.  See the
// [configmigrate] package.

// emptySysResolvers is an empty [SystemResolvers] implementation that always
// returns nil.
type emptySysResolvers struct{}

// Addrs implements the aghnet.SystemResolvers interface for emptySysResolvers.
func (emptySysResolvers) Addrs() (addrs []netip.AddrPort) {
	return nil
}

// loadTestData loads the test data from the file with the given name into
// cases.
func loadTestData(tb testing.TB, casesFileName string, cases any) {
	tb.Helper()

	var f *os.File
	f, err := os.Open(filepath.Join("testdata", casesFileName))
	require.NoError(tb, err)
	testutil.CleanupAndRequireSuccess(tb, f.Close)

	err = json.NewDecoder(f).Decode(cases)
	require.NoError(tb, err)
}

const (
	jsonExt = ".json"

	// testBlockedRespTTL is the TTL for blocked responses to use in tests.
	testBlockedRespTTL = 10
)

func TestDNSForwardHTTP_handleGetConfig(t *testing.T) {
	filterConf := &filtering.Config{
		ProtectionEnabled:     true,
		BlockingMode:          filtering.BlockingModeDefault,
		BlockedResponseTTL:    testBlockedRespTTL,
		SafeBrowsingEnabled:   true,
		SafeBrowsingCacheSize: 1000,
		SafeSearchConf:        filtering.SafeSearchConfig{Enabled: true},
		SafeSearchCacheSize:   1000,
		ParentalCacheSize:     1000,
		CacheTime:             30,
	}
	forwardConf := ServerConfig{
		UDPListenAddrs: []*net.UDPAddr{},
		TCPListenAddrs: []*net.TCPAddr{},
		TLSConf:        &TLSConfig{},
		Config: Config{
			UpstreamDNS:            []string{"8.8.8.8:53", "8.8.4.4:53"},
			FallbackDNS:            []string{"9.9.9.10"},
			RatelimitSubnetLenIPv4: 24,
			RatelimitSubnetLenIPv6: 56,
			UpstreamMode:           UpstreamModeLoadBalance,
			EDNSClientSubnet:       &EDNSClientSubnet{Enabled: false},
			ClientsContainer:       EmptyClientsContainer{},
		},
		ConfModifier:  agh.EmptyConfigModifier{},
		ServePlainDNS: true,
	}
	s := createTestServer(t, filterConf, forwardConf)
	s.sysResolvers = &emptySysResolvers{}

	require.NoError(t, s.Start(testutil.ContextWithTimeout(t, testTimeout)))
	testutil.CleanupAndRequireSuccess(t, func() (err error) {
		return s.Stop(testutil.ContextWithTimeout(t, testTimeout))
	})

	defaultConf := s.conf

	w := httptest.NewRecorder()

	testCases := []struct {
		conf func() ServerConfig
		name string
	}{{
		conf: func() ServerConfig {
			return defaultConf
		},
		name: "all_right",
	}, {
		conf: func() ServerConfig {
			conf := defaultConf
			conf.UpstreamMode = UpstreamModeFastestAddr

			return conf
		},
		name: "fastest_addr",
	}, {
		conf: func() ServerConfig {
			conf := defaultConf
			conf.UpstreamMode = UpstreamModeParallel

			return conf
		},
		name: "parallel",
	}}

	var data map[string]json.RawMessage
	loadTestData(t, t.Name()+jsonExt, &data)

	for _, tc := range testCases {
		caseWant, ok := data[tc.name]
		require.True(t, ok)

		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(w.Body.Reset)

			s.conf = tc.conf()
			s.handleGetConfig(w, httptest.NewRequest(http.MethodGet, "/", nil))

			cType := w.Header().Get(httphdr.ContentType)
			assert.Equal(t, aghhttp.HdrValApplicationJSON, cType)
			assert.JSONEq(t, string(caseWant), w.Body.String())
		})
	}
}

func TestDNSForwardHTTP_handleSetConfig(t *testing.T) {
	filterConf := &filtering.Config{
		ProtectionEnabled:     true,
		BlockingMode:          filtering.BlockingModeDefault,
		BlockedResponseTTL:    testBlockedRespTTL,
		SafeBrowsingEnabled:   true,
		SafeBrowsingCacheSize: 1000,
		SafeSearchConf:        filtering.SafeSearchConfig{Enabled: true},
		SafeSearchCacheSize:   1000,
		ParentalCacheSize:     1000,
		CacheTime:             30,
	}
	forwardConf := ServerConfig{
		UDPListenAddrs: []*net.UDPAddr{},
		TCPListenAddrs: []*net.TCPAddr{},
		TLSConf:        &TLSConfig{},
		Config: Config{
			UpstreamDNS:            []string{"8.8.8.8:53", "8.8.4.4:53"},
			RatelimitSubnetLenIPv4: 24,
			RatelimitSubnetLenIPv6: 56,
			UpstreamMode:           UpstreamModeLoadBalance,
			EDNSClientSubnet:       &EDNSClientSubnet{Enabled: false},
			ClientsContainer:       EmptyClientsContainer{},
		},
		ConfModifier:  agh.EmptyConfigModifier{},
		ServePlainDNS: true,
	}
	s := createTestServer(t, filterConf, forwardConf)
	s.sysResolvers = &emptySysResolvers{}

	defaultConf := s.conf

	err := s.Start(testutil.ContextWithTimeout(t, testTimeout))
	assert.NoError(t, err)
	testutil.CleanupAndRequireSuccess(t, func() (err error) {
		return s.Stop(testutil.ContextWithTimeout(t, testTimeout))
	})

	w := httptest.NewRecorder()

	testCases := []struct {
		name    string
		wantSet string
	}{{
		name:    "upstream_dns",
		wantSet: "OK",
	}, {
		name:    "bootstraps",
		wantSet: "OK",
	}, {
		name:    "blocking_mode_good",
		wantSet: "OK",
	}, {
		name: "blocking_mode_bad",
		wantSet: "validating dns config: " +
			"blocking_ipv4 must be valid ipv4 on custom_ip blocking_mode",
	}, {
		name:    "ratelimit",
		wantSet: "OK",
	}, {
		name:    "ratelimit_subnet_len",
		wantSet: "OK",
	}, {
		name:    "ratelimit_whitelist_not_ip",
		wantSet: `decoding request: ParseAddr("not.ip"): unexpected character (at "not.ip")`,
	}, {
		name:    "edns_cs_enabled",
		wantSet: "OK",
	}, {
		name:    "edns_cs_use_custom",
		wantSet: "OK",
	}, {
		name:    "edns_cs_use_custom_bad_ip",
		wantSet: "decoding request: ParseAddr(\"bad.ip\"): unexpected character (at \"bad.ip\")",
	}, {
		name:    "dnssec_enabled",
		wantSet: "OK",
	}, {
		name:    "cache_size",
		wantSet: "OK",
	}, {
		name:    "cache_enabled",
		wantSet: "OK",
	}, {
		name:    "upstream_mode_parallel",
		wantSet: "OK",
	}, {
		name:    "upstream_mode_fastest_addr",
		wantSet: "OK",
	}, {
		name: "upstream_dns_bad",
		wantSet: `validating dns config: upstream servers: parsing error at index 0: ` +
			`cannot prepare the upstream: invalid address !!!: bad domain name "!!!": ` +
			`bad top-level domain name label "!!!": bad top-level domain name label rune '!'`,
	}, {
		name: "bootstraps_bad",
		wantSet: `validating dns config: checking bootstrap a: not a bootstrap: ParseAddr("a"): ` +
			`unable to parse IP`,
	}, {
		name:    "cache_bad_ttl",
		wantSet: `validating dns config: cache_ttl_min must be less than or equal to cache_ttl_max`,
	}, {
		name:    "upstream_mode_bad",
		wantSet: `validating dns config: upstream_mode: incorrect value "somethingelse"`,
	}, {
		name:    "local_ptr_upstreams_good",
		wantSet: "OK",
	}, {
		name: "local_ptr_upstreams_bad",
		wantSet: `validating dns config: private upstream servers: ` +
			`bad arpa domain name "non.arpa": not a reversed ip network`,
	}, {
		name:    "local_ptr_upstreams_null",
		wantSet: "OK",
	}, {
		name:    "fallbacks",
		wantSet: "OK",
	}, {
		name:    "blocked_response_ttl",
		wantSet: "OK",
	}, {
		name:    "multiple_domain_specific_upstreams",
		wantSet: "OK",
	}, {
		name:    "ipset_create_valid",
		wantSet: "OK",
	}, {
		name:    "ipset_create_empty_name",
		wantSet: "validating dns config: ipset_create.sets[0]: name cannot be empty",
	}, {
		name:    "ipset_create_invalid_type",
		wantSet: `validating dns config: ipset_create.sets[0]: invalid type "bad:type"`,
	}, {
		name:    "ipset_create_invalid_family",
		wantSet: `validating dns config: ipset_create.sets[0]: invalid family "bad", expected inet, inet6, ipv4 or ipv6`,
	}}

	var data map[string]struct {
		Req  json.RawMessage `json:"req"`
		Want json.RawMessage `json:"want"`
	}

	testData := t.Name() + jsonExt
	loadTestData(t, testData, &data)

	for _, tc := range testCases {
		// NOTE:  Do not use require.Contains, because the size of the data
		// prevents it from printing a meaningful error message.
		caseData, ok := data[tc.name]
		require.Truef(t, ok, "%q does not contain test data for test case %s", testData, tc.name)

		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(func() {
				s.dnsFilter.SetBlockingMode(
					filtering.BlockingModeDefault,
					netip.Addr{},
					netip.Addr{},
				)
				s.conf = defaultConf
				s.conf.Config.EDNSClientSubnet = &EDNSClientSubnet{}
				s.dnsFilter.SetBlockedResponseTTL(testBlockedRespTTL)
			})

			rBody := io.NopCloser(bytes.NewReader(caseData.Req))
			var r *http.Request
			r, err = http.NewRequest(http.MethodPost, "http://example.com", rBody)
			require.NoError(t, err)

			s.handleSetConfig(w, r)
			assert.Equal(t, tc.wantSet, strings.TrimSuffix(w.Body.String(), "\n"))
			w.Body.Reset()

			s.handleGetConfig(w, httptest.NewRequest(http.MethodGet, "/", nil))
			assert.JSONEq(t, string(caseData.Want), w.Body.String())
			w.Body.Reset()
		})
	}
}

// newLocalUpstreamListener creates a local upstream listener and returns its
// address.  The listener is started in a separate goroutine and stopped when
// the tb's test is finished.
func newLocalUpstreamListener(tb testing.TB, port uint16, h dns.Handler) (real netip.AddrPort) {
	tb.Helper()

	startCh := make(chan struct{})
	upsSrv := &dns.Server{
		Addr:              netip.AddrPortFrom(netutil.IPv4Localhost(), port).String(),
		Net:               "tcp",
		Handler:           h,
		NotifyStartedFunc: func() { close(startCh) },
	}

	pt := testutil.NewPanicT(tb)
	go func() {
		err := upsSrv.ListenAndServe()
		require.NoError(pt, err)
	}()

	<-startCh
	testutil.CleanupAndRequireSuccess(tb, upsSrv.Shutdown)

	return testutil.RequireTypeAssert[*net.TCPAddr](tb, upsSrv.Listener.Addr()).AddrPort()
}

func TestServer_HandleTestUpstreamDNS(t *testing.T) {
	hdlr := dns.HandlerFunc(func(w dns.ResponseWriter, m *dns.Msg) {
		err := w.WriteMsg(new(dns.Msg).SetReply(m))
		require.NoError(testutil.NewPanicT(t), err)
	})

	ups := (&url.URL{
		Scheme: "tcp",
		Host:   newLocalUpstreamListener(t, 0, hdlr).String(),
	}).String()

	const (
		upsTimeout = 100 * time.Millisecond

		hostsFileName = "hosts"
		upstreamHost  = "custom.localhost"
	)

	hostsListener := newLocalUpstreamListener(t, 0, hdlr)
	hostsUps := (&url.URL{
		Scheme: "tcp",
		Host:   netutil.JoinHostPort(upstreamHost, hostsListener.Port()),
	}).String()

	watcher := aghtest.NewFSWatcher()
	watcher.OnEvents = func() (e <-chan struct{}) { return nil }
	watcher.OnAdd = func(_ string) (err error) { return nil }
	watcher.OnShutdown = func(_ context.Context) (err error) { return nil }

	ctx := testutil.ContextWithTimeout(t, testTimeout)
	hc, err := aghnet.NewHostsContainer(
		ctx,
		testLogger,
		fstest.MapFS{
			hostsFileName: &fstest.MapFile{
				Data: []byte(hostsListener.Addr().String() + " " + upstreamHost),
			},
		},
		watcher,
		hostsFileName,
	)
	require.NoError(t, err)

	srv := createTestServer(t, &filtering.Config{
		BlockingMode: filtering.BlockingModeDefault,
		EtcHosts:     hc,
	}, ServerConfig{
		UDPListenAddrs:  []*net.UDPAddr{{}},
		TCPListenAddrs:  []*net.TCPAddr{{}},
		UpstreamTimeout: upsTimeout,
		TLSConf:         &TLSConfig{},
		Config: Config{
			UpstreamMode:     UpstreamModeLoadBalance,
			EDNSClientSubnet: &EDNSClientSubnet{Enabled: false},
			ClientsContainer: EmptyClientsContainer{},
		},
		ServePlainDNS: true,
	})
	srv.etcHosts = upstream.NewHostsResolver(hc)
	startDeferStop(t, srv)

	testCases := []struct {
		body     map[string]any
		wantResp map[string]any
		name     string
	}{{
		body: map[string]any{
			"upstream_dns": []string{hostsUps},
		},
		wantResp: map[string]any{
			hostsUps: "OK",
		},
		name: "etc_hosts",
	}, {
		body: map[string]any{
			"upstream_dns": []string{ups, "#this.is.comment"},
		},
		wantResp: map[string]any{
			ups: "OK",
		},
		name: "comment_mix",
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var reqBody []byte
			reqBody, err = json.Marshal(tc.body)
			require.NoError(t, err)

			w := httptest.NewRecorder()

			var r *http.Request
			r, err = http.NewRequest(http.MethodPost, "", bytes.NewReader(reqBody))
			require.NoError(t, err)

			srv.handleTestUpstreamDNS(w, r)
			require.Equal(t, http.StatusOK, w.Code)

			resp := map[string]any{}
			err = json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)

			assert.Equal(t, tc.wantResp, resp)
		})
	}

	t.Run("timeout", func(t *testing.T) {
		pt := testutil.NewPanicT(t)
		slowHandler := dns.HandlerFunc(func(w dns.ResponseWriter, m *dns.Msg) {
			time.Sleep(upsTimeout * 2)
			writeErr := w.WriteMsg(new(dns.Msg).SetReply(m))
			require.NoError(pt, writeErr)
		})
		sleepyUps := (&url.URL{
			Scheme: "tcp",
			Host:   newLocalUpstreamListener(t, 0, slowHandler).String(),
		}).String()

		req := map[string]any{
			"upstream_dns": []string{sleepyUps},
		}

		var reqBody []byte
		reqBody, err = json.Marshal(req)
		require.NoError(t, err)

		w := httptest.NewRecorder()

		var r *http.Request
		r, err = http.NewRequest(http.MethodPost, "", bytes.NewReader(reqBody))
		require.NoError(t, err)

		srv.handleTestUpstreamDNS(w, r)
		require.Equal(t, http.StatusOK, w.Code)

		resp := map[string]any{}
		err = json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err)

		require.Contains(t, resp, sleepyUps)
		sleepyRes := testutil.RequireTypeAssert[string](t, resp[sleepyUps])

		assert.True(t, strings.HasSuffix(sleepyRes, "i/o timeout"))
	})
}

func TestCheckIPSetCreate(t *testing.T) {
	testCases := []struct {
		name    string
		config  *jsonIpsetCreateConfig
		wantErr string
	}{{
		name:    "nil_config",
		config:  nil,
		wantErr: "",
	}, {
		name: "disabled",
		config: &jsonIpsetCreateConfig{
			Enabled: false,
			Sets: []jsonIpsetSetConfig{{
				Name:   "",
				Type:   "",
				Family: "",
			}},
		},
		wantErr: "",
	}, {
		name: "valid",
		config: &jsonIpsetCreateConfig{
			Enabled: true,
			Sets: []jsonIpsetSetConfig{{
				Name:    "test_set",
				Type:    "hash:ip",
				Family:  "inet",
				Timeout: 0,
			}},
		},
		wantErr: "",
	}, {
		name: "empty_name",
		config: &jsonIpsetCreateConfig{
			Enabled: true,
			Sets: []jsonIpsetSetConfig{{
				Name:   "",
				Type:   "hash:ip",
				Family: "inet",
			}},
		},
		wantErr: "ipset_create.sets[0]: name cannot be empty",
	}, {
		name: "empty_type",
		config: &jsonIpsetCreateConfig{
			Enabled: true,
			Sets: []jsonIpsetSetConfig{{
				Name:   "test",
				Type:   "",
				Family: "inet",
			}},
		},
		wantErr: "ipset_create.sets[0]: type cannot be empty",
	}, {
		name: "invalid_type",
		config: &jsonIpsetCreateConfig{
			Enabled: true,
			Sets: []jsonIpsetSetConfig{{
				Name:   "test",
				Type:   "invalid:type",
				Family: "inet",
			}},
		},
		wantErr: `ipset_create.sets[0]: invalid type "invalid:type"`,
	}, {
		name: "empty_family",
		config: &jsonIpsetCreateConfig{
			Enabled: true,
			Sets: []jsonIpsetSetConfig{{
				Name:   "test",
				Type:   "hash:ip",
				Family: "",
			}},
		},
		wantErr: "ipset_create.sets[0]: family cannot be empty",
	}, {
		name: "invalid_family",
		config: &jsonIpsetCreateConfig{
			Enabled: true,
			Sets: []jsonIpsetSetConfig{{
				Name:   "test",
				Type:   "hash:ip",
				Family: "invalid",
			}},
		},
		wantErr: `ipset_create.sets[0]: invalid family "invalid", expected inet, inet6, ipv4 or ipv6`,
	}, {
		name: "multiple_sets_second_invalid",
		config: &jsonIpsetCreateConfig{
			Enabled: true,
			Sets: []jsonIpsetSetConfig{{
				Name:   "valid_set",
				Type:   "hash:ip",
				Family: "inet",
			}, {
				Name:   "",
				Type:   "hash:net",
				Family: "inet6",
			}},
		},
		wantErr: "ipset_create.sets[1]: name cannot be empty",
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &jsonDNSConfig{IPSetCreate: tc.config}
			err := req.checkIPSetCreate()

			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, tc.wantErr, err.Error())
			}
		})
	}
}

func TestIpsetSetsEqual(t *testing.T) {
	testCases := []struct {
		name string
		a    []IpsetSetConfig
		b    []IpsetSetConfig
		want bool
	}{{
		name: "both_empty",
		a:    []IpsetSetConfig{},
		b:    []IpsetSetConfig{},
		want: true,
	}, {
		name: "both_nil",
		a:    nil,
		b:    nil,
		want: true,
	}, {
		name: "equal_single",
		a:    []IpsetSetConfig{{Name: "test", Type: "hash:ip", Family: "inet", Timeout: 0}},
		b:    []IpsetSetConfig{{Name: "test", Type: "hash:ip", Family: "inet", Timeout: 0}},
		want: true,
	}, {
		name: "different_length",
		a:    []IpsetSetConfig{{Name: "test", Type: "hash:ip", Family: "inet"}},
		b:    []IpsetSetConfig{},
		want: false,
	}, {
		name: "different_name",
		a:    []IpsetSetConfig{{Name: "test1", Type: "hash:ip", Family: "inet"}},
		b:    []IpsetSetConfig{{Name: "test2", Type: "hash:ip", Family: "inet"}},
		want: false,
	}, {
		name: "different_type",
		a:    []IpsetSetConfig{{Name: "test", Type: "hash:ip", Family: "inet"}},
		b:    []IpsetSetConfig{{Name: "test", Type: "hash:net", Family: "inet"}},
		want: false,
	}, {
		name: "different_family",
		a:    []IpsetSetConfig{{Name: "test", Type: "hash:ip", Family: "inet"}},
		b:    []IpsetSetConfig{{Name: "test", Type: "hash:ip", Family: "inet6"}},
		want: false,
	}, {
		name: "different_timeout",
		a:    []IpsetSetConfig{{Name: "test", Type: "hash:ip", Family: "inet", Timeout: 100}},
		b:    []IpsetSetConfig{{Name: "test", Type: "hash:ip", Family: "inet", Timeout: 200}},
		want: false,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := ipsetSetsEqual(tc.a, tc.b)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestShouldCreateIpsets(t *testing.T) {
	validConfig := &IpsetCreateConfig{
		Enabled: true,
		Sets: []IpsetSetConfig{{
			Name:   "test_set",
			Type:   "hash:ip",
			Family: "inet",
		}},
	}

	disabledConfig := &IpsetCreateConfig{
		Enabled: false,
		Sets: []IpsetSetConfig{{
			Name:   "test_set",
			Type:   "hash:ip",
			Family: "inet",
		}},
	}

	emptySetsConfig := &IpsetCreateConfig{
		Enabled: true,
		Sets:    []IpsetSetConfig{},
	}

	testCases := []struct {
		name               string
		effectiveCreate    *IpsetCreateConfig
		ipsetCreateChanged bool
		ipsetRulesChanged  bool
		want               bool
	}{{
		name:               "nil_config",
		effectiveCreate:    nil,
		ipsetCreateChanged: true,
		ipsetRulesChanged:  true,
		want:               false,
	}, {
		name:               "disabled_config",
		effectiveCreate:    disabledConfig,
		ipsetCreateChanged: true,
		ipsetRulesChanged:  true,
		want:               false,
	}, {
		name:               "empty_sets",
		effectiveCreate:    emptySetsConfig,
		ipsetCreateChanged: true,
		ipsetRulesChanged:  true,
		want:               false,
	}, {
		name:               "config_changed_only",
		effectiveCreate:    validConfig,
		ipsetCreateChanged: true,
		ipsetRulesChanged:  false,
		want:               true,
	}, {
		name:               "rules_changed_only",
		effectiveCreate:    validConfig,
		ipsetCreateChanged: false,
		ipsetRulesChanged:  true,
		want:               true,
	}, {
		name:               "both_changed",
		effectiveCreate:    validConfig,
		ipsetCreateChanged: true,
		ipsetRulesChanged:  true,
		want:               true,
	}, {
		name:               "nothing_changed",
		effectiveCreate:    validConfig,
		ipsetCreateChanged: false,
		ipsetRulesChanged:  false,
		want:               false,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldCreateIpsets(tc.effectiveCreate, tc.ipsetCreateChanged, tc.ipsetRulesChanged)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExtractIpsetNames(t *testing.T) {
	testCases := []struct {
		name  string
		rules []string
		want  []string
	}{{
		name:  "empty",
		rules: []string{},
		want:  nil,
	}, {
		name:  "single_rule_single_ipset",
		rules: []string{"example.com/my_set"},
		want:  []string{"my_set"},
	}, {
		name:  "single_rule_multiple_ipsets",
		rules: []string{"example.com/set1,set2,set3"},
		want:  []string{"set1", "set2", "set3"},
	}, {
		name:  "multiple_rules",
		rules: []string{"example.com/set1", "example.org/set2"},
		want:  []string{"set1", "set2"},
	}, {
		name:  "multiple_domains_single_ipset",
		rules: []string{"example.com,example.org/my_set"},
		want:  []string{"my_set"},
	}, {
		name:  "duplicate_ipsets",
		rules: []string{"example.com/set1", "example.org/set1,set2"},
		want:  []string{"set1", "set2"},
	}, {
		name:  "whitespace",
		rules: []string{"  example.com / set1 , set2  "},
		want:  []string{"set1", "set2"},
	}, {
		name:  "invalid_rule_no_slash",
		rules: []string{"invalid_rule"},
		want:  nil,
	}, {
		name:  "mixed_valid_invalid",
		rules: []string{"invalid", "example.com/valid_set", ""},
		want:  []string{"valid_set"},
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractIpsetNames(tc.rules)
			assert.Equal(t, tc.want, got)
		})
	}
}
