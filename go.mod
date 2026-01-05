module github.com/monetarium/node

go 1.23

toolchain go1.23.4

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/decred/base58 v1.0.5
	github.com/decred/dcrtest/dcrdtest v1.0.1-0.20240404170936-a2529e936df1
	github.com/decred/go-socks v1.1.0
	github.com/decred/slog v1.2.0
	github.com/gorilla/websocket v1.5.1
	github.com/jessevdk/go-flags v1.5.0
	github.com/jrick/bitset v1.0.0
	github.com/jrick/logrotate v1.0.0
	github.com/monetarium/node/addrmgr v0.0.0
	github.com/monetarium/node/bech32 v0.0.0
	github.com/monetarium/node/blockchain v0.0.0
	github.com/monetarium/node/blockchain/stake v0.0.0
	github.com/monetarium/node/blockchain/standalone v0.0.0
	github.com/monetarium/node/certgen v0.0.0
	github.com/monetarium/node/chaincfg v0.0.0
	github.com/monetarium/node/chaincfg/chainhash v0.0.0
	github.com/monetarium/node/cointype v0.0.0
	github.com/monetarium/node/connmgr v0.0.0
	github.com/monetarium/node/container/apbf v0.0.0
	github.com/monetarium/node/container/lru v0.0.0
	github.com/monetarium/node/crypto/blake256 v0.0.0
	github.com/monetarium/node/crypto/rand v0.0.0
	github.com/monetarium/node/crypto/ripemd160 v0.0.0
	github.com/monetarium/node/database v0.0.0
	github.com/monetarium/node/dcrec v0.0.0
	github.com/monetarium/node/dcrec/secp256k1 v0.0.0
	github.com/monetarium/node/dcrjson v0.0.0
	github.com/monetarium/node/dcrutil v0.0.0
	github.com/monetarium/node/gcs v0.0.0
	github.com/monetarium/node/math/uint256 v0.0.0
	github.com/monetarium/node/mixing v0.0.0
	github.com/monetarium/node/peer v0.0.0
	github.com/monetarium/node/rpc/jsonrpc/types v0.0.0
	github.com/monetarium/node/rpcclient v0.0.0
	github.com/monetarium/node/txscript v0.0.0
	github.com/monetarium/node/wire v0.0.0
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
	github.com/decred/dcrd v1.8.0 // indirect
	github.com/decred/dcrd/addrmgr/v2 v2.0.2 // indirect
	github.com/decred/dcrd/bech32 v1.1.3 // indirect
	github.com/decred/dcrd/blockchain/stake/v5 v5.0.0 // indirect
	github.com/decred/dcrd/blockchain/standalone/v2 v2.2.0 // indirect
	github.com/decred/dcrd/certgen v1.1.2 // indirect
	github.com/decred/dcrd/chaincfg/chainhash v1.0.4 // indirect
	github.com/decred/dcrd/chaincfg/v3 v3.2.0 // indirect
	github.com/decred/dcrd/connmgr/v3 v3.1.1 // indirect
	github.com/decred/dcrd/container/apbf v1.0.1 // indirect
	github.com/decred/dcrd/crypto/blake256 v1.0.1 // indirect
	github.com/decred/dcrd/crypto/ripemd160 v1.0.2 // indirect
	github.com/decred/dcrd/database/v3 v3.0.1 // indirect
	github.com/decred/dcrd/dcrec v1.0.1 // indirect
	github.com/decred/dcrd/dcrec/edwards/v2 v2.0.3 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.2.0 // indirect
	github.com/decred/dcrd/dcrjson/v4 v4.0.1 // indirect
	github.com/decred/dcrd/dcrutil/v4 v4.0.1 // indirect
	github.com/decred/dcrd/gcs/v4 v4.0.0 // indirect
	github.com/decred/dcrd/hdkeychain/v3 v3.1.1 // indirect
	github.com/decred/dcrd/lru v1.1.2 // indirect
	github.com/decred/dcrd/math/uint256 v1.0.1 // indirect
	github.com/decred/dcrd/peer/v3 v3.0.2 // indirect
	github.com/decred/dcrd/rpc/jsonrpc/types/v4 v4.0.0 // indirect
	github.com/decred/dcrd/rpcclient/v8 v8.0.0 // indirect
	github.com/decred/dcrd/txscript/v4 v4.1.0 // indirect
	github.com/decred/dcrd/wire v1.6.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/monetarium/node/dcrec/edwards v0.0.0 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/sync v0.11.0 // indirect
	golang.org/x/text v0.22.0 // indirect
)

replace (
	github.com/monetarium/node/addrmgr => ./addrmgr
	github.com/monetarium/node/bech32 => ./bech32
	github.com/monetarium/node/blockchain => ./blockchain
	github.com/monetarium/node/blockchain/stake => ./blockchain/stake
	github.com/monetarium/node/blockchain/standalone => ./blockchain/standalone
	github.com/monetarium/node/certgen => ./certgen
	github.com/monetarium/node/chaincfg => ./chaincfg
	github.com/monetarium/node/chaincfg/chainhash => ./chaincfg/chainhash
	github.com/monetarium/node/cointype => ./cointype
	github.com/monetarium/node/connmgr => ./connmgr
	github.com/monetarium/node/container/apbf => ./container/apbf
	github.com/monetarium/node/container/lru => ./container/lru
	github.com/monetarium/node/crypto/blake256 => ./crypto/blake256
	github.com/monetarium/node/crypto/rand => ./crypto/rand
	github.com/monetarium/node/crypto/ripemd160 => ./crypto/ripemd160
	github.com/monetarium/node/database => ./database
	github.com/monetarium/node/dcrec => ./dcrec
	github.com/monetarium/node/dcrec/edwards => ./dcrec/edwards
	github.com/monetarium/node/dcrec/secp256k1 => ./dcrec/secp256k1
	github.com/monetarium/node/dcrjson => ./dcrjson
	github.com/monetarium/node/dcrutil => ./dcrutil
	github.com/monetarium/node/gcs => ./gcs
	github.com/monetarium/node/hdkeychain => ./hdkeychain
	github.com/monetarium/node/limits => ./limits
	github.com/monetarium/node/math/uint256 => ./math/uint256
	github.com/monetarium/node/mixing => ./mixing
	github.com/monetarium/node/peer => ./peer
	github.com/monetarium/node/rpc/jsonrpc/types => ./rpc/jsonrpc/types
	github.com/monetarium/node/rpcclient => ./rpcclient
	github.com/monetarium/node/txscript => ./txscript
	github.com/monetarium/node/wire => ./wire
)
