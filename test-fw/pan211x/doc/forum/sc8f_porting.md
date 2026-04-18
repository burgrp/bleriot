# PAN211x Porting to SC8F2892B (中微) Platform

**Source:** https://bbs.panchip.com/forum.php?mod=viewthread&tid=8422  
**Author:** Ding Jiachang (丁家昌)  
**Posted:** 2024-12-04 | **Replies:** 0 | **Views:** 6491

---

## Summary

Reference implementation for integrating PAN211x with the **SC8F2892B** microcontroller (中微/Sino Microelectronics) using 3-wire SPI.

## Attachment

**File:** `pan211_tx_rx.rar` (797.88 KB, ~1908 downloads)

Contains TX/RX source code and configuration examples for SC8F2892B ↔ PAN211 bidirectional wireless communication.

## Implementation Approach

| Protocol | MCU | Method |
|----------|-----|--------|
| 3-wire SPI | SC8F2892B | EASY RF SDK example |

- Based on the **EASY RF** implementation from the PAN211x SDK.
- The archive includes both transmit and receive sides, making it a complete bidirectional reference.

## Notes

- SC8F2892B is an 8-bit MCU from SinoMicro (中微半导体), common in consumer electronics and toy applications.
- This porting reference is one of the most downloaded threads in the forum (~1908 downloads), suggesting it is widely used as a starting point.
- For I²C porting on similar MCUs, see [yinaguang_porting.md](yinaguang_porting.md) which references the PAN7050 SDK.
