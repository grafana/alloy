// Package snmp implements a discovery.snmp component.
package snmp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	snmp_config "github.com/prometheus/snmp_exporter/config"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/discovery/snmp/fingerprint"
	"github.com/grafana/alloy/internal/featuregate"
)

const (
	defaultSnmpRefreshInterval = 30 * time.Second
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.snmp",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   discovery.Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configures the discovery.snmp component.
type Arguments struct {
	Interval           time.Duration `alloy:"refresh_interval,attr,optional"`
	Network            string        `alloy:"network,attr"`
	FingerprintDefPath string        `alloy:"fingerprint_file_path,attr"`
	FingerprintName    string        `alloy:"fingerprinter_name,attr"`
	SnmpConfigs        []string      `alloy:"snmp_exporter_configs,attr"`
	AuthName           string        `alloy:"auth_name,attr"`
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	_, _, err := net.ParseCIDR(args.Network)
	if err != nil {
		return fmt.Errorf("invalid network: %w", err)
	}
	return nil
}

// New returns a new instance of a discovery.snmp component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.DiscovererConfig, error) {
		newArgs := args.(Arguments)
		return &snmpDiscoveryConfig{
			args: newArgs,
			opts: opts,
		}, nil
	})
}

type Discovery struct {
	network  *net.IPNet
	log      *slog.Logger
	fp       *fingerprint.Fingerprinter
	auth     *snmp_config.Auth
	authName string
}

func NewSNMPDiscovery(args Arguments, log *slog.Logger) (*Discovery, error) {
	_, net, err := net.ParseCIDR(args.Network)
	if err != nil {
		return nil, fmt.Errorf("parsing network cidr: %w", err)
	}

	fps, err := fingerprint.LoadFingerprinters(args.FingerprintDefPath)
	if err != nil {
		return nil, fmt.Errorf("loading fingerprinting definitions: %w", err)
	}

	fp, ok := fps.Fingerprinters[args.FingerprintName]
	if !ok {
		return nil, fmt.Errorf("couldn't find fingerprinter with name %s in %v", args.FingerprintName, maps.Keys(fps.Fingerprinters))
	}

	cfg := snmp_config.Config{
		Auths:   map[string]*snmp_config.Auth{},
		Modules: map[string]*snmp_config.Module{},
	}
	for _, rawCfg := range args.SnmpConfigs {
		parsed := snmp_config.Config{}
		if err := yaml.UnmarshalStrict([]byte(rawCfg), &parsed); err != nil {
			return nil, fmt.Errorf("invalid snmp_exporter config: %s", err)
		}
		maps.Copy(cfg.Auths, parsed.Auths)
	}

	auth, ok := cfg.Auths[args.AuthName]
	if !ok {
		return nil, fmt.Errorf("no auth with name %s found", args.AuthName)
	}

	return &Discovery{
		network:  net,
		log:      log,
		fp:       &fp,
		auth:     auth,
		authName: args.AuthName,
	}, nil
}

