// Module hub/link defines the COBS-framed serial protocol spoken between the
// host hub and the MCU "dumb radio modem" (UART now, USB-CDC later).
//
// It is a standalone, dependency-free module so the exact same source compiles
// unchanged into both the host hub (hub/host) and the TinyGo modem firmware
// (hub/fw), single-sourcing the on-wire link framing.
module hub/link

go 1.25.2

