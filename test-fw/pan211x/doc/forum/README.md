# PAN211x Forum Knowledge Base

Translated and archived from https://bbs.panchip.com/forum.php?mod=forumdisplay&fid=159  
Forum section: BLE-Lite Series 2.4GHz TRX > PAN211x  
Archived: 2026-04-18

## Thread Index

| File | Title | Author | Date | Replies | Views |
|------|-------|--------|------|---------|-------|
| [product_info.md](product_info.md) | PAN211x Product Resources & Technical Q&A | Liu Min | 2024-08-06 | 14 | 12218 |
| [pa_schematic.md](pa_schematic.md) | PAN2110+PA Schematic (RFX2401) | Liu Min | 2025-04-18 | 3 | 4047 |
| [hardware_layout.md](hardware_layout.md) | PAN2110 Hardware Layout Considerations | Liu Min | 2026-03-18 | 1 | 334 |
| [motor_application.md](motor_application.md) | PAN2110 Motor Application Precautions | Liu Min | 2025-10-31 | 2 | 1708 |
| [faq.md](faq.md) | PAN211x/PAN2110 Frequently Asked Questions | Liu Min | 2026-03-18 | 0 | 294 |
| [xn297l_migration.md](xn297l_migration.md) | XN297L to PAN2110 Migration Guide | Liu Min | 2026-01-14 | 0 | 909 |
| [yinaguang_porting.md](yinaguang_porting.md) | Porting to Yinaguang (应广) Platform | Ding Jiachang | 2024-12-04 | 0 | 5862 |
| [sc8f_porting.md](sc8f_porting.md) | Porting to SC8F2892B (中微) Platform | Ding Jiachang | 2024-12-04 | 0 | 6491 |

## Key Technical Highlights

- **Initialization hang**: Register 0x6D stuck at 0x40 — extend the 55ms and 200µs delays before polling (see [product_info.md](product_info.md))
- **Auto-ACK**: Enhanced mode ACKs automatically; `noack=1` packets skip ACK; `TRXTWTL_CFG`/`TRXTWTH_CFG` control TX/RX transition timing (undocumented but critical)
- **Page 1 registers**: Not documented in the datasheet; use SDK example code as reference
- **XN297L→PAN2110**: Test at 1Mbps first; crystal resistor change (510Ω → 0Ω); frequency offset may be an issue at 250kbps
- **Crystal**: 32MHz 3225 ±10ppm 10pF recommended; 16MHz also supported (affects available channels)
- **PA design**: Reference schematic for PAN2110+RFX2401 available; ~5dBm for RED cert, up to ~9dBm for FCC
- **SDK**: https://wiki.panchip.com/ble-lite/2-4g-t-rx/pan211x