func (d *Discovery) Refresh(ctx context.Context) ([]*targetgroup.Group, error) {
	q := make(chan net.IP)
	go func() {
		for ip := d.network.IP.Mask(d.network.Mask); d.network.Contains(ip); incIP(ip) {
			q <- slices.Clone(ip)
		}
		close(q)
	}()
	targetsLock := sync.Mutex{}
	targets := []*targetgroup.Group{}
	eg, ctx := errgroup.WithContext(ctx)
	for range 20 {
		eg.Go(func() error {
			for {
				select {
				case ip, more := <-q:
					if !more {
						return nil
					}
					log := d.log.With("target", ip.String())
					res, err := d.probe(ip)
					if err != nil {
						d.log.DebugContext(ctx, "snmp probing failed", "err", err)
						continue
					}
					log.InfoContext(ctx, "discovered snmp target", "name", res.SysName, "desc", res.SysDescr, "addr", ip.String(), "sysObjectID", res.SysObjectID)
					tiers := d.fp.MatchTiers(map[string]string{
						"sysObjectID": res.SysObjectID,
					})
					log.InfoContext(ctx, "resolved modules", "hot", tiers.Hot, "cold", tiers.Cold, "topo", tiers.Topology)
					targetsLock.Lock()
					targets = append(targets,
						&targetgroup.Group{
							Targets: []model.LabelSet{
								{
									model.AddressLabel: model.LabelValue(ip.String()),
								},
							},
							Labels: model.LabelSet{
								"name":                    model.LabelValue(ip.String()),
								"address":                 model.LabelValue(ip.String()),
								"module":                  model.LabelValue(strings.Join(tiers.Hot, ",")),
								"auth":                    model.LabelValue(d.authName),
								model.ScrapeIntervalLabel: "60s",
							},
							Source: ip.String() + "-hot",
						},
						&targetgroup.Group{
							Targets: []model.LabelSet{
								{
									model.AddressLabel: model.LabelValue(ip.String()),
								},
							},
							Labels: model.LabelSet{
								"name":                    model.LabelValue(ip.String()),
								"module":                  model.LabelValue(strings.Join(tiers.Cold, ",")),
								"auth":                    model.LabelValue(d.authName),
								model.ScrapeIntervalLabel: "300s",
							},
							Source: ip.String() + "-cold",
						},
						&targetgroup.Group{
							Targets: []model.LabelSet{
								{
									model.AddressLabel: model.LabelValue(ip.String()),
								},
							},
							Labels: model.LabelSet{
								"name":                    model.LabelValue(ip.String()),
								"module":                  model.LabelValue(strings.Join(tiers.Topology, ",")),
								"auth":                    model.LabelValue(d.authName),
								model.ScrapeIntervalLabel: "300s",
							},
							Source: ip.String() + "-topo",
						},
					)
					targetsLock.Unlock()
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})
	}

	err := eg.Wait()
	if err != nil {
		return nil, err
	}
	d.log.Info("finished snmp discovery", "targets", len(targets))
	return targets, nil
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

type probeResult struct {
	SysObjectID string
	SysName     string
	SysDescr    string
}

func (d *Discovery) probe(target net.IP) (probeResult, error) {
	res := probeResult{}
	log := d.log.With("target", target.String())
	log.Debug("probing snmp")
	cl := &gosnmp.GoSNMP{
		Target:  target.String(),
		Port:    161,
		Retries: 0,
		Timeout: time.Second,
	}
	d.auth.ConfigureSNMP(cl, "")
	if err := cl.Connect(); err != nil {
		return res, err
	}
	defer cl.Conn.Close()
	resp, err := cl.Get([]string{"1.3.6.1.2.1.1.2.0", "1.3.6.1.2.1.1.5.0", "1.3.6.1.2.1.1.1.0"})
	if err != nil {
		return res, err
	}
	if resp == nil || len(resp.Variables) == 0 {
		return res, errors.New("empty snmp response")
	}
	for _, v := range resp.Variables {
		oid := normalizeOID(v.Name)
		val := stringifyPDU(v)
		switch {
		case strings.HasSuffix(oid, "1.2.1.1.2.0"):
			res.SysObjectID = normalizeOID(val)
		case strings.HasSuffix(oid, "1.2.1.1.5.0"):
			res.SysName = val
		case strings.HasSuffix(oid, "1.2.1.1.1.0"):
			res.SysDescr = val
		}
	}
	return res, nil
}

func normalizeOID(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, ".")
	s = strings.TrimPrefix(s, "iso.")
	return s
}

func stringifyPDU(v gosnmp.SnmpPDU) string {
	switch val := v.Value.(type) {
	case string:
		return strings.TrimSpace(val)
	case []byte:
		return strings.TrimSpace(string(val))
	default:
		if v.Value == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(v.Value))
	}
}
