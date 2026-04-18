# PAN211x Product Resources & Technical Q&A

**Source:** https://bbs.panchip.com/forum.php?mod=viewthread&tid=8381  
**Author:** Liu Min (刘敏) — Panchip Forum Administrator  
**Posted:** 2024-08-06 | **Replies:** 14 | **Views:** 12218

---

## Official Resources

- **Product Manual:** PAN211x Product Manual V1.7.pdf (2.55 MB)
- **Datasheet:** PAN211x Series Datasheet V1.6 (1.4 MB)
- **SDK & documentation:** https://wiki.panchip.com/ble-lite/2-4g-t-rx/pan211x

**Package note:** PAN211x only supports SOP8 and SOT23-8 packages. The QFN20 package is the **PAN2160** model — not PAN211x.

## Recommended Crystal Parameters

| Parameter | Value |
|-----------|-------|
| Package | 3225 |
| Frequency | 32 MHz |
| Load Capacitance | 10 pF |
| Tolerance | ±10 PPM |

A 16 MHz crystal is also supported, but it limits the available frequency channels compared to 32 MHz.

---

## Technical Q&A from Thread

### Issue: Initialization hangs — register 0x6D stuck at 0x40

**Reporter:** Markson (2025-09-15)

In `PAN211_Init()` (file: `01_SDK/example_spi_3line/00_normal_tx/radio/pan211.c`), the code polls register `0x6D` waiting for the value `0x80`. It always reads `0x40`, causing an infinite loop.

**Root cause / fix (Liu Min):**  
Register `0x6D` holds Factory Trim (FT) data. The initialization sequence includes mandatory delays (55ms and 200µs) before this poll. These delays were too short. **Extend them** to allow the chip to settle.

**Resolution confirmed by Markson** — extending delays resolved the hang.

---

### Issue: Page 1 registers are undocumented

**Reporter:** michaelchain (2025-10-03)

The datasheet documents only part of Page 0 registers. Page 1 registers are entirely absent. This blocks implementation of:
- ACK enable/disable for frequency matching
- Channel switching after frequency calibration  
- Frequency hopping mode

**Response (Liu Min, 2026-01-22):**  
"Some register combinations are complex and not suitable for independent modification. Use the SDK example code as reference instead of manipulating Page 1 registers directly."

---

### Issue: `TRXTWTL_CFG` / `TRXTWTH_CFG` — undocumented TX/RX transition timing

**Reporter:** michaelchain (2025-10-04)

These two registers control the TX-to-RX and RX-to-TX transition wait times — critical for ACK transmission timing. They are not mentioned in the standard datasheet but appear in application notes. Misconfiguration causes ACK failure and requires extensive debugging to diagnose.

**Takeaway:** When setting up enhanced mode with auto-ACK, explicitly configure `TRXTWTL_CFG` and `TRXTWTH_CFG` from the SDK example values. Do not leave them at power-on defaults.

---

### Auto-ACK behavior in enhanced mode

**Clarification (michaelchain):**

- Enhanced mode automatically transmits an ACK upon receiving a packet — **no explicit `txStart` call is needed**.
- Packets with the `noack=1` flag set will **not** trigger an auto-ACK, even in enhanced mode.
- Firmware can still write to the TX FIFO even when `noack=1` is set.

---

### XN297L (250kbps) → PAN2110 interoperability

**Reporter:** zenzebin (2025-12-16)  
Converted XN297L code to PAN2110 but receives nothing on the XN297L side.

**Response (Liu Min, posts 13–14):**
1. First test with **1Mbps** data rate — 250kbps interoperability is harder due to frequency offset.
2. Check for **frequency deviation / offset** between the two chips.
3. PAN2110 matching network may introduce frequency offset relative to XN297L.
4. See the XN297L→PAN2110 migration guide: [xn297l_migration.md](xn297l_migration.md)

---

### XN297L → PAN2110 hardware changes

**Post by 侯旭辉 (2026-01-21):**

- Change the crystal oscillator series resistor: **510Ω → 0Ω**
- Replace the driver software entirely (XN297L and PAN211x APIs are different)
- Update the transceiver flow: use PAN211x SDK demo sequence for init/TX/RX ordering
- Confirm crystal frequency matches hardware (16MHz or 32MHz)

---

## Purchase / Support

Taobao Panchip store was closed at time of writing.  
Contact via WeChat: **Miu_3603** (note "bbs" when messaging for offline purchase/support).
