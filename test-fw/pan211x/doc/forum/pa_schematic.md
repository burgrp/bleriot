# PAN2110+PA Schematic (RFX2401)

**Source:** https://bbs.panchip.com/forum.php?mod=viewthread&tid=8467  
**Author:** Liu Min (刘敏) — Panchip Forum Administrator  
**Posted:** 2025-04-18 | **Replies:** 3 | **Views:** 4047

---

## Attachment

**File:** `PAN2110P0AA&RFX2401.pdf` (222.74 KB, ~2045 downloads)

Reference schematic for pairing the PAN2110 with an external PA (Power Amplifier) using the **RFX2401** component.

---

## Community Notes

### Transmit power when using an external PA

**Question (ddr1024, 2026-04-16):**  
"When adding a PA, what TX power level is recommended for the PAN2110?"

No official answer was posted at time of archival. See [hardware_layout.md](hardware_layout.md) for regulatory guidance:
- Single-layer PCB: ~5 dBm recommended for RED (European) certification
- FCC approval may allow up to ~9 dBm

Consult the PAN211x Hardware Design Reference (in this doc directory) for matching network and power level details.
