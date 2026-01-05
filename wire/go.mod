module github.com/monetarium/node/wire

go 1.23

toolchain go1.23.4

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/monetarium/node/chaincfg/chainhash v0.0.0
	github.com/monetarium/node/cointype v0.0.0
	lukechampine.com/blake3 v1.3.0
)

require (
	github.com/monetarium/node/crypto/blake256 v0.0.0 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
)

replace github.com/monetarium/node/dcrutil => ../dcrutil

replace github.com/monetarium/node/cointype => ../cointype
