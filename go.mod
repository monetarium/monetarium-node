module github.com/monetarium/node

go 1.23.0

toolchain go1.23.4

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/decred/base58 v1.0.5
	github.com/decred/go-socks v1.1.0
	github.com/decred/slog v1.2.0
	github.com/gorilla/websocket v1.5.1
	github.com/jessevdk/go-flags v1.5.0
	github.com/jrick/bitset v1.0.0
	github.com/jrick/logrotate v1.0.0
	github.com/monetarium/node/addrmgr v1.0.3
	github.com/monetarium/node/bech32 v1.0.3
	github.com/monetarium/node/blockchain v1.0.3
	github.com/monetarium/node/blockchain/stake v1.0.3
	github.com/monetarium/node/blockchain/standalone v1.0.3
	github.com/monetarium/node/certgen v1.0.3
	github.com/monetarium/node/chaincfg v1.0.3
	github.com/monetarium/node/chaincfg/chainhash v1.0.3
	github.com/monetarium/node/cointype v1.0.3
	github.com/monetarium/node/connmgr v1.0.3
	github.com/monetarium/node/container/apbf v1.0.3
	github.com/monetarium/node/container/lru v1.0.3
	github.com/monetarium/node/crypto/blake256 v1.0.3
	github.com/monetarium/node/crypto/rand v1.0.3
	github.com/monetarium/node/crypto/ripemd160 v1.0.3
	github.com/monetarium/node/database v1.0.3
	github.com/monetarium/node/dcrec v1.0.3
	github.com/monetarium/node/dcrec/secp256k1 v1.0.3
	github.com/monetarium/node/dcrjson v1.0.3
	github.com/monetarium/node/dcrutil v1.0.3
	github.com/monetarium/node/gcs v1.0.3
	github.com/monetarium/node/math/uint256 v1.0.3
	github.com/monetarium/node/mixing v1.0.3
	github.com/monetarium/node/peer v1.0.3
	github.com/monetarium/node/rpc/jsonrpc/types v1.0.3
	github.com/monetarium/node/rpcclient v1.0.3
	github.com/monetarium/node/txscript v1.0.3
	github.com/monetarium/node/wire v1.0.3
	github.com/monetarium/test/dcrdtest v1.0.1
	github.com/syndtr/goleveldb v1.0.1-0.20210819022825-2ae1ddf74ef7
	golang.org/x/net v0.28.0
	golang.org/x/sys v0.30.0
	golang.org/x/term v0.29.0
	lukechampine.com/blake3 v1.3.0
)

require (
	decred.org/cspp/v2 v2.4.0 // indirect
	github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412 // indirect
	github.com/companyzero/sntrup4591761 v0.0.0-20220309191932-9e0f3af2f07a // indirect
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/monetarium/node/dcrec/edwards v1.0.3 // indirect
	github.com/monetarium/node/hdkeychain v1.0.3 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/sync v0.11.0 // indirect
	golang.org/x/text v0.22.0 // indirect
)
