package algorithm

import (
	"github.com/piersy/tendermint-go/tendermint"
)

type BasicOracle struct {
	numValidators int
	s             *Store
	height        uint64
}

func NewBasicOracle(numValidators int, height uint64) *BasicOracle {
	return &BasicOracle{
		numValidators: numValidators,
		height:        height,
		s:             NewStore(),
	}
}

func (b *BasicOracle) FThresh(round int64) bool {
	return b.s.CountAll(round) > b.numValidators/3
}

func (b *BasicOracle) Height() uint64 {
	return b.height
}

func (b *BasicOracle) MatchingProposal(round int64, valueHash *tendermint.Hash) *ConsensusMessage {
	return b.s.MatchingProposal(round, *valueHash)
}

func (b *BasicOracle) PrecommitQThresh(round int64, valueHash *tendermint.Hash) bool {
	return b.s.CountPrecommits(round, *valueHash) >= (b.numValidators*3/2)+1
}

func (b *BasicOracle) PrevoteQThresh(round int64, valueHash *tendermint.Hash) bool {
	return b.s.CountPrevotes(round, *valueHash) >= (b.numValidators*3/2)+1
}

func (b *BasicOracle) Valid(valueHash *tendermint.Hash) bool {
	return b.s.Valid(valueHash)
}
