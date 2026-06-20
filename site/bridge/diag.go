package bridge

import (
	"context"
	"log/slog"
	"time"

	"site/engine"
	"site/node"
	"site/radio"
)

// diagFlush is how often diagnostic registers are sampled and republished. It is
// a fixed implementation detail (the meaningful tuning dial is the rate window),
// chosen well below the registry TTL so values never lapse.
const diagFlush = 5 * time.Second

// DiagNodeSource provides per-node diagnostic counters. *engine.Engine satisfies
// it via SnapshotNode.
type DiagNodeSource interface {
	SnapshotNode(addr [node.AddrLen]byte) engine.NodeStats
}

// DiagNode names one node whose diagnostics are published.
type DiagNode struct {
	Name string
	Addr [node.AddrLen]byte
}

// DiagDongle names one dongle (by its channel name) whose diagnostics are
// published, with a function returning its current stats. *radio.Reconnecting
// satisfies the stats source via Stats.
type DiagDongle struct {
	Name  string
	Stats func() radio.DongleStats
}

// Diagnostics publishes hub-side synthetic diagnostic registers — per-node RF
// traffic and liveness, and per-dongle connection state and traffic — to the
// Registry. Every register is read-only: consumer change requests are drained
// and ignored. Counters are cumulative since hub start; their rate.* siblings are
// per-second averages over a trailing window.
type Diagnostics struct {
	src    DiagNodeSource
	reg    Registry
	prefix string
	window time.Duration
	flush  time.Duration
	ttl    time.Duration
	log    *slog.Logger
}

// DiagOption configures a Diagnostics in NewDiagnostics.
type DiagOption func(*Diagnostics)

// WithDiagLogger attaches an slog.Logger. A nil logger is ignored.
func WithDiagLogger(l *slog.Logger) DiagOption {
	return func(d *Diagnostics) {
		if l != nil {
			d.log = l
		}
	}
}

// NewDiagnostics creates a Diagnostics publisher. prefix namespaces every
// register (e.g. "diag" yields "diag.<node>.online"); window is the rate
// averaging span (default 30s); ttl is the registry provider TTL for each
// register.
func NewDiagnostics(src DiagNodeSource, reg Registry, prefix string, window, ttl time.Duration, opts ...DiagOption) *Diagnostics {
	if window <= 0 {
		window = 30 * time.Second
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	d := &Diagnostics{
		src:    src,
		reg:    reg,
		prefix: prefix,
		window: window,
		flush:  diagFlush,
		ttl:    ttl,
		log:    slog.New(discardHandler{}),
	}
	for _, o := range opts {
		o(d)
	}
	return d
}

// Serve publishes diagnostics for the given nodes and dongles until ctx is
// cancelled. It returns immediately; sampling and publishing run in background
// goroutines.
func (d *Diagnostics) Serve(ctx context.Context, nodes []DiagNode, dongles []DiagDongle) {
	nodeSets := make([]*nodeMetricSet, len(nodes))
	for i, n := range nodes {
		nodeSets[i] = &nodeMetricSet{prefix: d.prefix, name: n.Name, addr: n.Addr, ring: ring{window: d.window}}
	}
	dongleSets := make([]*dongleMetricSet, len(dongles))
	for i, dg := range dongles {
		dongleSets[i] = &dongleMetricSet{prefix: d.prefix, name: dg.Name, stats: dg.Stats, ring: ring{window: d.window}}
	}

	inboxes := make(map[string]chan any)
	provide := func(nt namedType) {
		updates, requests, err := d.reg.Provide(ctx, nt.name, nil, diagMeta(nt.typ), d.ttl)
		if err != nil {
			d.log.Error("diagnostics provide failed", "register", nt.name, "err", err)
			return
		}
		in := make(chan any, 1)
		inboxes[nt.name] = in
		go diagForward(ctx, in, updates)
		go diagDrain(ctx, requests) // read-only: discard change requests
	}
	for _, s := range nodeSets {
		for _, nt := range s.names() {
			provide(nt)
		}
	}
	for _, s := range dongleSets {
		for _, nt := range s.names() {
			provide(nt)
		}
	}
	d.log.Info("serving diagnostics", "prefix", d.prefix, "nodes", len(nodes), "dongles", len(dongles), "window", d.window)

	go d.run(ctx, nodeSets, dongleSets, inboxes)
}

// run samples every set on each flush tick and pushes the latest values to the
// registry. It seeds an immediate publish so values appear without waiting a
// full flush interval.
func (d *Diagnostics) run(ctx context.Context, nodeSets []*nodeMetricSet, dongleSets []*dongleMetricSet, inboxes map[string]chan any) {
	publish := func() {
		now := time.Now()
		for _, s := range nodeSets {
			for name, v := range s.values(d.src, now) {
				if in, ok := inboxes[name]; ok {
					latestPush(in, v)
				}
			}
		}
		for _, s := range dongleSets {
			for name, v := range s.values(now) {
				if in, ok := inboxes[name]; ok {
					latestPush(in, v)
				}
			}
		}
	}
	publish()
	t := time.NewTicker(d.flush)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			publish()
		}
	}
}

// namedType is a register name paired with its hub-side value type.
type namedType struct {
	name string
	typ  string
}

