// Copyright (c) 2014-2016 The btcsuite developers
// Copyright (c) 2015-2024 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaincfg

import (
	"math/big"
	"time"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/cointype"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/wire"
)

// TestNet3Params return the network parameters for the test currency network.
// This network is sometimes simply called "testnet".
// This is the third public iteration of testnet.
func TestNet3Params() *Params {
	// testNetPowLimit is the highest proof of work value a Decred block
	// can have for the test network.  It is the value 2^232 - 1.
	testNetPowLimit := new(big.Int).Sub(new(big.Int).Lsh(bigOne, 232), bigOne)

	// testNetPowLimitBits is the test network proof of work limit in its
	// compact representation.
	//
	// Note that due to the limited precision of the compact representation,
	// this is not exactly equal to the pow limit.  It is the value:
	//
	// 0x000000ffff000000000000000000000000000000000000000000000000000000
	const testNetPowLimitBits = 0x1e00ffff // 503382015

	// genesisBlock defines the genesis block of the block chain which serves as
	// the public transaction ledger for the test network (version 3).
	genesisBlock := wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:   6,
			PrevBlock: chainhash.Hash{},
			// MerkleRoot: Calculated below.
			Timestamp:    time.Unix(1533513600, 0), // 2018-08-06 00:00:00 +0000 UTC
			Bits:         testNetPowLimitBits,      // Difficulty 1
			SBits:        20000000,
			Nonce:        0x18aea41a,
			StakeVersion: 6,
		},
		Transactions: []*wire.MsgTx{{
			SerType: wire.TxSerializeFull,
			Version: 1,
			TxIn: []*wire.TxIn{{
				PreviousOutPoint: wire.OutPoint{
					Hash:  chainhash.Hash{},
					Index: 0xffffffff,
				},
				SignatureScript: hexDecode("0000"),
				Sequence:        0xffffffff,
			}},
			TxOut: []*wire.TxOut{{
				Value:    0x00000000,
				CoinType: cointype.CoinTypeVAR,
				Version:  0x0000,
				PkScript: hexDecode("801679e98561ada96caec2949a5d41c4cab3851e" +
					"b740d951c10ecbcf265c1fd9"),
			}},
			LockTime: 0,
			Expiry:   0,
		}},
	}
	// NOTE: This really should be TxHashFull, but it was defined incorrectly.
	//
	// Since the field is not used in any validation code, it does not have any
	// adverse effects, but correcting it would result in changing the block
	// hash which would invalidate the entire test network.  The next test
	// network should set the value properly.
	genesisBlock.Header.MerkleRoot = genesisBlock.Transactions[0].TxHash()

	return &Params{
		Name:        "testnet3",
		Net:         wire.TestNet3,
		DefaultPort: "19108",
		// DNSSeeds disabled - Monetarium testnet uses manual peer connections
		DNSSeeds: []DNSSeed{},

		// Chain parameters.
		//
		// Note that the minimum difficulty reduction parameter only applies up
		// to and including block height 962927.
		GenesisBlock:         &genesisBlock,
		GenesisHash:          genesisBlock.BlockHash(),
		PowLimit:             testNetPowLimit,
		PowLimitBits:         testNetPowLimitBits,
		ReduceMinDifficulty:  true,
		MinDiffReductionTime: time.Minute * 10, // ~99.3% chance to be mined before reduction
		GenerateSupported:    true,
		MaximumBlockSizes:    []int{1310720},
		MaxTxSize:            1000000,
		TargetTimePerBlock:   time.Minute * 2,

		// Version 1 difficulty algorithm (EMA + BLAKE256) parameters.
		WorkDiffAlpha:            1,
		WorkDiffWindowSize:       144,
		WorkDiffWindows:          20,
		TargetTimespan:           time.Minute * 2 * 144, // TimePerBlock * WindowSize
		RetargetAdjustmentFactor: 4,

		// Version 2 difficulty algorithm (ASERT + BLAKE3) parameters.
		WorkDiffV2Blake3StartBits: testNetPowLimitBits,
		WorkDiffV2HalfLifeSecs:    720, // 6 * TimePerBlock (12 minutes)

		// Subsidy parameters.
		BaseSubsidy:              6400000000, // 64 VAR per block (same as mainnet)
		MulSubsidy:               1,          // Numerator for halving (1/2)
		DivSubsidy:               2,          // Denominator for halving (1/2)
		SubsidyReductionInterval: 52560,      // ~6 months for testnet (faster than mainnet)
		WorkRewardProportion:     6,
		WorkRewardProportionV2:   5,
		StakeRewardProportion:    3,
		StakeRewardProportionV2:  5,
		BlockTaxProportion:       0,

		// AssumeValid is the hash of a block that has been externally verified
		// to be valid.  It allows several validation checks to be skipped for
		// blocks that are both an ancestor of the assumed valid block and an
		// ancestor of the best header.  It is also used to determine the old
		// forks rejection checkpoint.  This is intended to be updated
		// periodically with new releases.
		//
		// Block 88d61d7609c06c8e171f050789f6649d21525a144b820026f7b396476a05a44b
		// Height: 1377455
		AssumeValid: *newHashFromStr("88d61d7609c06c8e171f050789f6649d21525a144b820026f7b396476a05a44b"),

		// MinKnownChainWork is the minimum amount of known total work for the
		// chain at a given point in time.  This is intended to be updated
		// periodically with new releases.
		//
		// Block 50f244d269a61de438a9075f7f5477a785f3f2060d2c7127f000093176a386fa
		// Height: 1387535
		MinKnownChainWork: hexToBigInt("000000000000000000000000000000000000000000000000f376ddb1ab3a5a2e"),

		// Consensus rule change deployments.
		//
		// The miner confirmation window is defined as:
		//   target proof of work timespan / target proof of work spacing
		RuleChangeActivationQuorum:     2520, // 10 % of RuleChangeActivationInterval * TicketsPerBlock
		RuleChangeActivationMultiplier: 3,    // 75%
		RuleChangeActivationDivisor:    4,
		RuleChangeActivationInterval:   5040, // 1 week
		Deployments: map[uint32][]ConsensusDeployment{
			5: {{
				Vote: Vote{
					Id:          VoteIDSDiffAlgorithm,
					Description: "Change stake difficulty algorithm as defined in DCP0001",
					Mask:        0x0006, // Bits 1 and 2
					Choices: []Choice{{
						Id:          "abstain",
						Description: "abstain voting for change",
						Bits:        0x0000,
						IsAbstain:   true,
						IsNo:        false,
					}, {
						Id:          "no",
						Description: "keep the existing algorithm",
						Bits:        0x0002, // Bit 1
						IsAbstain:   false,
						IsNo:        true,
					}, {
						Id:          "yes",
						Description: "change to the new algorithm",
						Bits:        0x0004, // Bit 2
						IsAbstain:   false,
						IsNo:        false,
					}},
				},
				ForcedChoiceID: "yes",
				StartTime:      1493164800, // Apr 26th, 2017
				ExpireTime:     1524700800, // Apr 26th, 2018
			}},
			6: {{
				Vote: Vote{
					Id:          VoteIDLNFeatures,
					Description: "Enable features defined in DCP0002 and DCP0003 necessary to support Lightning Network (LN)",
					Mask:        0x0006, // Bits 1 and 2
					Choices: []Choice{{
						Id:          "abstain",
						Description: "abstain voting for change",
						Bits:        0x0000,
						IsAbstain:   true,
						IsNo:        false,
					}, {
						Id:          "no",
						Description: "keep the existing consensus rules",
						Bits:        0x0002, // Bit 1
						IsAbstain:   false,
						IsNo:        true,
					}, {
						Id:          "yes",
						Description: "change to the new consensus rules",
						Bits:        0x0004, // Bit 2
						IsAbstain:   false,
						IsNo:        false,
					}},
				},
				ForcedChoiceID: "yes",
				StartTime:      1505260800, // Sep 13th, 2017
				ExpireTime:     1536796800, // Sep 13th, 2018
			}},
			7: {{
				Vote: Vote{
					Id:          VoteIDFixLNSeqLocks,
					Description: "Modify sequence lock handling as defined in DCP0004",
					Mask:        0x0006, // Bits 1 and 2
					Choices: []Choice{{
						Id:          "abstain",
						Description: "abstain voting for change",
						Bits:        0x0000,
						IsAbstain:   true,
						IsNo:        false,
					}, {
						Id:          "no",
						Description: "keep the existing consensus rules",
						Bits:        0x0002, // Bit 1
						IsAbstain:   false,
						IsNo:        true,
					}, {
						Id:          "yes",
						Description: "change to the new consensus rules",
						Bits:        0x0004, // Bit 2
						IsAbstain:   false,
						IsNo:        false,
					}},
				},
				StartTime:  1548633600, // Jan 28th, 2019
				ExpireTime: 1580169600, // Jan 28th, 2020
			}},
			8: {{
				Vote: Vote{
					Id:          VoteIDHeaderCommitments,
					Description: "Enable header commitments as defined in DCP0005",
					Mask:        0x0006, // Bits 1 and 2
					Choices: []Choice{{
						Id:          "abstain",
						Description: "abstain voting for change",
						Bits:        0x0000,
						IsAbstain:   true,
						IsNo:        false,
					}, {
						Id:          "no",
						Description: "keep the existing consensus rules",
						Bits:        0x0002, // Bit 1
						IsAbstain:   false,
						IsNo:        true,
					}, {
						Id:          "yes",
						Description: "change to the new consensus rules",
						Bits:        0x0004, // Bit 2
						IsAbstain:   false,
						IsNo:        false,
					}},
				},
				StartTime:  1567641600, // Sep 5th, 2019
				ExpireTime: 1599264000, // Sep 5th, 2020
			}},
			9: {{
				Vote: Vote{
					Id:          VoteIDTreasury,
					Description: "Enable decentralized Treasury opcodes as defined in DCP0006",
					Mask:        0x0006, // Bits 1 and 2
					Choices: []Choice{{
						Id:          "abstain",
						Description: "abstain voting for change",
						Bits:        0x0000,
						IsAbstain:   true,
						IsNo:        false,
					}, {
						Id:          "no",
						Description: "keep the existing consensus rules",
						Bits:        0x0002, // Bit 1
						IsAbstain:   false,
						IsNo:        true,
					}, {
						Id:          "yes",
						Description: "change to the new consensus rules",
						Bits:        0x0004, // Bit 2
						IsAbstain:   false,
						IsNo:        false,
					}},
				},
				StartTime:  1596240000, // Aug 1st, 2020
				ExpireTime: 1627776000, // Aug 1st, 2021
			}},
			10: {{
				Vote: Vote{
					Id:          VoteIDRevertTreasuryPolicy,
					Description: "Change maximum treasury expenditure policy as defined in DCP0007",
					Mask:        0x0006, // Bits 1 and 2
					Choices: []Choice{{
						Id:          "abstain",
						Description: "abstain voting for change",
						Bits:        0x0000,
						IsAbstain:   true,
						IsNo:        false,
					}, {
						Id:          "no",
						Description: "keep the existing consensus rules",
						Bits:        0x0002, // Bit 1
						IsAbstain:   false,
						IsNo:        true,
					}, {
						Id:          "yes",
						Description: "change to the new consensus rules",
						Bits:        0x0004, // Bit 2
						IsAbstain:   false,
						IsNo:        false,
					}},
				},
				StartTime:  1631750400, // Sep 16th, 2021
				ExpireTime: 1694822400, // Sep 16th, 2023
			}, {
				Vote: Vote{
					Id:          VoteIDExplicitVersionUpgrades,
					Description: "Enable explicit version upgrades as defined in DCP0008",
					Mask:        0x0018, // Bits 3 and 4
					Choices: []Choice{{
						Id:          "abstain",
						Description: "abstain from voting",
						Bits:        0x0000,
						IsAbstain:   true,
						IsNo:        false,
					}, {
						Id:          "no",
						Description: "keep the existing consensus rules",
						Bits:        0x0008, // Bit 3
						IsAbstain:   false,
						IsNo:        true,
					}, {
						Id:          "yes",
						Description: "change to the new consensus rules",
						Bits:        0x0010, // Bit 4
						IsAbstain:   false,
						IsNo:        false,
					}},
				},
				StartTime:  1631750400, // Sep 16th, 2021
				ExpireTime: 1694822400, // Sep 16th, 2023
			}, {
				Vote: Vote{
					Id:          VoteIDAutoRevocations,
					Description: "Enable automatic ticket revocations as defined in DCP0009",
					Mask:        0x0060, // Bits 5 and 6
					Choices: []Choice{{
						Id:          "abstain",
						Description: "abstain voting for change",
						Bits:        0x0000,
						IsAbstain:   true,
						IsNo:        false,
					}, {
						Id:          "no",
						Description: "keep the existing consensus rules",
						Bits:        0x0020, // Bit 5
						IsAbstain:   false,
						IsNo:        true,
					}, {
						Id:          "yes",
						Description: "change to the new consensus rules",
						Bits:        0x0040, // Bit 6
						IsAbstain:   false,
						IsNo:        false,
					}},
				},
				StartTime:  1631750400, // Sep 16th, 2021
				ExpireTime: 1694822400, // Sep 16th, 2023
			}, {
				Vote: Vote{
					Id:          VoteIDChangeSubsidySplit,
					Description: "Change block reward subsidy split to 10/80/10 as defined in DCP0010",
					Mask:        0x0180, // Bits 7 and 8
					Choices: []Choice{{
						Id:          "abstain",
						Description: "abstain from voting",
						Bits:        0x0000,
						IsAbstain:   true,
						IsNo:        false,
					}, {
						Id:          "no",
						Description: "keep the existing consensus rules",
						Bits:        0x0080, // Bit 7
						IsAbstain:   false,
						IsNo:        true,
					}, {
						Id:          "yes",
						Description: "change to the new consensus rules",
						Bits:        0x0100, // Bit 8
						IsAbstain:   false,
						IsNo:        false,
					}},
				},
				StartTime:  1631750400, // Sep 16th, 2021
				ExpireTime: 1694822400, // Sep 16th, 2023
			}},
			11: {{
				Vote: Vote{
					Id:          VoteIDBlake3Pow,
					Description: "Change proof of work hashing algorithm to BLAKE3 as defined in DCP0011",
					Mask:        0x0006, // Bits 1 and 2
					Choices: []Choice{{
						Id:          "abstain",
						Description: "abstain voting for change",
						Bits:        0x0000,
						IsAbstain:   true,
						IsNo:        false,
					}, {
						Id:          "no",
						Description: "keep the existing consensus rules",
						Bits:        0x0002, // Bit 1
						IsAbstain:   false,
						IsNo:        true,
					}, {
						Id:          "yes",
						Description: "change to the new consensus rules",
						Bits:        0x0004, // Bit 2
						IsAbstain:   false,
						IsNo:        false,
					}},
				},
				StartTime:  1682294400, // Apr 24th, 2023
				ExpireTime: 1745452800, // Apr 24th, 2025
			}, {
				Vote: Vote{
					Id:          VoteIDChangeSubsidySplitR2,
					Description: "Change block reward subsidy split to 1/89/10 as defined in DCP0012",
					Mask:        0x0060, // Bits 5 and 6
					Choices: []Choice{{
						Id:          "abstain",
						Description: "abstain voting for change",
						Bits:        0x0000,
						IsAbstain:   true,
						IsNo:        false,
					}, {
						Id:          "no",
						Description: "keep the existing consensus rules",
						Bits:        0x0020, // Bit 5
						IsAbstain:   false,
						IsNo:        true,
					}, {
						Id:          "yes",
						Description: "change to the new consensus rules",
						Bits:        0x0040, // Bit 6
						IsAbstain:   false,
						IsNo:        false,
					}},
				},
				StartTime:  1682294400, // Apr 24th, 2023
				ExpireTime: 1745452800, // Apr 24th, 2025
			}},
		},

		// Enforce current block version once majority of the network has
		// upgraded.
		// 51% (51 / 100)
		// Reject previous block versions once a majority of the network has
		// upgraded.
		// 75% (75 / 100)
		BlockEnforceNumRequired: 51,
		BlockRejectNumRequired:  75,
		BlockUpgradeNumToCheck:  100,

		// AcceptNonStdTxs is a mempool param to either accept and relay non
		// standard txs to the network or reject them
		AcceptNonStdTxs: true,

		// Address encoding magics
		NetworkAddressPrefix: "T",
		PubKeyAddrID:         [2]byte{0x28, 0xf7}, // starts with Tk
		PubKeyHashAddrID:     [2]byte{0x0f, 0x21}, // starts with Ts
		PKHEdwardsAddrID:     [2]byte{0x0f, 0x01}, // starts with Te
		PKHSchnorrAddrID:     [2]byte{0x0e, 0xe3}, // starts with TS
		ScriptHashAddrID:     [2]byte{0x0e, 0xfc}, // starts with Tc
		PrivateKeyID:         [2]byte{0x23, 0x0e}, // starts with Pt

		// BIP32 hierarchical deterministic extended key magics
		HDPrivateKeyID: [4]byte{0x04, 0x35, 0x83, 0x97}, // starts with tprv
		HDPublicKeyID:  [4]byte{0x04, 0x35, 0x87, 0xd1}, // starts with tpub

		// BIP44 coin type used in the hierarchical deterministic path for
		// address generation.
		SLIP0044CoinType: 1,  // SLIP0044, Testnet (all coins)
		LegacyCoinType:   11, // for backwards compatibility

		// Decred PoS parameters
		MinimumStakeDiff:        20000000, // 0.2 Coin
		TicketPoolSize:          1024,
		TicketsPerBlock:         5,
		TicketMaturity:          16,
		TicketExpiry:            6144, // 6*TicketPoolSize
		CoinbaseMaturity:        16,
		SStxChangeMaturity:      1,
		TicketPoolSizeWeight:    4,
		StakeDiffAlpha:          1,
		StakeDiffWindowSize:     144,
		StakeDiffWindows:        20,
		StakeVersionInterval:    144 * 2 * 7, // ~1 week
		MaxFreshStakePerBlock:   20,          // 4*TicketsPerBlock
		StakeEnabledHeight:      16 + 16,     // CoinbaseMaturity + TicketMaturity
		StakeValidationHeight:   768,         // Arbitrary
		StakeBaseSigScript:      []byte{0x00, 0x00},
		StakeMajorityMultiplier: 3,
		StakeMajorityDivisor:    4,

		// Monetarium has no treasury (BlockTaxProportion = 0)
		OrganizationPkScript:        nil,
		OrganizationPkScriptVersion: 0,
		BlockOneLedger:              nil, // Monetarium has no premine

		// Sanctioned Politeia keys.
		PiKeys: [][]byte{
			hexDecode("03beca9bbd227ca6bb5a58e03a36ba2b52fff09093bd7a50aee1193bccd257fb8a"),
			hexDecode("03e647c014f55265da506781f0b2d67674c35cb59b873d9926d483c4ced9a7bbd3"),
		},

		// ~2 hours for tspend inclusion
		TreasuryVoteInterval: 60,

		// ~4.8 hours for short circuit approval
		TreasuryVoteIntervalMultiplier: 4,

		// ~1 day policy window
		TreasuryExpenditureWindow: 4,

		// ~6 day policy window check
		TreasuryExpenditurePolicy: 3,

		// 10000 dcr/tew as expense bootstrap
		TreasuryExpenditureBootstrap: 10000 * 1e8,

		TreasuryVoteQuorumMultiplier:   1, // 20% quorum required
		TreasuryVoteQuorumDivisor:      5,
		TreasuryVoteRequiredMultiplier: 3, // 60% yes votes required
		TreasuryVoteRequiredDivisor:    5,

		// HTTP seeders disabled - Monetarium testnet uses manual peer connections
		seeders: []string{},

		// SKA (Skarb) dual-coin system parameters for testnet
		// 50 atoms/KB ensures ~10 atoms fee for typical 200-byte tx
		SKAMinRelayTxFee: 50,

		// SKA coin type configurations (fast testing values)
		SKACoins: map[cointype.CoinType]*SKACoinConfig{
			1: {
				CoinType:       1,
				Name:           "Skarb-1",
				Symbol:         "SKA-1",
				MaxSupply:      10e6 * 1e8, // 10 million SKA-1
				EmissionHeight: 64,         // Fast emission for testing
				EmissionWindow: 100,        // 100 block window for testing
				Active:         true,
				Description:    "Primary asset-backed SKA coin type for testnet",
				EmissionAddresses: []string{
					"TsPlaceholderAddressForTestnetSKA1Emission", // REPLACE with real testnet address
				},
				EmissionAmounts: []int64{
					10e6 * 1e8,
				},
				// SECURITY NOTE: This is a placeholder key for development ONLY
				EmissionKey: mustParseHexPubKeyTestnet("02f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9"),
			},
			2: {
				CoinType:       2,
				Name:           "Skarb-2",
				Symbol:         "SKA-2",
				MaxSupply:      5e6 * 1e8, // 5 million SKA-2
				EmissionHeight: 64,        // Fast emission for testing
				EmissionWindow: 100,       // 100 block window for testing
				Active:         true,      // Active on testnet for testing
				Description:    "Secondary SKA coin type for testnet testing",
				EmissionAddresses: []string{
					"TsPlaceholderAddressForTestnetSKA2Emission", // REPLACE with real testnet address
				},
				EmissionAmounts: []int64{
					5e6 * 1e8,
				},
				// SECURITY NOTE: This is a placeholder key for development ONLY
				EmissionKey: mustParseHexPubKeyTestnet("0316e57ce5fdb617dc192576d9c860f57e7e7a95592aa32e25941731a2eb2c57d6"),
			},
		},

		// Initial SKA types to activate at network genesis
		InitialSKATypes: []cointype.CoinType{1},
	}
}

// mustParseHexPubKeyTestnet parses a hex-encoded public key for testnet.
// SECURITY WARNING: These are placeholder keys - production must use secure key generation.
func mustParseHexPubKeyTestnet(hexStr string) *secp256k1.PublicKey {
	keyBytes := mustParseHex(hexStr)
	pubKey, err := secp256k1.ParsePubKey(keyBytes)
	if err != nil {
		panic("failed to parse public key: " + err.Error())
	}
	return pubKey
}
