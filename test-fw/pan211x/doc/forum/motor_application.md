# PAN2110 Motor Application Precautions

**Source:** https://bbs.panchip.com/forum.php?mod=viewthread&tid=8650  
**Author:** Liu Min (刘敏) — Panchip Forum Administrator  
**Posted:** 2025-10-31 | **Replies:** 2 | **Views:** 1708

---

## Attachment

**File:** PAN211x Series Motor Application Guide (PDF, 1 MB, ~1068 downloads)

---

## Key Recommendations for Motor + Wireless Applications

Motor-driven systems generate significant electrical noise that can interfere with 2.4 GHz RF operation. The following precautions apply when using PAN2110 in products with motors (e.g., RC toys, appliances, drones).

### PCB Layout

- **Isolate ground planes**: Use separate copper pours for the RF/chip ground and the motor ground. Bridge them at a single point near the power supply.
- **Add filtering capacitors** on both the motor supply and the MCU/RF power rails.
- **Keep communication traces (SPI/I²C) short** to minimize coupling with motor noise.

### Component Selection

- Replace noisy motor driver ICs with **lower-noise alternatives** where possible.
- Consider switching to a **low-noise motor** if RF reception is marginal.
- For space-constrained designs, prefer small-package options: **PAN2110-SOT23-8** or an integrated SoC variant.

### Power Supply

- Decouple the RF supply independently from the motor supply.
- Use ferrite beads or LC filters between motor and RF power domains.
