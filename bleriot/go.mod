// Module bleriot is the neutral, dependency-free implementation of the BleRiot
// wire protocol (PROTOCOL.md §4–§8): packet encode/decode and XTEA.
//
// It is shared by both the node firmware (test-fw) and the host hub, so the
// on-wire format is single-sourced. It has no external dependencies and no
// build tags, so it compiles for the host and under TinyGo alike.
module bleriot

go 1.25.2
