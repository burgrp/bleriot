# PAN211x Porting to Yinaguang (应广) Platform

**Source:** https://bbs.panchip.com/forum.php?mod=viewthread&tid=8423  
**Author:** Ding Jiachang (丁家昌)  
**Posted:** 2024-12-04 | **Replies:** 0 | **Views:** 5862

---

## Summary

Reference implementation for integrating PAN211x with the **Yinaguang (应广) PMS152** microcontroller using 3-wire SPI.

## Attachments

- **PAN211-2.4G-debug.rar** (260.8 KB, ~1877 downloads) — SPI communication reference code
- I²C method also referenced via the **PAN7050 SDK**

## Implementation Approach

| Protocol | MCU | Method |
|----------|-----|--------|
| 3-wire SPI | PMS152 | EASY RF SDK example |
| I²C | — | PAN7050 SDK reference |

- Use the **EASY RF** implementation from the PAN211x SDK as the starting point for SPI.
- For I²C, the PAN7050 chip SDK provides a usable reference despite being a different chip — the register access protocol is similar.

## Notes

- PMS152 is a low-cost 8-bit MCU from Yinaguang (应广科技); common in RC toys and consumer wireless products.
- 3-wire SPI = MOSI/MISO/CLK without a separate CS line (CS managed in software or via the chip's CSN pin).
