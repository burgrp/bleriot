// Module protocol is the neutral, dependency-free implementation of the BleRiot
// RF wire format (PROTOCOL.md §4–§8): packet encode/decode and XTEA.
//
// It is shared by both the node firmware and the host hub
// (site), so the on-wire format is single-sourced. It has no external
// dependencies and no build tags, so it compiles for the host and under TinyGo
// alike.
module protocol

go 1.25.2
