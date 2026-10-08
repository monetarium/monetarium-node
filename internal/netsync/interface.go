// Copyright (c) 2020-2021 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"github.com/monetarium/monetarium-node/dcrutil"
	"github.com/monetarium/monetarium-node/mixing"
)

// PeerNotifier provides an interface to notify peers of status changes related
// to blocks and transactions.
type PeerNotifier interface {
	// AnnounceNewTransactions generates and relays inventory vectors and
	// notifies websocket clients of the passed transactions.
	AnnounceNewTransactions(txns []*dcrutil.Tx)

	// AnnounceMixMessages generates and relays inventory vectors of the
	// passed messages.
	AnnounceMixMessages(msgs []mixing.Message)

	// AnnounceIsCurrent notifies peers of the local address once the node
	// transitions to believing the chain is current.  This is used to
	// ensure outbound peers that connected while the node was still syncing
	// learn the local address after every catch-up, not only when they
	// connect to a node that is already current.
	AnnounceIsCurrent()
}
