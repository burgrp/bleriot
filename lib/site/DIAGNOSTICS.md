# BleRiot Diagnostic Metrics

This is the standalone reference for BleRiot diagnostics schema **8**. It
describes every Registry path published by the hub, the accounting rules behind
each value, and the corresponding Prometheus names. The implementation is in
`site/bridge/diag.go`; source counters come from the transaction engine and the
reconnecting radio supervisor.

## Enabling Diagnostics

Pass a non-empty Registry prefix to the hub:

```bash
bleriot hub --diagnostics bleriot
```

Diagnostics are disabled when `--diagnostics` is omitted or empty.
`--diag-interval` controls snapshot and publication cadence and defaults to one
second. Diagnostic TTL is the hub's `--ttl`, which defaults to 30 seconds. All
diagnostic registers are read-only.

The examples below use `bleriot` as the prefix. The publisher removes one
trailing `.` from the configured prefix before appending its paths.

## Types and Lifetime

Every diagnostic register has Registry type `int` and initial metadata:

```text
type=int, readOnly=true, diagnostic=true, schema=8
```

Metadata is sent when a path is first published by this process. Later updates
and TTL refreshes omit metadata. The active version is always the value of
`bleriot.hub.main.schema.version`; metadata retained on an existing Registry
path is not authoritative.

Metric kinds are:

- **Gauge:** a point-in-time value that may move in either direction.
- **Counter:** an unsigned cumulative value since this hub process created the
  corresponding component. It normally increases and resets on restart.
- **Timestamp:** Unix seconds; zero means the event has not occurred in this
  process lifetime.
- **State:** an integer enum documented in the channel catalog.
- **Duration sum:** a cumulative counter measured in microseconds.

Snapshots read independent atomics. During an active transaction or connection
transition, related values can briefly represent adjacent points in time. The
accounting identities below are exact for a quiescent snapshot and converge
after the operation finishes.

## Names and Escaping

Paths have one of three forms:

```text
<prefix>.hub.main.<metric>
<prefix>.node.<escaped-node-name>.<metric>
<prefix>.channel.<escaped-channel-name>.<metric>
```

A node or channel name is kept in one path component. Each `_` becomes `__`,
then each `.` becomes `_`:

```text
basement.fan -> basement_fan
zone_1       -> zone__1
```

This distinguishes `a.b` from `a_b`. The configured prefix itself is not
component-escaped.

The Registry Prometheus exporter maps dots to colons and also exposes path
components as positional labels. For example:

```text
Registry:   bleriot.node.basement_fan.transaction.get.outcome.timeout
Prometheus: bleriot:node:basement_fan:transaction:get:outcome:timeout
Labels:     n1="bleriot", n2="node", n3="basement_fan",
            n4="transaction", n5="get", n6="outcome", n7="timeout"
```

## Cardinality

| Scope | Paths |
|---|---:|
| Hub | 19 |
| Each configured node | 33 |
| Each configured RF channel | 18 |

For $N$ nodes and $C$ channels, the complete catalog contains:

$$
19 + 33N + 18C
$$

The 19 hub paths are 3 schema/process, 6 publisher, 2 latency totals, and 8
histogram buckets. Each node has 24 transaction paths and 9 packet paths. Each
channel has 12 state/connection paths and 6 packet paths.

## Hub Catalog

Hub paths use `bleriot.hub.main.`.

### Schema and process

| Suffix | Kind | Meaning |
|---|---|---|
| `schema.version` | Gauge | Active diagnostic schema, exactly `8`. |
| `process.started` | Timestamp | Time this diagnostics publisher was constructed. |
| `process.heartbeat` | Timestamp | Time of the latest in-memory catalog snapshot. |

### Publisher

| Suffix | Kind | Meaning |
|---|---|---|
| `publisher.batch.success` | Counter | Non-empty `SetRegisters` batches completed successfully. |
| `publisher.batch.error` | Counter | Non-empty batch calls that returned an error. |
| `publisher.values.sent` | Counter | Path values included in successful batches, including TTL refreshes. |
| `publisher.values.coalesced` | Counter | Catalog values omitted from successful non-empty cycles because they did not need publication. |
| `publisher.last.success` | Timestamp | Most recent successful batch. |
| `publisher.last.error` | Timestamp | Most recent failed batch. |

The publisher snapshots the whole catalog once per diagnostic interval and
sends at most one Registry `SetRegisters` batch per snapshot. A changed value is
included. Unchanged values are normally coalesced, then refreshed in stable,
distributed cohorts so every path is selected within half its TTL. The cohort
count is:

