# generator — BleRiot code generator

Host-side tooling that turns hand-authored register descriptors into the two
artifacts BleRiot needs, from a single run:

- **Node code** (Go, compiled into firmware): `const` wire IDs, a slice of all
  IDs, and the descriptor version hash. No per-class wrappers — the firmware
  backs registers generically by `uint16` ID.
- **Node descriptor** (JSON, consumed by the [host hub](../host/README.md)): the
  resolved flat list of `id → {name, type, scaling, metadata}`.

Because both come from one run, firmware and hub **cannot drift**. The generator
is the *sole authority* that assigns wire IDs; descriptors never contain IDs.

> Module path: `generator`. Host tooling only — never compiled into firmware.
> See [protocol §11](../protocol/README.md#11-register-model-descriptors-and-code-generation).

---

## Authoring model

| Concept | Role | Carries wire IDs? |
|---------|------|-------------------|
| **Class descriptor** | A reusable, named set of registers (a "register profile") — names, types, scaling, metadata only. | No |
| **Node spec** | Composition of one or more class *instances*, plus channel and node metadata. | No |
| **Generated node code** | Firmware artifact: const wire IDs + version hash. | Yes (assigned) |
| **Generated node descriptor** | Hub artifact: resolved `id → name/meta`. | Yes (assigned) |

The qualified register name is `instanceName + "." + registerName`
(e.g. `outdoor.temperature`); these are the keys used by ID allocation and the
hub. A class may be instantiated more than once.

### Register types

| Type | Meaning |
|------|---------|
| `int` | No scaling. |
| `float` | `display = wire × multiplier / divider`. |
| `bool` | `0` = false, `1` = true; scaling ignored. |

The node always transmits raw `int32`; `type`/`multiplier`/`divider` are hub-side
hints only.

---

## Packages

| Package | Responsibility |
|---------|----------------|
| [`descriptor`](descriptor) | The authoring schema (class/register/node spec) and `AllocateIDs` — the deterministic FNV-1a-based wire-ID allocation (protocol §11.6). |
| [`codegen`](codegen) | `GenerateNodeCode` and `GenerateDescriptorJSON` — emit the firmware Go source and hub JSON from a resolved spec. |
| [`example`](example) | End-to-end demo: defines a class library + node spec, allocates IDs, writes both artifacts. |

---

## ID allocation (summary)

For every qualified register name, in lexicographic order:

1. Primary slot: `id = fnv1a32(name) & 0xFFFF` (`0x0000` is reserved, never used).
2. On collision, linear-probe `(id + 1) & 0xFFFF`, skipping `0x0000`.
3. A **version hash** is computed over the full resolved set and embedded in both
   artifacts, so the hub can detect a firmware/descriptor mismatch.

Allocation is stable across reordering the node spec. See
[protocol §11.6](../protocol/README.md#116-wire-id-allocation-generator-algorithm).

---

## Run the example

```sh
go run ./example            # writes to ./example/out
go run ./example /tmp/bgen  # writes to a custom directory
go test ./...
```

It produces, per node:

```
<out>/<node>_gen.go   firmware node code (const IDs + RegisterIDs + version)
<out>/<node>.json     hub node descriptor
```
