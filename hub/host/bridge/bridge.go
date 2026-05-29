// Package bridge connects the BleRiot protocol engine to the external Registry
// service. For every node register it acts as a Registry Provider: it publishes
// the register's value (seeded by a GET, then kept current by a WATCH push
// subscription) and turns consumer change requests into BleRiot SET operations.
//
// The bridge is generic — it has no per-class or per-register knowledge beyond
// the node descriptor (PROTOCOL.md §11.7). It depends on small interfaces so it
// can be tested without real hardware or a running registry; *engine.Engine and
// the reg client satisfy them.
package bridge

import (
	"context"
	"log/slog"
	"time"

	"hub/host/engine"
	"hub/host/node"
)

// Transactor is the engine surface the bridge needs. *engine.Engine satisfies it.
type Transactor interface {
	Get(ctx context.Context, addr [node.AddrLen]byte, reg uint16) (engine.Update, error)
	Set(ctx context.Context, addr [node.AddrLen]byte, reg uint16, value int32) (engine.Update, error)
	Watch(ctx context.Context, addr [node.AddrLen]byte, reg uint16, cb engine.Callback) error
}

// Registry is the registry-client surface the bridge needs. It matches the
// Provide method of github.com/burgrp/reg/pkg/client.Client.
type Registry interface {
	Provide(ctx context.Context, name string, value any, metadata map[string]any, ttl time.Duration) (chan<- any, <-chan any, error)
}

// Bridge publishes node registers to a Registry and applies consumer changes.
type Bridge struct {
	tx  Transactor
	reg Registry
	ttl time.Duration
	log *slog.Logger
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
	b := &Bridge{tx: tx, reg: reg, ttl: ttl, log: slog.New(discardHandler{})}
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
// served by its own goroutine; ServeNode returns immediately.
func (b *Bridge) ServeNode(ctx context.Context, n *node.Node) {
	for i := range n.Registers {
		r := &n.Registers[i]
		go b.serveRegister(ctx, n.Address, r)
	}
}

func (b *Bridge) serveRegister(ctx context.Context, addr [node.AddrLen]byte, r *node.Register) {
	log := b.log.With("register", r.Name, "id", r.ID)

	// Seed the initial value with a best-effort GET.
	var initial any
	if u, err := b.tx.Get(ctx, addr, r.ID); err == nil {
		initial = regValue(r, u)
		log.Debug("seeded initial value from node GET", "value", initial)
	} else {
		log.Debug("initial GET failed; providing null", "err", err)
	}

	md := metadata(r)
	log.Debug("registry provide", "value", initial, "metadata", md, "ttl", b.ttl)
	updates, requests, err := b.reg.Provide(ctx, r.Name, initial, md, b.ttl)
	if err != nil {
		log.Error("registry provide failed", "err", err)
		return
	}
	// Note: the reg client performs the initial value set fire-and-forget and
	// does not surface a connection error here, so this only confirms the
	// provider was set up locally — not that the registry server accepted it.
	log.Info("providing register to registry", "value", initial)

	// Node-side changes arrive via the watch callback (engine RX goroutine).
	// Decouple it from the registry with a buffered channel so a slow registry
	// never blocks packet reception.
	pushes := make(chan engine.Update, 16)
	_ = b.tx.Watch(ctx, addr, r.ID, func(u engine.Update) {
		select {
		case pushes <- u:
		default: // drop if backed up; the next push carries the latest state
			log.Debug("push dropped: buffer full")
		}
	})

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case u := <-pushes:
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
			wire, err := r.FromValue(req)
			if err != nil {
				log.Warn("ignoring malformed change request", "value", req, "err", err)
				continue // ignore malformed change requests
			}
			// Apply the write. The node's IS reply flows back through the watch
			// subscription and updates the registry, so we don't publish here.
			if _, err := b.tx.Set(ctx, addr, r.ID, wire); err != nil {
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
// plus the structural fields useful to consumers.
func metadata(r *node.Register) map[string]any {
	m := make(map[string]any, len(r.Metadata)+3)
	for k, v := range r.Metadata {
		m[k] = v
	}
	m["type"] = string(r.Type)
	if r.Class != "" {
		m["class"] = r.Class
	}
	if r.Instance != "" {
		m["instance"] = r.Instance
	}
	return m
}