$$
\max\left(1, \left\lfloor\frac{TTL}{2 \times interval}\right\rfloor\right)
$$

A failed batch marks every included path pending; all are retried on the next
snapshot, with metadata again if none has yet succeeded. Failed batches do not
increase `values.sent` or `values.coalesced`. A snapshot with no values to send
does not count as a successful batch. Publisher self-counters are updated after
a successful batch is assembled, so the batch carrying their new values can
lag the event by one interval.

### Successful transaction latency

The hub histogram combines successful GET and SET transactions across all
configured nodes. Timing starts when `Engine.Get`, `Set`, or `SetNull` enters;
it includes node lookup, waiting for the channel lane, sends, response waits,
and retries, and ends when a matching VALUE or ACK arrives. Failed and canceled
transactions are excluded.

| Suffix | Kind | Meaning |
|---|---|---|
| `latency.success.count` | Counter | Successful GET and SET observations. |
| `latency.success.microseconds` | Duration sum | Sum of their wall times. |
| `latency.success.bucket.le_0_025` | Counter | Successful observations at or below 25 ms. |
| `latency.success.bucket.le_0_05` | Counter | At or below 50 ms. |
| `latency.success.bucket.le_0_1` | Counter | At or below 100 ms. |
| `latency.success.bucket.le_0_2` | Counter | At or below 200 ms. |
| `latency.success.bucket.le_0_5` | Counter | At or below 500 ms. |
| `latency.success.bucket.le_1` | Counter | At or below 1 s. |
| `latency.success.bucket.le_2` | Counter | At or below 2 s. |
| `latency.success.bucket.le_plus_Inf` | Counter | All successful observations. |

Buckets are cumulative. At quiescence:

$$
hub\ latency.count = \sum_{nodes}(GET\ latency.count + SET\ latency.count)
$$

$$
latency.bucket.le\_plus\_Inf = latency.success.count
$$

The process-lifetime mean in microseconds is
`latency.success.microseconds / latency.success.count`. The mean over a
Prometheus range is:

```promql
increase(bleriot:hub:main:latency:success:microseconds[$__range])
/
increase(bleriot:hub:main:latency:success:count[$__range])
```

A range histogram bucket count is the `increase()` of that bucket. Quantiles
can be estimated from the cumulative bucket increases; the finite bounds are
fixed and no per-node histogram is published.

## Per-Node Catalog

Node paths use `bleriot.node.<node>.`.

### GET and SET transactions

Both operations publish the following 12 suffixes under
`transaction.get.` and `transaction.set.`. SET includes ordinary assignments
and `SetNull` clear assignments.

| Relative suffix | Kind | Meaning |
|---|---|---|
| `outcome.success_first` | Counter | A matching response followed the initial send. |
| `outcome.success_retry` | Counter | A matching response followed one or more retries. |
| `outcome.timeout` | Counter | Every configured response window expired and the final drain did not end in cancellation. |
| `outcome.send_error` | Counter | A radio `Send` returned an error; send errors are terminal and are not retried. |
| `outcome.canceled` | Counter | Context cancellation ended the call before send, while queued, while waiting, or after timed-out attempts. |
| `outcome.no_radio` | Counter | The engine has no radio object registered for the node's channel. |
| `attempt.initial` | Counter | Transactions that entered their initial send attempt. |
| `attempt.retry` | Counter | Retry send attempts entered after earlier response timeouts. |
| `attempt.send_error` | Counter | Entered attempts whose `Send` returned an error. |
| `attempt.timeout` | Counter | Successfully sent attempts whose response window expired. |
| `latency.success.count` | Counter | Successful observations for this operation. |
| `latency.success.microseconds` | Duration sum | Sum of successful wall times for this operation. |

An **attempt** is one prospective packet send. An **outcome** is the one terminal
classification of an engine call. Every call for a known node increments one
outcome after metrics are resolved. A call for an unknown address returns
`ErrUnknownNode` and has no per-node path to increment.

At quiescence, for each operation:

$$
latency.success.count = outcome.success\_first + outcome.success\_retry
$$

$$
attempt.send\_error = outcome.send\_error
$$

Across the process lifetime, `attempt.initial` is at most the sum of all
outcomes: pre-send cancellation and `no_radio` add an outcome without an
attempt. `attempt.retry` may exceed transaction count because one transaction
can retry several times. `attempt.timeout` counts expired response windows, not
terminal timeout outcomes.

