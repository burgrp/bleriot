// Module link defines the COBS-framed serial protocol spoken between the host
// hub and the MCU "dumb radio modem" (UART now, USB-CDC later).
//
// It is a standalone, dependency-free module so the exact same source compiles
// unchanged into both the host hub and the TinyGo node firmware, single-sourcing
// the on-wire framing. It has no external dependencies.
module link

go 1.25.2
