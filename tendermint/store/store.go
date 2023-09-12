package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"

	"github.com/piersy/tendermint-go/tendermint"
	"github.com/piersy/tendermint-go/tendermint/algorithm"
)

type Store struct {
	messages []*algorithm.ConsensusMessage
	hashes   map[tendermint.Hash]struct{}
	valid    map[tendermint.Hash]struct{}
}

// AddMessage adds the given message to the store.
func (s *Store) AddMessage(m *algorithm.ConsensusMessage) error {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(m)
	if err != nil {
		return err
	}
	h := sha256.Sum256(b.Bytes())
	_, ok := s.hashes[h]
	if !ok {
		s.hashes[h] = struct{}{}
		s.messages = append(s.messages, m)
	}
	return nil
}

// SetValid sets the given value hash as valid.
func (s *Store) SetValid(valueHash *tendermint.Hash) {
	s.valid[*valueHash] = struct{}{}
}

// Valid checks the given value hash to see if it has been marked valid.
func (s *Store) Valid(valueHash *tendermint.Hash) bool {
	_, ok := s.valid[*valueHash]
	return ok
}

// Returns a proposal for the given round and valueHash if it exists.
func (s *Store) MatchingProposal(round int64, valueHash tendermint.Hash) *algorithm.ConsensusMessage {
	for _, v := range s.messages {
		if v.MsgType == algorithm.Propose && v.Round == round && v.Value == valueHash {
			return v
		}
	}
	return nil
}

// CountPrevotes returns true if a there is a quorum of prevotes for valueHash.
// Passing algorithm.NilValue as the valueHash acts as a wildcard and will
// cause all prevotes for the round to be counted.
func (s *Store) CountPrevotes(round int64, valueHash tendermint.Hash) int {
	result := 0
	for _, v := range s.messages {
		if v.MsgType == algorithm.Prevote && v.Round == round && (valueHash == algorithm.NilValue || v.Value == valueHash) {
			result++
		}
	}
	return result
}

// CountPrecommits returns true if a there is a quorum of prevotes for valueHash.
// Passing algorithm.NilValue as the valueHash acts as a wildcard and will
// cause all precommits for the round to be counted.
func (s *Store) CountPrecommits(round int64, valueHash tendermint.Hash) int {
	result := 0
	for _, v := range s.messages {
		if v.MsgType == algorithm.Precommit && v.Round == round && (valueHash == algorithm.NilValue || v.Value == valueHash) {
			result++
		}
	}
	return result
}

// CountAll counts the number of precommit and prevote messages for the given round.
func (s *Store) CountAll(round int64) int {
	result := 0
	for _, v := range s.messages {
		if (v.MsgType == algorithm.Precommit || v.MsgType == algorithm.Prevote) && v.Round == round && v.Value == algorithm.NilValue {
			result++
		}
	}
	return result
}
