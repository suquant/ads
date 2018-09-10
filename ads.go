package ads

import (
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
	"net"
	"strings"
)

var log = clog.NewWithPlugin("ads")

type DNSAdBlock struct {
	Next       plugin.Handler
	BlockLists []string
	TargetIP   net.IP
	LogBlocks  bool
	blockMap   BlockMap
}

func (e DNSAdBlock) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	qname := state.Name()

	qname = strings.TrimSuffix(qname, ".")

	requestCount.WithLabelValues(metrics.WithServer(ctx)).Inc()
	requestCountBySource.WithLabelValues(metrics.WithServer(ctx), state.IP()).Inc()

	if e.blockMap[qname] {
		answers := a(state.Name(), []net.IP{e.TargetIP})
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative, m.RecursionAvailable = true, true
		m.Answer = answers

		w.WriteMsg(m)

		blockedRequestCount.WithLabelValues(metrics.WithServer(ctx)).Inc()
		blockedRequestCountBySource.WithLabelValues(metrics.WithServer(ctx), state.IP()).Inc()

		if e.LogBlocks {
			log.Infof("Blocked request %q from %q", qname, state.IP())
		}

		return dns.RcodeSuccess, nil
	}

	return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
}

// Name implements the Handler interface.
func (e DNSAdBlock) Name() string { return "ads" }

func a(zone string, ips []net.IP) []dns.RR {
	answers := []dns.RR{}
	for _, ip := range ips {
		r := new(dns.A)
		r.Hdr = dns.RR_Header{Name: zone, Rrtype: dns.TypeA,
			Class: dns.ClassINET, Ttl: 3600}
		r.A = ip
		answers = append(answers, r)
	}
	return answers
}