With one initial attempt and three configured retries, individual calls account
as follows:

- **First success:** `outcome.success_first += 1`, `attempt.initial += 1`, and
  successful latency count/sum increase. A GET also adds one matched VALUE; SET
  adds one matched ACK.
- **Success on the first retry:** `outcome.success_retry += 1`,
  `attempt.initial += 1`, `attempt.retry += 1`, `attempt.timeout += 1`, and
  successful latency increases. A duplicate or delayed response consumed by
  the post-retry drain is an orphan.
- **Terminal timeout:** `outcome.timeout += 1`, `attempt.initial += 1`,
  `attempt.retry += 3`, and `attempt.timeout += 4`. Successful latency and
  matched-response counters do not change.
- **Initial send error:** `outcome.send_error += 1`, `attempt.initial += 1`, and
  `attempt.send_error += 1`. There is no retry, response timeout, or latency
  observation. A send error after earlier response timeouts keeps those earlier
  retry/timeout increments and still has one terminal send-error outcome.
- **Cancellation before send or while queued:** only `outcome.canceled`
  increases. Cancellation after a successful send retains the attempts and any
  earlier response timeouts, drains possible late replies, and adds no latency.
- **No registered radio:** only `outcome.no_radio` increases. A registered
  reconnecting radio that is physically offline instead returns `ErrOffline`
  from `Send`; that is `outcome.send_error`, not `outcome.no_radio`, and the
  channel records `packet.tx.offline`.

### Response packets

| Suffix | Kind | Meaning |
|---|---|---|
| `packet.value.matched` | Counter | Valid VALUE packets that completed the active GET. |
| `packet.value.orphan` | Counter | Valid VALUE packets that did not match the active source, register, and expected type, or arrived during a drain. |
| `packet.value.null` | Counter | Valid VALUE packets carrying NULL, whether matched or orphaned. |
| `packet.ack.matched` | Counter | Valid ACK packets that satisfied the active SET transaction by confirming request receipt. |
| `packet.ack.orphan` | Counter | Valid ACK packets that did not match the active SET or arrived during a drain. |
| `packet.invalid.decode` | Counter | Packets attributed to this known plaintext source that failed version/decode validation. |
| `packet.invalid.type` | Counter | Successfully decoded packets whose type was neither VALUE nor ACK. |
| `packet.last.received` | Timestamp | Latest packet attributed to this known node by plaintext source, before validation. |
| `packet.last.valid` | Timestamp | Latest attributed packet that passed decode and VALUE/ACK type validation. |

Matching requires the active transaction to accept responses and the plaintext
source, decoded register, and decoded response type all to match. A valid packet
that fails those conditions is an orphan and cannot satisfy a later
transaction. NULL is a property of VALUE packets, so:

$$
packet.value.null \le packet.value.matched + packet.value.orphan
$$

At quiescence:

$$
packet.value.matched = GET\ outcome.success\_first + GET\ outcome.success\_retry
$$

$$
packet.ack.matched = SET\ outcome.success\_first + SET\ outcome.success\_retry
$$

Unknown plaintext sources are discarded without a node metric. A known
register is not required for packet validity: a VALUE or ACK for an unknown
register can still be valid and orphaned. The engine tracks unknown-register
and raw receive totals internally, but schema 8 does not publish them.

## Per-Channel Catalog

Channel paths use `bleriot.channel.<channel>.` and describe the stable
reconnecting endpoint assigned to an inventory channel.

### State and connection lifecycle

| Suffix | Kind | Meaning |
|---|---|---|
| `state` | State | Current endpoint state. |
| `state.since` | Timestamp | Time the current state began. |
| `connection.open.attempt` | Counter | Physical dongle open calls entered. |
| `connection.open.success` | Counter | Open calls that returned a dongle. |
| `connection.open.error` | Counter | Open calls that returned an error. |
| `connection.open.last_attempt` | Timestamp | Most recent open call start. |
| `connection.open.last_error` | Timestamp | Most recent failed open. |
| `connection.connected_at` | Timestamp | Most recent successful open. |
| `connection.disconnected_at` | Timestamp | Most recent established dongle dropped by transport failure. |
| `connection.disconnect.total` | Counter | Established dongles dropped after send or receive failure. |
| `connection.disconnect.send_error` | Counter | Drops first detected by `Send`. |
| `connection.disconnect.receive_error` | Counter | Drops first detected by `Receive`. |

