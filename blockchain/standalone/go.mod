module github.com/monetarium/node/blockchain/standalone

go 1.23

toolchain go1.23.4

require (
	github.com/monetarium/node/chaincfg/chainhash v1.0.0
	github.com/monetarium/node/cointype v1.0.0
	github.com/monetarium/node/wire v1.0.0
)

require (
	github.com/monetarium/node/crypto/blake256 v1.0.0 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	lukechampine.com/blake3 v1.3.0 // indirect
)

// Use local packages with dual-coin support
replace github.com/monetarium/node/wire => ../../wire

replace github.com/monetarium/node/cointype => ../../cointype
