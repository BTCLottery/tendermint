package fnConsensus

import (
	"sync"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/state"

	dbm "github.com/tendermint/tendermint/libs/db"
)

const FnVoteSetChannelID = byte(0x50)

type FnConsensusReactor struct {
	p2p.BaseReactor

	connectedPeers map[p2p.ID]p2p.Peer
	mtx            sync.RWMutex
	state          *ReactorState
	db             dbm.DB
	tmStateDB      dbm.DB
	chainID        string
}

func NewFnConsensusReactor(chainID string, db dbm.DB, tmStateDB dbm.DB) *FnConsensusReactor {
	reactor := &FnConsensusReactor{
		connectedPeers: make(map[p2p.ID]p2p.Peer),
		db:             db,
		chainID:        chainID,
		tmStateDB:      tmStateDB,
	}

	reactor.BaseReactor = *p2p.NewBaseReactor("FnConsensusReactor", reactor)
	return reactor
}

func (f *FnConsensusReactor) OnStart() error {
	reactorState, err := LoadReactorState(f.db)
	if err != nil {
		return err
	}
	f.state = reactorState
	return nil
}

// GetChannels returns the list of channel descriptors.
func (f *FnConsensusReactor) GetChannels() []*conn.ChannelDescriptor {
	// Priorities are deliberately set to low, to prevent interfering with core TM
	return []*conn.ChannelDescriptor{
		{
			ID:                  FnVoteSetChannelID,
			Priority:            25,
			SendQueueCapacity:   100,
			RecvBufferCapacity:  100,
			RecvMessageCapacity: 10,
		},
	}
}

// AddPeer is called by the switch when a new peer is added.
func (f *FnConsensusReactor) AddPeer(peer p2p.Peer) {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	f.connectedPeers[peer.ID()] = peer
	// Start go routine for state sync
	// Start go routine for vote sync
}

// RemovePeer is called by the switch when the peer is stopped (due to error
// or other reason).
func (f *FnConsensusReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	// Stop go routine for state sync
	// Stop go routine for vote sync
	delete(f.connectedPeers, peer.ID())
}

func (f *FnConsensusReactor) areWeValidator() bool {
	return true
}

// Receive is called when msgBytes is received from peer.
//
// NOTE reactor can not keep msgBytes around after Receive completes without
// copying.
//
// CONTRACT: msgBytes are not nil.
func (f *FnConsensusReactor) Receive(chID byte, peer p2p.Peer, msgBytes []byte) {
	currentState := state.LoadState(f.tmStateDB)

	switch chID {
	case FnVoteSetChannelID:
		remoteVoteSet := &FnVoteSet{}
		if err := remoteVoteSet.Unmarshal(msgBytes); err != nil {
			f.Logger.Error("FnConsensusReactor: Invalid Data passed, ignoring...")
			return
		}
		meta := remoteVoteSet.Meta(f.chainID, currentState.Validators)
		if !meta.IsValid {
			f.Logger.Error("FnConsensusReactor: Invalid VoteSet specified, ignoring...")
			return
		}

		if meta.IsMaj23 {
			f.Logger.Error("FnConsensusReactor: Protocol violation: Received VoteSet with majority of validators signed, Ignoring...")
			return
		}

		// TODO: Check nonce with mainnet before accepting remote vote set
		if f.state.CurrentVoteSet == nil {
			f.state.CurrentVoteSet = remoteVoteSet
		} else {
			if err := f.state.CurrentVoteSet.Merge(remoteVoteSet); err != nil {
				f.Logger.Error("FnConsensusReactor: Unable to merge remote vote set into our own.", "error:", err)
				return
			}
		}

		if f.areWeValidator() {
			// Add our vote
			// If it becomes maj23, call Oracle and return
		}

		// Propogate voteset to all nodes if voteSet is not identical to the one sent by Peer, otherwise send to all nodes minus peer who sent it

		break
	default:
		f.Logger.Error("FnConsensusReactor: Unknown channel: %v", chID)
	}
}
