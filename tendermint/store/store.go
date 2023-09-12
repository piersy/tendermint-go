package store

import (
	"github.com/piersy/tendermint-go/tendermint"
	"github.com/piersy/tendermint-go/tendermint/algorithm"
)

type Store struct {
	messages []algorithm.ConsensusMessage
}

func (s *Store) MatchingProposal(round int64, valueHash tendermint.Hash) *algorithm.ConsensusMessage {
	for _, v := range s.messages {
		if v.Round == round && v.Value == valueHash {
			return v
		}
	}
}

// MatchingProposal(*ConsensusMessage) *ConsensusMessage
// // PrevoteQThresh returns true if a there is a quorum of prevotes for valueID.
// PrevoteQThresh(round int64, valueID *ValueID) bool
// // PrevoteQThresh returns true if a there is a quorum of precommits for valueID.
// PrecommitQThresh(round int64, valueID *ValueID) bool
// // FThresh indicates whether we have messages whose voting power exceeds
// // the failure threshold for the given round.
// FThresh(round int64) bool
