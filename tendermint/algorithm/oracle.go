package algorithm

import (
	"github.com/piersy/tendermint-go/tendermint"
)

type BasicOracle struct {
	numValidators int
	store         *Store
	height        uint64
}

func NewBasicOracle(numValidators int, height uint64, store *Store) *BasicOracle {
	return &BasicOracle{
		numValidators: numValidators,
		height:        height,
		store:         store,
	}
}

func (b *BasicOracle) FThresh(round int) bool {
	return b.store.CountFailures(round) > b.numValidators/3
}

func (b *BasicOracle) Height() uint64 {
	return b.height
}

func (b *BasicOracle) MatchingProposal(round int, valueHash *tendermint.Hash) *ConsensusMessage {
	return b.store.MatchingProposal(round, *valueHash)
}

func (b *BasicOracle) PrecommitQThresh(round int, valueHash *tendermint.Hash) bool {
	return b.store.CountPrecommits(round, valueHash) >= (b.numValidators*2/3)+1
}

func (b *BasicOracle) PrevoteQThresh(round int, valueHash *tendermint.Hash) bool {
	return b.store.CountPrevotes(round, valueHash) >= (b.numValidators*2/3)+1
}

func (b *BasicOracle) Valid(valueHash *tendermint.Hash) bool {
	return b.store.Valid(valueHash)
}
