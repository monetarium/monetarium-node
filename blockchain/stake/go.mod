module github.com/monetarium/node/blockchain/stake

go 1.23

toolchain go1.23.4

require (
	github.com/monetarium/node/blockchain/standalone v0.0.0
	github.com/monetarium/node/chaincfg/chainhash v0.0.0
	github.com/monetarium/node/chaincfg v0.0.0
	github.com/monetarium/node/cointype v0.0.0
	github.com/monetarium/node/database v0.0.0
	github.com/monetarium/node/dcrec/secp256k1 v0.0.0
	github.com/monetarium/node/dcrutil v0.0.0
	github.com/monetarium/node/txscript v0.0.0
	github.com/monetarium/node/wire v0.0.0
	github.com/decred/slog v1.2.0
)

require (
	github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412 // indirect
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/decred/base58 v1.0.5 // indirect
	github.com/monetarium/node/crypto/blake256 v0.0.0 // indirect
	github.com/monetarium/node/crypto/ripemd160 v0.0.0 // indirect
	github.com/monetarium/node/dcrec v0.0.0 // indirect
	github.com/monetarium/node/dcrec/edwards v0.0.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20210819022825-2ae1ddf74ef7 // indirect
	lukechampine.com/blake3 v1.3.0 // indirect
)

replace github.com/monetarium/node/blockchain/standalone => ../standalone

replace github.com/monetarium/node/cointype => ../../cointype

replace github.com/monetarium/node/wire => ../../wire
