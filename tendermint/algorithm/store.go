package algorithm

import (
	"fmt"

	"github.com/piersy/tendermint-go/tendermint"
)

type Store struct {
	// Messages is a map of arrays one per node, with the 0 element holding the
	// prevote and the 1 element holding the precommit.
	proposals  map[int]*ConsensusMessage
	messages   map[int]map[NodeID][2]*ConsensusMessage
	msgByHash  map[tendermint.Hash][]byte
	validValue map[*tendermint.Hash]struct{}
}

func NewStore() *Store {
	return &Store{
		proposals:  make(map[int]*ConsensusMessage),
		messages:   make(map[int]map[NodeID][2]*ConsensusMessage),
		msgByHash:  make(map[tendermint.Hash][]byte),
		validValue: make(map[*tendermint.Hash]struct{}),
	}
}

// AddMessage adds the given message to the store.

// Cases we need to check for any node sending a different message for a
// position that they have already sent a message for. E.G. Proposer sending 2
// differetn propose messages or any node sending 2 different prevote or
// precommit messages.
func (s *Store) AddMessage(m *ConsensusMessage, raw []byte, hash tendermint.Hash) error {
	_, ok := s.msgByHash[hash]
	if ok {
		// We received duplicate message from network, ignore.
		return nil
	}

	roundMsgs := s.messages[m.Round]
	if roundMsgs == nil {
		roundMsgs = make(map[NodeID][2]*ConsensusMessage)
		s.messages[m.Round] = roundMsgs
	}

	msgs := roundMsgs[m.Sender]
	switch m.MsgType {
	case Propose:
		if s.proposals[m.Round] != nil {
			return fmt.Errorf("equivocation detected received %v & %v", s.proposals[m.Round], m)
		}
		s.proposals[m.Round] = m
	case Prevote:
		if msgs[0] != nil {
			return fmt.Errorf("equivocation detected received %v & %v", msgs[0], m)
		}
		msgs[0] = m
	case Precommit:
		if msgs[1] != nil {
			return fmt.Errorf("equivocation detected received %v & %v", msgs[1], m)
		}
		msgs[1] = m
	}
	roundMsgs[m.Sender] = msgs

	// Store raw message by hash
	s.msgByHash[hash] = raw
	return nil
}

// SetValid sets the given value hash as valid.
func (s *Store) SetValid(valueHash *tendermint.Hash) {
	s.validValue[valueHash] = struct{}{}
}

// Valid checks the given value hash to see if it has been marked valid.
func (s *Store) Valid(valueHash *tendermint.Hash) bool {
	_, ok := s.validValue[valueHash]
	return ok
}

// Returns a proposal for the given round & valueHash or nil if none exists.
func (s *Store) MatchingProposal(round int, valueHash tendermint.Hash) *ConsensusMessage {
	proposal := s.proposals[round]
	if proposal != nil && proposal.Value == valueHash {
		return proposal
	}
	return nil
}

// CountPrevotes returns true if a there is a quorum of prevotes for valueHash.
// Passing nil as the valueHash acts as a wildcard and will cause all prevotes
// for the round to be counted.
func (s *Store) CountPrevotes(round int, valueHash *tendermint.Hash) int {
	result := 0
	for _, msgs := range s.messages[round] {
		if msgs[0] != nil && (valueHash == nil || msgs[0].Value == *valueHash) {
			result++
		}
	}
	return result
}

// CountPrecommits returns true if a there is a quorum of prevotes for
// valueHash. Passing nil as the valueHash acts as a wildcard and will cause
// all precommits for the round to be counted.
func (s *Store) CountPrecommits(round int, valueHash *tendermint.Hash) int {
	result := 0
	for _, msgs := range s.messages[round] {
		if msgs[1] != nil && (valueHash == nil || msgs[1].Value == *valueHash) {
			result++
		}
	}
	return result
}

// CountAll counts the number of precommit and prevote messages for the given round voting for NilValue
func (s *Store) CountFailures(round int) int {
	result := 0
	for _, msgs := range s.messages[round] {
		if msgs[0] != nil && msgs[0].Value == NilValue {
			result++
		}
		if msgs[1] != nil && msgs[1].Value == NilValue {
			result++
		}
	}
	return result
}
