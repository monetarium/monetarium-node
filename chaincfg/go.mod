module github.com/monetarium/node/chaincfg

go 1.23

toolchain go1.23.4

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/monetarium/node/chaincfg/chainhash v1.0.0
	github.com/monetarium/node/cointype v1.0.0
	github.com/monetarium/node/dcrec/secp256k1 v1.0.0
	github.com/monetarium/node/wire v1.0.0
)

require (
	github.com/monetarium/node/crypto/blake256 v1.0.0 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	lukechampine.com/blake3 v1.3.0 // indirect
)

// Use local modules instead of external versions
replace github.com/monetarium/node/cointype => ../cointype

replace github.com/monetarium/node/dcrutil => ../dcrutil

replace github.com/monetarium/node/wire => ../wire

replace github.com/monetarium/node/dcrec/secp256k1 => ../dcrec/secp256k1
