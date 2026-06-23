// Package bridge connects the BleRiot protocol engine to the external Registry
// service. For every node register it acts as a Registry Provider: it publishes
// the register's value (seeded by a GET, then kept current by watch-all push
// subscription) and turns consumer change requests into BleRiot SET operations.
//
// A node is watched with a single watch-all subscription (protocol/README.md
// §8.3) rather than one WATCH per register: this collapses the subscription
// refresh traffic to one packet per node and avoids overflowing a node's bounded
// subscription table when it exposes more registers than the table holds. The
// engine fans each register's push out to the matching register's provider, and
// when the node goes offline (§10) nulls every register at once.
//
// Each register is published under a name scoped by the device instance: the
// node's name (the base name of its instance file on the hub) is prefixed to the
// descriptor's qualified register name, e.g. node "kitchen" + register
// "heating.state" → "kitchen.heating.state". This keeps names distinct across
// devices that share one per-type descriptor.
//
// The bridge is generic — it has no per-class or per-register knowledge beyond
// the node descriptor (protocol/README.md §11.7). It depends on small interfaces so it
// can be tested without real hardware or a running registry; *engine.Engine and
// the reg client satisfy them.
package bridge

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"site/engine"
	"site/node"
)

// defaultSeedRetryInterval is how often serveRegister re-attempts the seeding
// GET for a watch-all register while the node is unreachable (§8.3): a watch-all
// draws no value dump, so the hub must GET the current value itself, and a GET
// can fail while the node is briefly offline (e.g. the dongle is still
// enumerating) or powers up later. Retrying keeps a register from being stuck
// null until its next spontaneous change. It is the same order as the
// subscription refresh cadence, and stops as soon as the node answers or a push
// lands.
const defaultSeedRetryInterval = 5 * time.Second

// Transactor is the engine surface the bridge needs. *engine.Engine satisfies it.
type Transactor interface {
	Get(ctx context.Context, addr [node.AddrLen]byte, reg uint16) (engine.Update, error)
	Set(ctx context.Context, addr [node.AddrLen]byte, reg uint16, value int32) error
	SetNull(ctx context.Context, addr [node.AddrLen]byte, reg uint16) error
	// WatchAll subscribes to every register of a node at once (§8.3). The
	// callback receives each register's push identified by its ID, and once with
	// reg == engine.RegAll and a NULL Update when the node is detected offline.
	WatchAll(ctx context.Context, addr [node.AddrLen]byte, cb engine.AllCallback) error
}

// Registry is the registry-client surface the bridge needs. It matches the
// Provide method of github.com/burgrp/reg/pkg/client.Client.
type Registry interface {
	Provide(ctx context.Context, name string, value any, metadata map[string]any, ttl time.Duration) (chan<- any, <-chan any, error)
}

// Bridge publishes node registers to a Registry and applies consumer changes.
type Bridge struct {
	tx        Transactor
	reg       Registry
	ttl       time.Duration
	seedRetry time.Duration // interval for retrying a failed seeding GET
	log       *slog.Logger
}

// Option configures a Bridge in New.
type Option func(*Bridge)

// WithLogger attaches an slog.Logger. The bridge logs Registry communication at
// debug level: the initial Provide per register, every value pushed to the
// registry, consumer change requests, and the resulting SET. A nil logger is
// ignored.
func WithLogger(l *slog.Logger) Option {
	return func(b *Bridge) {
		if l != nil {
			b.log = l
		}
	}
}

// New creates a Bridge. ttl is the Registry provider TTL for each register;
// the registry client refreshes it automatically.
func New(tx Transactor, reg Registry, ttl time.Duration, opts ...Option) *Bridge {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	b := &Bridge{tx: tx, reg: reg, ttl: ttl, seedRetry: defaultSeedRetryInterval, log: slog.New(discardHandler{})}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// discardHandler is a no-op slog.Handler used when no logger is supplied, so the
// bridge can always call b.log without nil checks.
type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (h discardHandler) WithAttrs([]slog.Attr) slog.Handler      { return h }
func (h discardHandler) WithGroup(string) slog.Handler           { return h }

// ServeNode bridges every register of n until ctx is cancelled. Each register is
// served by its own goroutine; a single watch-all subscription (§8.3) feeds
// every register's push channel. ServeNode returns immediately.
func (b *Bridge) ServeNode(ctx context.Context, n *node.Node) {
	// One push channel per register, fed by the shared watch-all subscription and
	// drained by that register's provider goroutine. Buffer the channel so a slow
	// registry never blocks packet reception.
	channels := make(map[uint16]chan engine.Update, len(n.Registers))
	for i := range n.Registers {
		r := &n.Registers[i]
		ch := make(chan engine.Update, 16)
		channels[r.ID] = ch
		go b.serveRegister(ctx, n.Name, n.Address, r, ch)
	}

	// A single watch-all subscription covers every register: the engine routes
	// each push by register ID, and signals the node going offline (§10) with
	// reg == engine.RegAll so we null every register at once.
	log := b.log.With("device", n.Name, "address", n.Address)
	if err := b.tx.WatchAll(ctx, n.Address, func(reg uint16, u engine.Update) {
		if reg == engine.RegAll {
			for _, ch := range channels {
				deliver(ch, engine.Update{Null: true})
			}
			return
		}
		if ch, ok := channels[reg]; ok {
			deliver(ch, u)
		}
	}); err != nil {
		log.Debug("initial watch-all failed; engine will keep retrying", "err", err)
	}
}

// deliver forwards a push to a register's channel, dropping it if the buffer is
// full — the next push carries the latest state.
func deliver(ch chan engine.Update, u engine.Update) {
	select {
	case ch <- u:
	default:
	}
}

// reseedLoop keeps a watch-all register's value fresh whenever it has none. A
// watch-all subscription (§8.3) never dumps current values, so the hub must GET
// them itself; this loop re-fetches the value every time the register loses it.
// seeded is cleared by a failed initial GET (the node was unreachable at
// startup) and by a NULL push — most importantly the offline signal (§10) the
// engine raises when the dongle disconnects, which nulls every register. The
// loop retries until the node answers: a successful GET, even one returning
// NULL, marks the register seeded and the loop idles until the value is lost
// again. This is what re-populates a register after the dongle is unplugged and
// replugged — the offline NULL clears seeded, and the first GET that succeeds
// once the device returns republishes the current value, without waiting for the
// register's next spontaneous change. It runs until ctx is cancelled.
func (b *Bridge) reseedLoop(ctx context.Context, addr [node.AddrLen]byte, r *node.Register, updates chan<- any, seeded *atomic.Bool, log *slog.Logger) {
	ticker := time.NewTicker(b.seedRetry)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if seeded.Load() {
				continue // value present; nothing to re-seed
			}
			u, err := b.tx.Get(ctx, addr, r.ID)
			if err != nil {
				continue // node unreachable; try again next tick
			}
			seeded.Store(true)
			v := regValue(r, u)
			select {
			case updates <- v:
				log.Debug("seeded value from node GET", "value", v)
			case <-ctx.Done():
				return
			}
		}
	}
}

