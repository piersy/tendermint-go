// Copyright (C) 2021 Clearmatics

package algorithm

import (
	"crypto/rand"
	"testing"

	"github.com/piersy/tendermint-go/tendermint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newNodeID(t *testing.T) NodeID {
	var nodeID NodeID
	_, err := rand.Read(nodeID[:])
	require.NoError(t, err)
	return nodeID
}

func newValue(t *testing.T) tendermint.Hash {
	var value tendermint.Hash
	_, err := rand.Read(value[:])
	require.NoError(t, err)
	return value
}

func TestStartRound(t *testing.T) {
	var round int64 = 0
	value := newValue(t)

	o := NewBasicOracle(2, 0)

	// We are proposer, expect propose message
	algo := New(newNodeID(t), o)
	expected := &ConsensusMessage{
		MsgType:    Propose,
		Height:     o.Height(),
		Round:      round,
		Value:      value,
		ValidRound: -1,
	}
	cm, to := algo.StartRound(value, round)
	assert.Nil(t, to)
	assert.Equal(t, expected, cm)

	// We are proposer, and validValue has been set, expect propose with
	// validValue.
	algo = New(newNodeID(t), o)
	algo.validValue = newValue(t)
	expected = &ConsensusMessage{
		MsgType:    Propose,
		Height:     o.Height(),
		Round:      round,
		Value:      algo.validValue,
		ValidRound: -1,
	}
	cm, to = algo.StartRound(value, round)
	assert.Nil(t, to)
	assert.Equal(t, expected, cm)

	// We are not the proposer, expect timeout message
	algo = New(newNodeID(t), o)
	expectedTimeout := &Timeout{
		timeoutType: Propose,
		Delay:       1,
		height:      o.Height(),
		round:       round,
	}
	cm, to = algo.StartRound(NilValue, round)
	assert.Nil(t, cm)
	assert.Equal(t, expectedTimeout, to)
}

func TestOnTimeout(t *testing.T) {
	o := &mockOracle{
		height: 1,
	}
	algo := New(newNodeID(t), o)
	to := &Timeout{
		timeoutType: Propose,
		height:      o.height,
		round:       algo.round,
	}

	// Propose timeout, should result in nil prevote
	cm, rc := algo.OnTimeout(to)
	assert.Nil(t, rc)
	assert.Equal(t, &ConsensusMessage{
		MsgType:    Prevote,
		Height:     o.Height(),
		Round:      algo.round,
		Value:      NilValue,
		ValidRound: 0,
	}, cm)

	to = &Timeout{
		timeoutType: Prevote,
		height:      o.height,
		round:       algo.round,
	}
	// Prevote timeout, should result in nil precommit
	cm, rc = algo.OnTimeout(to)
	assert.Nil(t, rc)
	assert.Equal(t, &ConsensusMessage{
		MsgType:    Precommit,
		Height:     o.Height(),
		Round:      algo.round,
		Value:      NilValue,
		ValidRound: 0,
	}, cm)

	to = &Timeout{
		timeoutType: Precommit,
		height:      o.height,
		round:       algo.round,
	}
	// Precommit timeout should result in a round change with no decision.
	cm, rc = algo.OnTimeout(to)
	assert.Nil(t, cm)
	assert.Equal(t, &RoundChange{
		Decision: nil,
		Round:    algo.round + 1,
	}, rc)
}

// Handling a proposal message for a new value
func TestReceiveMessageLine22(t *testing.T) {
	// proposer := newNodeID(t)
	var height uint64 = 1
	var round int64 = 1
	newValueProposal := &ConsensusMessage{
		MsgType:    Propose,
		Height:     height,
		Round:      round,
		Value:      newValue(t),
		ValidRound: -1,
	}
	o := &mockOracle{
		height: height,
		matchingProposal: func(round int64, valueHash *tendermint.Hash) *ConsensusMessage {
			return newValueProposal
		},
		valid: func(v *tendermint.Hash) bool {
			return *v == newValueProposal.Value
		},
	}

	algo := New(newNodeID(t), o)
	algo.round = round
	// We haven't locked a round or a value, so we expect to prevote for the
	// proposal.
	rc, cm, to := algo.ReceiveMessage(newValueProposal)
	assert.Nil(t, rc)
	assert.Nil(t, to)

	expected := &ConsensusMessage{
		MsgType: Prevote,
		Height:  height,
		Round:   round,
		Value:   newValueProposal.Value,
	}
	assert.Equal(t, expected, cm)

	// We locked value v in round 0 and now v is proposed in round 1 so we expect to prevote for it.
	algo = New(newNodeID(t), o)
	algo.round = round
	algo.lockedRound = 0
	algo.lockedValue = newValueProposal.Value
	rc, cm, to = algo.ReceiveMessage(newValueProposal)
	assert.Nil(t, rc)
	assert.Nil(t, to)

	expected = &ConsensusMessage{
		MsgType: Prevote,
		Height:  height,
		Round:   round,
		Value:   newValueProposal.Value,
	}
	assert.Equal(t, expected, cm)

	// We locked value v in round 0 and now a value other than v is proposed in round 1 so we expect to prevote for nil.
	algo = New(newNodeID(t), o)
	algo.round = round
	algo.lockedRound = 0
	algo.lockedValue = newValue(t)
	rc, cm, to = algo.ReceiveMessage(newValueProposal)
	assert.Nil(t, rc)
	assert.Nil(t, to)

	expected = &ConsensusMessage{
		MsgType: Prevote,
		Height:  height,
		Round:   round,
		Value:   NilValue,
	}
	assert.Equal(t, expected, cm)
}

type mockOracle struct {
	valid            func(v *tendermint.Hash) bool
	matchingProposal func(round int64, value *tendermint.Hash) *ConsensusMessage
	prevoteQThresh   func(round int64, value *tendermint.Hash) bool
	precommitQThresh func(round int64, value *tendermint.Hash) bool
	fThresh          func(round int64) bool
	height           uint64
}

func (m *mockOracle) Valid(value *tendermint.Hash) bool {
	return m.valid(value)
}

func (m *mockOracle) MatchingProposal(round int64, value *tendermint.Hash) *ConsensusMessage {
	return m.matchingProposal(round, value)
}

func (m *mockOracle) PrevoteQThresh(round int64, value *tendermint.Hash) bool {
	return m.prevoteQThresh(round, value)
}

func (m *mockOracle) PrecommitQThresh(round int64, value *tendermint.Hash) bool {
	return m.precommitQThresh(round, value)
}

func (m *mockOracle) FThresh(round int64) bool {
	return m.fThresh(round)
}

func (m *mockOracle) Height() uint64 {
	return m.height
}
