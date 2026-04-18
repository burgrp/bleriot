# XN297L to PAN2110 Migration Guide

**Source:** https://bbs.panchip.com/forum.php?mod=viewthread&tid=8715  
**Author:** Liu Min (刘敏) — Panchip Forum Administrator  
**Posted:** 2026-01-14 | **Replies:** 0 | **Views:** 909

---

## Attachment

**File:** PAN211x Series User Guide V1.2.pdf (1.42 MB, ~605 downloads, uploaded 2026-01-26)

---

## Hardware Changes

| Item | XN297L | PAN2110 |
|------|--------|---------|
| Crystal series resistor | 510 Ω | **0 Ω** (short) |
| Package | various | SOT23-8 or SOP8 |

Change the crystal oscillator series resistor from **510 Ω to 0 Ω** on the PCB.

---

## Software Changes

- The XN297L and PAN211x APIs are **not compatible** — the driver must be fully replaced.
- Follow the PAN211x SDK demo for the correct **init → TX/RX sequence and ordering**.
- Confirm the crystal frequency (16 MHz or 32 MHz) matches hardware.

---

## Interoperability: PAN2110 (TX) + XN297L (RX) at 250kbps

**Problem:** XN297L fails to receive packets from PAN2110 when both are configured for 250kbps.

**Diagnosis steps (from forum, posts 13–14):**

1. **Start at 1 Mbps.** 250kbps interoperability is more sensitive to frequency offset. Confirm the link works at 1M first.
2. **Check frequency deviation.** The PAN2110 matching network may introduce a frequency offset relative to XN297L. Measure the carrier frequency of both devices with a spectrum analyser or SDR.
3. If frequency offset is the issue, tune the PAN2110 matching network or use the frequency calibration registers.

---

## See Also

- [product_info.md](product_info.md) — community Q&A on 250kbps interoperability (posts 8, 11–14)
- [hardware_layout.md](hardware_layout.md) — antenna matching and layout guidance
