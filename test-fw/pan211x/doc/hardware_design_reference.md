# PAN211x Hardware Design Reference

**Version:** V1.2
**Release Date:** July 2025
**Manufacturer:** Shanghai Panchip Microelectronics Co., Ltd.

---

## Table of Contents

- [1. Schematic Design](#1-schematic-design)
  - [1.1 Reference Schematic](#11-reference-schematic)
    - [1.1.1 SOP8 Reference Schematic](#111-sop8-reference-schematic)
    - [1.1.2 Bill of Materials](#112-bill-of-materials)
  - [1.2 Power Supply Circuit](#12-power-supply-circuit)
  - [1.3 Crystal Circuit](#13-crystal-circuit)
    - [1.3.1 Recommended Crystal Parameters](#131-recommended-crystal-parameters)
    - [1.3.2 Internal Capacitor Frequency Adjustment Range](#132-internal-capacitor-frequency-adjustment-range)
  - [1.4 Antenna Matching Circuit](#14-antenna-matching-circuit)
  - [1.5 SPI/I2C Interface Circuit](#15-spii2c-interface-circuit)
- [2. PCB Design](#2-pcb-design)
  - [2.1 PCB Material and Stackup Design](#21-pcb-material-and-stackup-design)
  - [2.2 Power and Ground Layout](#22-power-and-ground-layout)
  - [2.3 Crystal Layout](#23-crystal-layout)
  - [2.4 SPI/I2C Interface Layout](#24-spii2c-interface-layout)
  - [2.5 RF Matching Circuit Layout](#25-rf-matching-circuit-layout)
  - [2.6 Antenna Layout](#26-antenna-layout)
  - [2.7 ESD Protection](#27-esd-protection)
  - [2.8 PCB Layout Examples](#28-pcb-layout-examples)

---

## 1. Schematic Design

### 1.1 Reference Schematic

#### 1.1.1 SOP8 Reference Schematic

![Reference Schematic](images/hdr_schematic_page.png)

**Figure 1:** PAN211x SOP8 Package Reference Schematic

The reference schematic shows a minimal application circuit with:
- Power supply decoupling capacitors (C1, C2)
- 32MHz or 16MHz crystal oscillator (Y1)
- Optional antenna matching components
- SPI/I2C interface connections

#### 1.1.2 Bill of Materials

| Reference | Value | Package | Description | Quantity |
|-----------|-------|---------|-------------|----------|
| C1 | 1μF | 0402 | NPO, ±10%, 16V | 1 |
| C2 | 100nF | 0402 | NPO, ±10%, 16V | 1 |
| R1 | 0Ω/4pF | 0402 | For PIFA antenna, use 4pF capacitor | 1 |
| Y1 | 32MHz | 3225 | Frequency tolerance ±10ppm; Load capacitance 10pF | 1 |
| C3, C4, C7, C8 | NC | - | Not connected (optional external load capacitors) | - |

**Notes:**
1. For modules without FCC/CE certification using PIFA antenna, a series capacitor (4pF) to the antenna is required to avoid high-power self-oscillation.
2. For FCC/CE certification, at least reserve a Pi-type matching structure. Component values should refer to the regulatory compliance design document.

---

### 1.2 Power Supply Circuit

The integrity of the power supply design affects chip performance. A good power supply design makes it easier to maximize wireless module performance.

#### Power Supply Requirements

- **Voltage Range:** 1.8V - 3.6V
- **Ripple:** Less than ±100mV
- **Ripple Frequency:** Less than 1MHz

#### Design Guidelines

1. **Current Margin:**
   - Output current capability should be greater than 2× peak current under normal conditions
   - Minimum 1.5× peak current if current margin is limited

2. **Ripple Management:**
   - In 3.3V supply systems, excessive ripple can couple through wires or ground plane to sensitive circuits
   - Sensitive signals include: antenna, feedlines, clock lines, and other critical RF signals
   - Excessive ripple degrades RF performance

3. **DC-DC Converter Usage:**
   - Wait for output voltage to stabilize after DC-DC enable before configuring the RF chip
   - After entering Deep Sleep mode, the DC-DC enable signal can be pulled low to reduce module base current

---

### 1.3 Crystal Circuit

#### 1.3.1 Recommended Crystal Parameters

1. **Crystal Frequency:** 32MHz or 16MHz
2. **ESR (Equivalent Series Resistance):** Less than 80Ω
3. **Crystal Load Capacitance:** 10pF
4. **Frequency Tolerance:** Within ±20ppm

#### 1.3.2 Internal Capacitor Frequency Adjustment Range

According to measurements with recommended crystal parameters, the internal capacitor can adjust frequency deviation by approximately ±140kHz. Different crystals and PCBs will have variations.

**Recommendation:** Reserve external capacitor positions C7 and C8. If the crystal frequency deviation is large, these capacitors can be used to adjust the frequency offset.

#### Frequency Adjustment Test Data

The table below shows carrier frequency changes in single carrier mode at 2440MHz when adjusting the internal capacitors. Red text indicates default configuration. PCB is FR4 double-sided board. Different board materials and crystals will have different results. Test data is for reference only.

**Test Conditions:**
- Mode: Single carrier
- Frequency point: 2440MHz
- PCB: FR4 double-sided board

**Crystal Specifications Tested:**

| Package | Load Cap | Freq Tolerance | ESR |
|---------|----------|----------------|-----|
| SMD3225 | 8pF, 9pF, 10pF, 12pF, 16pF, 18pF, 20pF | ±10ppm | ≤40-80Ω |
| DIP_49S | 10pF, 12pF, 20pF | ±20ppm | ≤30Ω |
| SMD2016 | 8pF | ±10ppm | - |

The internal capacitor adjustment provides fine-tuning of ±140kHz around the center frequency. The FSYNXO_CAP2 and FSYNXO_CAPSEL registers control the internal capacitance to adjust the crystal oscillator frequency.

**Default Configuration:** FSYNXO_CAP2=0, FSYNXO_CAPSEL=100000

*For detailed frequency adjustment tables, refer to the full hardware design reference document.*

---

### 1.4 Antenna Matching Circuit

Antenna matching components depend on whether FCC/CE certification is required.

#### Without Regulatory Requirements

- Matching structure should still be reserved (in case module power is low and matching optimization is needed)
- Use 0Ω resistor in series to antenna
- **PIFA antenna:** Requires ~4pF DC-blocking capacitor in series to antenna

#### With FCC/CE Certification

- Must reserve Pi-type matching network
- Component values determined through testing and regulatory compliance process

---

### 1.5 SPI/I2C Interface Circuit

PAN211x supports:
- **3-wire SPI:** CSN, SCK, DATA
- **I2C:** SCK, DATA

#### I2C Pull-up Configuration

- Internal pull-up configured through software
- Resistance value: approximately 4.7kΩ

#### Interface Mode Switching

Refer to "PAN211x Product Manual" Chapter 9 for different interface switching methods.

#### IRQ Functionality

- All interface modes support IRQ multiplexing
- In 3-wire SPI and I2C modes, IRQ is multiplexed with MOSI

#### Interface Speed

- **SPI:** Maximum 10Mbps
- **I2C:** Maximum 2Mbps
- **eFuse Operations:** Speed must not exceed half the crystal clock frequency

---

## 2. PCB Design

### 2.1 PCB Material and Stackup Design

**Recommendation:** Double-sided FR4 board structure

The final stackup structure should be determined based on the actual product requirements.

---

### 2.2 Power and Ground Layout

#### 1. Power Trace Width

- Power traces should be as wide as possible, preferably **≥20 mils**
- Power must pass through decoupling capacitors before reaching chip power pins
- Place two parallel capacitors close to chip power pins for low-pass filtering
- Small value capacitor should be placed closer to chip pins to better filter high-frequency noise
- Ensure good return path for filter capacitors
- On double-sided boards, place vias near ground pads to minimize return path

#### 2. Star Connection Topology

- Use radial (star) connection for power and ground lines
- Single-point connection to power/ground with separate traces
- RF chip power/ground traces should be separate from other chips or components
- Route from main reference power/ground separately to prevent interference
- **Recommended:** Use solid ground plane (not hatched)

#### 3. Ground Plane Connection

- Connect ground plane to low-noise ground or main reference ground
- Do not connect to high-signal or high-interference component grounds
- This effectively reduces overall board noise

---

### 2.3 Crystal Layout

![Crystal Layout](images/hdr_crystal_layout_page.png)

**Figure 3:** Crystal Layout Example

#### Layout Guidelines

1. **Trace Routing:**
   - 32MHz crystal traces to chip pins should be as wide and short as possible
   - **Do not use vias** on crystal traces

2. **Through-hole Crystal Pads:**
   - Ensure outer diameter and inner diameter difference is **≥0.2mm**

3. **Ground Plane:**
   - Complete ground plane on both sides of crystal pads and traces
   - Preferably no other traces or components in this area

4. **RF Isolation:**
   - Keep crystal away from RF traces to avoid interference with RF signals

5. **Antenna Isolation:**
   - To avoid interference from high-power transmission, keep crystal circuit (including load capacitors) away from antenna circuit
   - RF antenna section and crystal circuit should have ground isolation between them

---

### 2.4 SPI/I2C Interface Layout

#### 1. Proximity to MCU

- RF chip should be placed as close to MCU as possible
- Control lines should be as short as possible
- Layout should avoid strong interference sources
- Traces should have ground shields nearby to reduce interference risk

#### 2. Debug Connections

- When debugging, keep SPI/I2C external wire length within **15cm** to avoid interface signal instability

---

### 2.5 RF Matching Circuit Layout

RF matching circuit significantly affects RF performance and requires special attention.

#### Component Selection

- **Recommended:** 0402 package for matching components
- Follow reference schematic for matching structure

#### RF Matching Layout Principles

**1. Minimize Loss:**
- Trace from ANT pin to antenna matching circuit should be **<2mm**
- Pi-type matching circuit traces should be smooth and straight
- Parallel component pads should overlap with traces when possible
- **Prohibited:** Vias on RF traces (no layer changes)

**2. Trace Width:**
- Adjust based on matching component package (0402/0603)
- Width controlled between **0.5-1mm**
- Avoid width mismatch between trace and component pads (affects impedance continuity)

**3. Ground Planes:**
- Good ground pour on both sides of RF traces (multiple vias for multilayer boards)
- Spacing between ground and RF trace: **0.2-0.4mm** (based on PCB fabrication process)
- Maintain **50Ω impedance** matching
- Complete reference ground plane on back side under RF matching section
- Avoid placing components or traces on back side

**4. EPAD Connection:**
- Chip has exposed pad (EPAD)
- RF reference ground and EPAD must have good connection
- Multilayer boards: place **≥4 vias** on EPAD to connect to bottom ground layer

**5. Debugging Access:**
- Can place 0Ω resistor in series between ANT pin and Pi matching network
- Expose GND copper area next to resistor for antenna tuning

---

### 2.6 Antenna Layout

**Note:** For detailed antenna design, refer to "2.4G PCB Antenna Design Guide V1.0" (contact technical support to obtain document).

#### PIFA Antenna

- **Minimum clearance:** 1mm from ground plane copper
- **Bottom layer:** No ground plane under antenna section
- **Spacing to reference ground:** **≥1mm**

#### Clearance Requirements

- Antenna vicinity should be free of metal structures, components, and traces
- Maintain **≥3cm** clearance around antenna on PCB
- Avoid placing large metal-bodied components in this area

#### Wire Antenna

- **Feedpoint clearance:** ≥2mm around wire antenna feedpoint

---

### 2.7 ESD Protection

#### 1. TVS Protection Devices

- Add ESD protection devices (typically TVS) to sensitive signal lines
- **TVS Placement:** As close as possible to ESD source (connectors, etc.)
- Place farther from protected IC than ESD source
- Route ESD source directly to TVS
- Minimize parasitic inductance between TVS and return ground

#### 2. Trace Routing

- Keep sensitive signal traces away from PCB board edges
- To avoid crosstalk between traces and antenna, route traces away from antenna

#### 3. Copper Pour Management

- Remove isolated copper islands
- Use ground pour to wrap sensitive signals, reducing radiation interference

#### 4. Via Optimization

- Maximize via drill diameter and pad diameter to reduce parasitic inductance

#### 5. Trace Length

- Minimize trace length to reduce parasitic inductance
- **Avoid right-angle traces** as they produce greater electromagnetic radiation
- Use 45° or curved traces instead

---

### 2.8 PCB Layout Examples

![PCB Layout Examples](images/hdr_pcb_examples_page.png)

The examples show:
- **Single-sided remote control board** with wire antenna
- **Single-sided toy car board** with wire antenna
- **Double-sided module board** with PIFA antenna (Top and Bottom layers)

Arrows indicate RF chip location in each design.

#### Single-Sided Wire Antenna Example

Shows single-layer PCB with PAN211x and wire antenna implementation suitable for remote controls and simple applications.

#### Double-Sided PIFA Antenna Module Example

**Top Layer:**
- PAN211x chip placement
- PIFA antenna structure
- Component placement
- RF matching network

**Bottom Layer:**
- Ground plane (solid, not under antenna)
- Via connections
- Additional components

---

## Design Best Practices Summary

### Critical Points

1. **Power Supply**
   - Clean, stable power with <±100mV ripple
   - Adequate decoupling capacitors close to IC
   - 2× peak current capability

2. **Crystal Oscillator**
   - Short, wide traces without vias
   - Isolated from RF circuits
   - Complete ground shielding

3. **RF Path**
   - Minimize trace length (<2mm ANT to matching)
   - 50Ω controlled impedance
   - No vias on RF traces
   - Solid ground planes on both sides

4. **Antenna**
   - Adequate clearance (≥1mm from ground, ≥3cm from metal objects)
   - No ground plane under antenna on back layer
   - Pi-matching network for regulatory compliance

5. **ESD Protection**
   - TVS devices on exposed interfaces
   - Proper grounding and shielding
   - Avoid right-angle traces

---

## Document Information

**Copyright © 2025 Panchip Microelectronics Co., Ltd.**

**Contact Information:**
- Company: Shanghai Panchip Microelectronics Co., Ltd.
- Address: Room 302, Building D, No. 666 Shengxia Road, Zhangjiang Hi-Tech Park, Shanghai, China
- Phone: 021-50802371
- Website: http://www.panchip.com

**Revision History:**

| Version | Date | Description |
|---------|------|-------------|
| V1.0 | 2024.11 | Initial version created |
| V1.1 | 2025.02.10 | Optimized descriptions in some sections |
| V1.2 | 2025.07.24 | Adjusted chapter organization |

---

**DISCLAIMER**

Due to version upgrades or other reasons, the content of this document will be updated from time to time. Unless otherwise agreed, the content of this document is only used as a guide. All statements, information and suggestions in this document do not constitute any express or implied warranty.

The full or partial products, services or features described in this document may not be within your purchase or use scope. Unless otherwise agreed in the contract, Panchip Microelectronics Co., Ltd. makes no express or implied statements or guarantees regarding the content of this document.