func (b *Bridge) serveRegister(ctx context.Context, nodeName string, addr [node.AddrLen]byte, r *node.Register, pushes <-chan engine.Update) {
	// The Registry name is scoped by the device instance (the node's name, i.e.
	// the base name of its instance file on the hub), so the same per-type
	// descriptor used by many devices yields distinct, fully-qualified names —
	// e.g. node "kitchen" + register "heating.state" → "kitchen.heating.state".
	regName := nodeName + "." + r.Name
	log := b.log.With("register", regName, "id", r.ID)

	// Seed the initial value with a GET. Unlike a single-register WATCH, a
	// watch-all subscription (§8.3) does not dump current values, so the hub must
	// seed them itself. seeded tracks whether the register has a value yet, set
	// either by a successful GET (here or in the retry loop) or by the first push.
	var seeded atomic.Bool
	var initial any
	if u, err := b.tx.Get(ctx, addr, r.ID); err == nil {
		initial = regValue(r, u)
		seeded.Store(true)
		log.Debug("seeded initial value from node GET", "value", initial)
	} else {
		log.Debug("initial GET failed; will retry seeding", "err", err)
	}

	md := metadata(r, nodeName)
	log.Debug("registry provide", "value", initial, "metadata", md, "ttl", b.ttl)
	updates, requests, err := b.reg.Provide(ctx, regName, initial, md, b.ttl)
	if err != nil {
		log.Error("registry provide failed", "err", err)
		return
	}
	// Note: the reg client performs the initial value set fire-and-forget and
	// does not surface a connection error here, so this only confirms the
	// provider was set up locally — not that the registry server accepted it.
	log.Info("providing register to registry", "value", initial)

	// Keep the value re-seeded for the node's lifetime: the initial GET above
	// seeds it now, and this loop re-fetches it whenever it is later lost — when
	// the node was unreachable at startup, and after the dongle disconnects and
	// returns, when the node is signalled offline (§10) and every register nulled.
	go b.reseedLoop(ctx, addr, r, updates, &seeded, log)

	// Node-side changes arrive on the shared watch-all push channel (fed by the
	// engine RX goroutine). Forward them to the registry, decoupled so a slow
	// registry never blocks packet reception.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case u := <-pushes:
				// A value push seeds the register; a NULL push (the node going
				// offline, §10, or a genuine clear) clears it, re-arming reseedLoop so
				// the value is re-fetched once the node answers again.
				seeded.Store(!u.Null)
				v := regValue(r, u)
				select {
				case updates <- v:
					log.Debug("pushed value to registry", "value", v)
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-requests:
			if !ok {
				return
			}
			log.Debug("registry change request", "value", req)
			// A nil request clears the register: send a SET with the NULL flag
			// (§8.2) rather than coercing nil through FromValue.
			if req == nil {
				if err := b.tx.SetNull(ctx, addr, r.ID); err != nil {
					log.Warn("SET NULL failed", "err", err)
				} else {
					log.Debug("applied SET NULL to node")
				}
				continue
			}
			wire, err := r.FromValue(req)
			if err != nil {
				log.Warn("ignoring malformed change request", "value", req, "err", err)
				continue // ignore malformed change requests
			}
			// Apply the write. The node's value change flows back through the watch
			// subscription and updates the registry, so we don't publish here.
			if err := b.tx.Set(ctx, addr, r.ID, wire); err != nil {
				log.Warn("SET failed", "wire", wire, "err", err)
			} else {
				log.Debug("applied SET to node", "wire", wire)
			}
		}
	}
}

// regValue converts an engine Update into the Registry-facing value, mapping a
// NULL register to nil.
func regValue(r *node.Register, u engine.Update) any {
	if u.Null {
		return nil
	}
	return r.ToValue(u.Value)
}

// metadata builds the Registry metadata for a register: its descriptor metadata
// plus the structural fields useful to consumers (the value type and the device
// instance name).
func metadata(r *node.Register, deviceName string) map[string]any {
	m := make(map[string]any, len(r.Metadata)+2)
	for k, v := range r.Metadata {
		m[k] = v
	}
	m["type"] = string(r.Type)
	m["device"] = deviceName
	return m
}
