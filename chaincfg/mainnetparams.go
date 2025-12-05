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

// MainNetParams returns the network parameters for the main Decred network.
func MainNetParams() *Params {
	// mainPowLimit is the highest proof of work value a Decred block can have
	// for the main network.  It is the value 2^224 - 1.
	mainPowLimit := new(big.Int).Sub(new(big.Int).Lsh(bigOne, 224), bigOne)

	// mainNetPowLimitBits is the main network proof of work limit in its
	// compact representation.
	//
	// Note that due to the limited precision of the compact representation,
	// this is not exactly equal to the pow limit.  It is the value:
	//
	// 0x00000000ffff0000000000000000000000000000000000000000000000000000
	const mainPowLimitBits = 0x1d00ffff // 486604799

	// genesisBlock defines the genesis block of the block chain which serves as
	// the public transaction ledger for the main network.
	//
	// The genesis block for Decred mainnet, testnet, and simnet are not
	// evaluated for proof of work. The only values that are ever used elsewhere
	// in the blockchain from it are:
	// (1) The genesis block hash is used as the PrevBlock.
	// (2) The difficulty starts off at the value given by Bits.
	// (3) The stake difficulty starts off at the value given by SBits.
	// (4) The timestamp, which guides when blocks can be built on top of it
	//      and what the initial difficulty calculations come out to be.
	//
	// The genesis block is valid by definition and none of the fields within it
	// are validated for correctness.
	genesisBlock := wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:   1,
			PrevBlock: chainhash.Hash{}, // All zero.
			// MerkleRoot: Calculated below.
			StakeRoot:    chainhash.Hash{},
			Timestamp:    time.Unix(1760649600, 0), // Thu, 16 Oct 2025 00:00:00 GMT
			Bits:         0x1d00ffff,               // Difficulty 1 - CPU mining friendly for bootstrap
			SBits:        2 * 1e8,                  // 2 Coin
			Nonce:        0x00000000,
			StakeVersion: 0,
		},
		Transactions: []*wire.MsgTx{{
			SerType: wire.TxSerializeFull,
			Version: 1,
			TxIn: []*wire.TxIn{{
				// Fully null.
				PreviousOutPoint: wire.OutPoint{
					Hash:  chainhash.Hash{},
					Index: 0xffffffff,
					Tree:  0,
				},
				SignatureScript: hexDecode("0000"),
				Sequence:        0xffffffff,
				BlockHeight:     wire.NullBlockHeight,
				BlockIndex:      wire.NullBlockIndex,
				ValueIn:         wire.NullValueIn,
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
	genesisBlock.Header.MerkleRoot = genesisBlock.Transactions[0].TxHashFull()

	return &Params{
		Name:        "mainnet",
		Net:         wire.MainNet,
		DefaultPort: "9108",
		// DNSSeeds disabled - Monetarium uses manual peer connections for bootstrap
		DNSSeeds: []DNSSeed{},

		// Chain parameters
		GenesisBlock:         &genesisBlock,
		GenesisHash:          genesisBlock.BlockHash(),
		PowLimit:             mainPowLimit,
		PowLimitBits:         mainPowLimitBits,
		ReduceMinDifficulty:  false,
		MinDiffReductionTime: 0,    // Does not apply since ReduceMinDifficulty false
		GenerateSupported:    true, // Enable CPU mining for Monetarium mainnet bootstrap
		MaximumBlockSizes:    []int{393216},
		MaxTxSize:            393216,
		TargetTimePerBlock:   time.Minute * 5,

		// Version 1 difficulty algorithm (EMA + BLAKE256) parameters.
		WorkDiffAlpha:            1,
		WorkDiffWindowSize:       144,
		WorkDiffWindows:          20,
		TargetTimespan:           time.Minute * 5 * 144, // TimePerBlock * WindowSize
		RetargetAdjustmentFactor: 4,

		// Version 2 difficulty algorithm (ASERT + BLAKE3) parameters.
		WorkDiffV2Blake3StartBits: 0x1d00ffff, // Difficulty 1 - easy CPU mining for bootstrap
		WorkDiffV2HalfLifeSecs:    43200,      // 144 * TimePerBlock (12 hours)

		// Subsidy parameters.
		BaseSubsidy:              6400000000, // 64 VAR per block
		MulSubsidy:               1,          // Numerator for halving (1/2)
		DivSubsidy:               2,          // Denominator for halving (1/2)
		SubsidyReductionInterval: 420480,     // ~4 years (420,480 blocks)
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
		// Block *newHashFromStr("f04628f2fe7fd0d33055dc326936a6af3772ec5226525bc8fca50631f3081faa")
		// Height: 865184
		AssumeValid: chainhash.Hash{},

		// MinKnownChainWork is the minimum amount of known total work for the
		// chain at a given point in time.
		//
		// Not set for Monetarium mainnet to allow bootstrap from genesis.
		// This is a new network, not a continuation of Decred's chain.
		MinKnownChainWork: nil,

		// The miner confirmation window is defined as:
		//   target proof of work timespan / target proof of work spacing
		RuleChangeActivationQuorum:     16, // 4032, // 10 % of RuleChangeActivationInterval * TicketsPerBlock
		RuleChangeActivationMultiplier: 3,  // 75%
		RuleChangeActivationDivisor:    4,
		RuleChangeActivationInterval:   32, // 2016 * 4, // 4 weeks
		Deployments: map[uint32][]ConsensusDeployment{
			4: {{
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
			}, {
				Vote: Vote{
					Id:          VoteIDLNSupport,
					Description: "Request developers begin work on Lightning Network (LN) integration",
					Mask:        0x0018, // Bits 3 and 4
					Choices: []Choice{{
						Id:          "abstain",
						Description: "abstain from voting",
						Bits:        0x0000,
						IsAbstain:   true,
						IsNo:        false,
					}, {
						Id:          "no",
						Description: "no, do not work on integrating LN support",
						Bits:        0x0008, // Bit 3
						IsAbstain:   false,
						IsNo:        true,
					}, {
						Id:          "yes",
						Description: "yes, begin work on integrating LN support",
						Bits:        0x0010, // Bit 4
						IsAbstain:   false,
						IsNo:        false,
					}},
				},
				StartTime:  1493164800, // Apr 26th, 2017
				ExpireTime: 1508976000, // Oct 26th, 2017
			}},
			5: {{
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
			6: {{
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
				ForcedChoiceID: "yes",
				StartTime:      1548633600, // Jan 28th, 2019
				ExpireTime:     1580169600, // Jan 28th, 2020
			}},
			7: {{
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
				ForcedChoiceID: "yes",
				StartTime:      1567641600, // Sep 5th, 2019
				ExpireTime:     1599264000, // Sep 5th, 2020
			}},
			8: {{
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
			9: {{
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
				ForcedChoiceID: "yes",
				StartTime:      1631750400, // Sep 16th, 2021
				ExpireTime:     1694822400, // Sep 16th, 2023
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
				ForcedChoiceID: "yes",
				StartTime:      1631750400, // Sep 16th, 2021
				ExpireTime:     1694822400, // Sep 16th, 2023
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
			10: {{
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
				ForcedChoiceID: "yes",
				StartTime:      1682294400, // Apr 24th, 2023
				ExpireTime:     1745452800, // Apr 24th, 2025
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
			}, {
				Vote: Vote{
					Id:          "activateska2",
					Description: "Activate SKA-2 coin type for transactions",
					Mask:        0x0006,
					Choices: []Choice{{
						Id:          "abstain",
						Description: "abstain from voting",
						Bits:        0x0000,
						IsAbstain:   true,
						IsNo:        false,
					}, {
						Id:          "no",
						Description: "keep SKA-2 inactive",
						Bits:        0x0002,
						IsAbstain:   false,
						IsNo:        true,
					}, {
						Id:          "yes",
						Description: "activate SKA-2 for use",
						Bits:        0x0004,
						IsAbstain:   false,
						IsNo:        false,
					}},
				},
				StartTime:  1764956763, // Dec 5th, 2025
				ExpireTime: 1765039557, // Dec 6th, 2025
			}},
		},

		// Enforce current block version once majority of the network has
		// upgraded.
		// 75% (750 / 1000)
		//
		// Reject previous block versions once a majority of the network has
		// upgraded.
		// 95% (950 / 1000)
		BlockEnforceNumRequired: 750,
		BlockRejectNumRequired:  950,
		BlockUpgradeNumToCheck:  1000,

		// AcceptNonStdTxs is a mempool param to either accept and relay non
		// standard txs to the network or reject them
		AcceptNonStdTxs: false,

		// Address encoding magics
		NetworkAddressPrefix: "M",
		PubKeyAddrID:         [2]byte{0x1f, 0xc5}, // starts with Mk
		PubKeyHashAddrID:     [2]byte{0x0b, 0xc0}, // starts with Ms
		PKHEdwardsAddrID:     [2]byte{0x0b, 0x9f}, // starts with Me
		PKHSchnorrAddrID:     [2]byte{0x0b, 0x81}, // starts with MS
		ScriptHashAddrID:     [2]byte{0x0b, 0x9a}, // starts with Mc
		PrivateKeyID:         [2]byte{0x22, 0xdc}, // starts with Pm

		// BIP32 hierarchical deterministic extended key magics
		HDPrivateKeyID: [4]byte{0x02, 0xfd, 0xa4, 0xe8}, // starts with dprv
		HDPublicKeyID:  [4]byte{0x02, 0xfd, 0xa9, 0x26}, // starts with dpub

		// BIP44 coin type used in the hierarchical deterministic path for
		// address generation.
		SLIP0044CoinType: 42, // SLIP0044, Decred
		LegacyCoinType:   20, // for backwards compatibility

		// Decred PoS parameters
		MinimumStakeDiff:        2 * 1e8, // 2 Coin
		TicketPoolSize:          8192,
		TicketsPerBlock:         5,
		TicketMaturity:          16,    // PRODUCTION: Normal mainnet value (temporary: 16)
		TicketExpiry:            40960, // 5*TicketPoolSize
		CoinbaseMaturity:        16,    // PRODUCTION: Normal mainnet value (temporary: 16)
		SStxChangeMaturity:      1,
		TicketPoolSizeWeight:    4,
		StakeDiffAlpha:          1, // Minimal
		StakeDiffWindowSize:     144,
		StakeDiffWindows:        20,
		StakeVersionInterval:    144 * 2 * 7, // ~1 week
		MaxFreshStakePerBlock:   20,          // 4*TicketsPerBlock
		StakeEnabledHeight:      32,          // PRODUCTION: CoinbaseMaturity + TicketMaturity = 512 (temporary: 16 + 16 = 32)
		StakeValidationHeight:   64,          // PRODUCTION: Normal mainnet value (temporary: 64)
		StakeBaseSigScript:      []byte{0x00, 0x00},
		StakeMajorityMultiplier: 3,
		StakeMajorityDivisor:    4,

		// Decred organization related parameters
		// Organization address is Dcur2mcGjmENx4DhNqDctW5wJCVyT3Qeqkx
		OrganizationPkScript:        hexDecode("a914f5916158e3e2c4551c1796708db8367207ed13bb87"),
		OrganizationPkScriptVersion: 0,
		BlockOneLedger:              tokenPayouts_MainNetParams(),

		// Sanctioned Politeia keys.
		PiKeys: [][]byte{},

		// ~1 day for tspend inclusion
		TreasuryVoteInterval: 288,

		// ~7.2 days for short circuit approval, ~42%
		// target=ticket-pool-equivalent participation
		TreasuryVoteIntervalMultiplier: 12,

		// Sum of tspends within any ~24 day window cannot exceed
		// policy check
		TreasuryExpenditureWindow: 2,

		// policy check is average of prior ~4.8 months + a 50%
		// increase allowance
		TreasuryExpenditurePolicy: 6,

		// 16000 dcr/tew as expense bootstrap
		TreasuryExpenditureBootstrap: 16000 * 1e8,

		TreasuryVoteQuorumMultiplier:   1, // 20% quorum required
		TreasuryVoteQuorumDivisor:      5,
		TreasuryVoteRequiredMultiplier: 3, // 60% yes votes required
		TreasuryVoteRequiredDivisor:    5,

		// HTTP seeders disabled - Monetarium uses manual peer connections for bootstrap
		// To add peers, use --connect=<ip>:9108 or --addpeer=<ip>:9108
		seeders: []string{},

		// SKA (Skarb) dual-coin system parameters for mainnet
		SKAMinRelayTxFee: 1e4, // 0.0001 SKA minimum relay fee

		// SKA coin type configurations for multiple coin support
		SKACoins: map[cointype.CoinType]*SKACoinConfig{
			1: {
				CoinType:       1,
				Name:           "Skarb-1",
				Symbol:         "SKA-1",
				MaxSupply:      10e6 * 1e8, // 10 million SKA-1
				EmissionHeight: 64,         // PRODUCTION: Emit at block 1024, aligned with StakeValidationHeight (temporary: 64)
				EmissionWindow: 4320,       // 30-day emission window (~144 blocks/day * 30)
				Active:         true,
				Description:    "Primary asset-backed SKA coin type for mainnet",
				// Governance-approved emission distribution (TO BE REPLACED WITH REAL ADDRESSES)
				EmissionAddresses: []string{
					"MsMz7mvUPBu5GDFexM2W8KiFxEeToFAC4Wv",
				},
				EmissionAmounts: []int64{
					10e6 * 1e8, // 1,000,000 SKA-1 to staking rewards
				},
				// SECURITY NOTE: This is a placeholder key for development ONLY
				// Production deployment MUST generate secure keys with proper key ceremony
				EmissionKey: mustParseHexPubKey("02f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9"),
			},
			2: {
				CoinType:       2,
				Name:           "Skarb-2",
				Symbol:         "SKA-2",
				MaxSupply:      5e6 * 1e8, // 5 million SKA-2 (proof of concept)
				EmissionHeight: 64,        // Emit at block 150k
				EmissionWindow: 4320,      // 30-day emission window (~144 blocks/day * 30)
				Active:         true,      // Not yet active, for proof of concept
				Description:    "Secondary SKA coin type for proof of concept testing",
				// Governance-approved emission distribution (TO BE REPLACED WITH REAL ADDRESSES)
				EmissionAddresses: []string{
					"MsMz7mvUPBu5GDFexM2W8KiFxEeToFAC4Wv", // Full amount to treasury
				},
				EmissionAmounts: []int64{
					5e6 * 1e8, // 5,000,000 SKA-2 to treasury
				},
				// SECURITY NOTE: This is a placeholder key for development ONLY
				// Production deployment MUST generate secure keys with proper key ceremony
				EmissionKey: mustParseHexPubKey("0316e57ce5fdb617dc192576d9c860f57e7e7a95592aa32e25941731a2eb2c57d6"),
			},
		},

		// Initial SKA types to activate at network genesis
		InitialSKATypes: []cointype.CoinType{1}, // Only SKA-1 initially active
	}
}

// mustParseHexPubKey parses a hex-encoded public key and panics if invalid.
// This is intended for use with hardcoded keys during development.
// SECURITY WARNING: These are placeholder keys - production must use secure key generation.
func mustParseHexPubKey(hexStr string) *secp256k1.PublicKey {
	keyBytes := mustParseHex(hexStr)
	pubKey, err := secp256k1.ParsePubKey(keyBytes)
	if err != nil {
		panic("failed to parse public key: " + err.Error())
	}
	return pubKey
}
