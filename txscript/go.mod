module github.com/monetarium/monetarium-node/txscript

go 1.23

toolchain go1.23.4

require (
	github.com/dchest/siphash v1.2.3
	github.com/decred/base58 v1.0.5
	github.com/monetarium/monetarium-node/chaincfg/chainhash v1.0.10
	github.com/monetarium/monetarium-node/chaincfg v1.0.10
	github.com/monetarium/monetarium-node/cointype v1.0.10
	github.com/monetarium/monetarium-node/crypto/blake256 v1.0.10
	github.com/monetarium/monetarium-node/crypto/ripemd160 v1.0.10
	github.com/monetarium/monetarium-node/dcrec v1.0.10
	github.com/monetarium/monetarium-node/dcrec/edwards v1.0.10
	github.com/monetarium/monetarium-node/dcrec/secp256k1 v1.0.10
	github.com/monetarium/monetarium-node/wire v1.0.10
	github.com/decred/slog v1.2.0
)

require (
	github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	lukechampine.com/blake3 v1.3.0 // indirect
)
