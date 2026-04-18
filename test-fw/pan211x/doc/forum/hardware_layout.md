# PAN2110 Hardware Layout Considerations

**Source:** https://bbs.panchip.com/forum.php?mod=viewthread&tid=8738  
**Author:** Liu Min (刘敏) — Panchip Forum Administrator  
**Posted:** 2026-03-18 | **Replies:** 1 | **Views:** 334

---

## Antenna Trace Layout

- Strictly follow the hardware design documentation for antenna layout and impedance matching.
- Control RF trace width between **0.5–1 mm**.
- Avoid inconsistencies between trace width and component pad sizes — these break impedance continuity.

## Ground Plane Spacing

- Maintain distance between copper ground planes and RF traces at **0.2–0.4 mm**, adjusted for PCB manufacturing capability.
- Goal: preserve 50 Ω impedance matching throughout the RF path.

## Pre-allocate Antenna Matching Components

- Always reserve footprint space for antenna matching components (L/C pi-network), even if populated with 0Ω/DNP initially.
- This allows post-production tuning without board respins.

## Regulatory / Certification Guidance

| Certification | Board Type | Recommended TX Power |
|---------------|------------|----------------------|
| RED (Europe) | Single-layer | ~5 dBm |
| FCC (USA) | Single-layer | up to ~9 dBm |

- Reference: PAN211x Regulatory Design Reference materials (available from Panchip support).
- Board material, package, TX power, and test distance all factor into certification strategy.

## Reference Documents

- PAN211x Hardware Design Guide: `test-fw/pan211x/doc/hardware_design_reference.md`
- PAN2110+PA reference schematic: [pa_schematic.md](pa_schematic.md)