// nodeCounters and dongleCounters fix the order of the cumulative counter
// vectors used for rate computation; the same order maps NodeStats/DongleStats
// fields to register name suffixes.
var (
	nodeCounters   = []string{"rx.all", "rx.is", "rx.acks", "rx.corrupt", "tx.all", "tx.retries", "timeouts"}
	dongleCounters = []string{"tx.all", "tx.err", "rx.all"}
)

func nodeVector(s engine.NodeStats) []uint64 {
	return []uint64{s.RxAll, s.RxIS, s.RxACK, s.RxCorrupt, s.TxAll, s.TxRetries, s.Timeouts}
}

func dongleVector(s radio.DongleStats) []uint64 {
	return []uint64{s.TxAll, s.TxErr, s.RxAll}
}

// nodeMetricSet publishes one node's diagnostic registers and tracks its counter
// history for rate computation.
type nodeMetricSet struct {
	prefix string
	name   string
	addr   [node.AddrLen]byte
	ring   ring
}

func (m *nodeMetricSet) prefixDot() string { return m.prefix + ".node." + m.name + "." }

func (m *nodeMetricSet) names() []namedType {
	p := m.prefixDot()
	out := []namedType{
		{p + "online", "bool"},
		{p + "seen", "int"},
		{p + "misses", "int"},
	}
	for _, c := range nodeCounters {
		out = append(out, namedType{p + "count." + c, "int"}, namedType{p + "rate." + c, "float"})
	}
	return out
}

func (m *nodeMetricSet) values(src DiagNodeSource, now time.Time) map[string]any {
	s := src.SnapshotNode(m.addr)
	vec := nodeVector(s)
	rates := m.ring.sample(now, vec)
	p := m.prefixDot()
	out := map[string]any{
		p + "online": s.Online,
		p + "seen":   s.LastRx,
		p + "misses": int64(s.Misses),
	}
	for i, c := range nodeCounters {
		out[p+"count."+c] = int64(vec[i])
		out[p+"rate."+c] = rates[i]
	}
	return out
}

// dongleMetricSet publishes one dongle's diagnostic registers and tracks its
// counter history for rate computation.
type dongleMetricSet struct {
	prefix string
	name   string
	stats  func() radio.DongleStats
	ring   ring
}

func (m *dongleMetricSet) prefixDot() string { return m.prefix + ".dongle." + m.name + "." }

func (m *dongleMetricSet) names() []namedType {
	p := m.prefixDot()
	out := []namedType{
		{p + "connected", "bool"},
		{p + "reconnects", "int"},
		{p + "since", "int"},
	}
	for _, c := range dongleCounters {
		out = append(out, namedType{p + "count." + c, "int"}, namedType{p + "rate." + c, "float"})
	}
	return out
}

func (m *dongleMetricSet) values(now time.Time) map[string]any {
	s := m.stats()
	vec := dongleVector(s)
	rates := m.ring.sample(now, vec)
	p := m.prefixDot()
	out := map[string]any{
		p + "connected":  s.Connected,
		p + "reconnects": int64(s.Reconnects),
		p + "since":      s.Since,
	}
	for i, c := range dongleCounters {
		out[p+"count."+c] = int64(vec[i])
		out[p+"rate."+c] = rates[i]
	}
	return out
}

// ring keeps a trailing window of cumulative counter samples and derives a
// per-second rate from the oldest sample still inside the window: rate =
// (newest - oldest) / elapsed. Before the window has filled, elapsed is the time
// actually spanned, so the rate is a correct average over the available history.
type ring struct {
	window time.Duration
	ts     []time.Time
	vs     [][]uint64
}

func (r *ring) sample(now time.Time, v []uint64) []float64 {
	r.ts = append(r.ts, now)
	r.vs = append(r.vs, v)
	cutoff := now.Add(-r.window)
	// Drop samples older than the window, keeping the first one that straddles
	// the cutoff as the rate baseline (and always at least two samples).
	for len(r.ts) > 2 && r.ts[1].Before(cutoff) {
		r.ts = r.ts[1:]
		r.vs = r.vs[1:]
	}
	rates := make([]float64, len(v))
	elapsed := now.Sub(r.ts[0]).Seconds()
	if elapsed > 0 {
		old := r.vs[0]
		for i := range v {
			rates[i] = float64(v[i]-old[i]) / elapsed
		}
	}
	return rates
}

// diagMeta builds the registry metadata for a diagnostic register: its hub-side
// value type and a flag marking it as a hub-synthesised diagnostic.
func diagMeta(typ string) map[string]any {
	return map[string]any{"type": typ, "diagnostic": true}
}

// diagForward relays the latest value from a per-register inbox to its registry
// updates channel, decoupling the slow registry from the sampling loop.
func diagForward(ctx context.Context, in <-chan any, updates chan<- any) {
	for {
		select {
		case <-ctx.Done():
			return
		case v := <-in:
			select {
			case updates <- v:
			case <-ctx.Done():
				return
			}
		}
	}
}

// diagDrain discards consumer change requests for a read-only diagnostic
// register until ctx is cancelled or the channel closes.
func diagDrain(ctx context.Context, requests <-chan any) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-requests:
			if !ok {
				return
			}
		}
	}
}

// latestPush stores v in a buffer-of-one inbox with latest-wins semantics: if a
// value is already pending it is replaced, so a stalled registry never makes the
// sampler block or queue stale values.
func latestPush(in chan any, v any) {
	select {
	case in <- v:
	default:
		select {
		case <-in:
		default:
		}
		select {
		case in <- v:
		default:
		}
	}
}