State codes are:

| Value | Name | Meaning |
|---:|---|---|
| `0` | Offline | No physical dongle is open; the supervisor continues reconnecting. |
| `1` | Connected | A physical dongle is serving the channel. |
| `2` | Closed | `Close` was called; this endpoint will not reconnect. |

At quiescence:

$$
connection.open.attempt = connection.open.success + connection.open.error
$$

$$
connection.disconnect.total = connection.disconnect.send\_error + connection.disconnect.receive\_error
$$

An in-progress open can temporarily make attempts exceed completed open
outcomes by one. `disconnected_at` and disconnect counters do not increase for
an explicit `Close`.

### Packet traffic

| Suffix | Kind | Meaning |
|---|---|---|
| `packet.tx.attempt` | Counter | Calls to the reconnecting endpoint's `Send`. |
| `packet.tx.success` | Counter | Sends completed through a physical dongle. |
| `packet.tx.offline` | Counter | Sends attempted with no physical dongle connected. |
| `packet.tx.error` | Counter | Sends that reached a dongle and failed, triggering disconnect. |
| `packet.rx.success` | Counter | Packets returned successfully by the physical receive API. |
| `packet.rx.error` | Counter | Receive transport errors, each triggering disconnect. |

At quiescence:

$$
packet.tx.attempt = packet.tx.success + packet.tx.offline + packet.tx.error
$$

Each `packet.tx.error` causes one send-error disconnect, and each
`packet.rx.error` causes one receive-error disconnect, unless another goroutine
has already dropped that same physical dongle. The drop operation is
deduplicated by physical-device identity. Idle receive polls and receives while
offline are not packet attempts and do not increment a receive metric.

## Prometheus Queries

The examples assume the `bleriot` prefix and the positional labels shown above.

### GET success rate by node

```promql
sum by (n3) (
  rate({n1="bleriot",n2="node",n4="transaction",n5="get",n6="outcome",n7=~"success_first|success_retry"}[5m])
)
```

### GET terminal-timeout share

```promql
sum by (n3) (
  rate({n1="bleriot",n2="node",n4="transaction",n5="get",n6="outcome",n7="timeout"}[5m])
)
/
sum by (n3) (
  rate({n1="bleriot",n2="node",n4="transaction",n5="get",n6="outcome"}[5m])
)
```

### Mean successful GET latency by node

```promql
sum by (n3) (
  increase({n1="bleriot",n2="node",n4="transaction",n5="get",n6="latency",n7="success",n8="microseconds"}[5m])
)
/
sum by (n3) (
  increase({n1="bleriot",n2="node",n4="transaction",n5="get",n6="latency",n7="success",n8="count"}[5m])
)
```

### Valid-packet age

```promql
(time() - {n1="bleriot",n2="node",n4="packet",n5="last",n6="valid"})
and
({n1="bleriot",n2="node",n4="packet",n5="last",n6="valid"} > 0)
```

### Registry publication errors

```promql
increase({n1="bleriot",n2="hub",n3="main",n4="publisher",n5="batch",n6="error"}[1h])
```

## Resets and Interpretation

- Engine, publisher, and reconnecting-radio counters live in memory and reset
  when the hub restarts. `process.started` identifies the current publisher
  epoch. Prometheus `rate()` and `increase()` handle ordinary counter resets.
- Existing Registry values can remain visible until TTL expiry during an
  outage or after a node/channel is removed. A restarted publisher sends its
  current full catalog on its first batch.
- Timestamps use whole Unix seconds. Several events in one second can share a
  timestamp, and heartbeat can repeat when the interval is below one second.
- `packet.last.received` trusts only the plaintext source selector. Use
  `packet.last.valid` when decoded protocol activity is required.
- `send_error` identifies the host radio send path, including an offline
  reconnecting endpoint; it does not by itself prove an over-air RF failure.
- A terminal timeout means all response windows expired. It does not prove the
  node never handled a request; late or duplicate replies can appear as
  orphans during the bounded drain.
- Successful SET latency ends at ACK. ACK confirms request receipt, not
  acceptance, physical completion, or a resulting value. Later GETs publish the
  state currently reported by the node, which may be old or intermediate while
  an asynchronous action settles.
- Fleet latency is traffic-weighted. Busy nodes and operations contribute more
  observations than quiet ones, and queueing behind other channel work is part
  of measured latency.
