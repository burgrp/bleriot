# PAN211x/PAN2110 Frequently Asked Questions

**Source:** https://bbs.panchip.com/forum.php?mod=viewthread&tid=8737  
**Author:** Liu Min (刘敏) — Panchip Forum Administrator  
**Posted:** 2026-03-18 | **Replies:** 0 | **Views:** 294

---

## Attachment

**File:** PAN211x 常见问题.pdf (235.12 KB) — official FAQ PDF from Panchip

The full FAQ is in the attached PDF. The thread post itself highlights one key distinction:

---

## 16 MHz vs 32 MHz Crystal — Effect on Frequency Channels

The choice of crystal oscillator frequency affects the **number of available RF channels**:

- **32 MHz crystal**: Full channel set available (recommended — see [product_info.md](product_info.md) for crystal specs)
- **16 MHz crystal**: Reduced set of available frequency channels

When designing for a specific channel plan or for interoperability with other devices, confirm the crystal frequency matches the expected channel table.

---

## Other Known FAQs (from other threads)

See [product_info.md](product_info.md) for:
- Initialization hang on register 0x6D
- Auto-ACK behavior in enhanced mode
- `TRXTWTL_CFG` / `TRXTWTH_CFG` undocumented transition timing
- Page 1 registers (undocumented — use SDK examples)

See [xn297l_migration.md](xn297l_migration.md) for XN297L → PAN2110 migration questions.
