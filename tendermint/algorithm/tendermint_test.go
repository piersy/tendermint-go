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
	numValidators := 2
	nodeID := newNodeID(t)

	s := NewStore()
	o := NewBasicOracle(numValidators, 0, s)

	// We are proposer, expect propose message
	algo := New(nodeID, o)
	expected := &ConsensusMessage{
		Sender:     nodeID,
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
	algo = New(nodeID, o)
	algo.validValue = newValue(t)
	expected = &ConsensusMessage{
		Sender:     nodeID,
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
	algo = New(nodeID, o)
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
	nodeID := newNodeID(t)
	algo := New(nodeID, o)
	to := &Timeout{
		timeoutType: Propose,
		height:      o.height,
		round:       algo.round,
	}

	// Propose timeout, should result in nil prevote
	cm, rc := algo.OnTimeout(to)
	assert.Nil(t, rc)
	assert.Equal(t, &ConsensusMessage{
		Sender:     nodeID,
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
		Sender:     nodeID,
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
func TestSuccessfulRun(t *testing.T) {
	// proposer := newNodeID(t)
	var height uint64 = 1
	var round int64 = 0
	value := newValue(t)
	nodeID := newNodeID(t)
	numValidators := 2
	s := NewStore()
	o := NewBasicOracle(numValidators, height, s)
	algo := New(nodeID, o)
	proposal, to := algo.StartRound(value, round)
	assert.Nil(t, to)
	require.NoError(t, s.AddMessage(proposal))
	s.SetValid(&proposal.Value)
	// We haven't locked a round or a value, so we expect to prevote for the
	// proposal.
	rc, cm, to := algo.ReceiveMessage(proposal)
	assert.Nil(t, rc)
	assert.Nil(t, to)

	expected := &ConsensusMessage{
		Sender:  nodeID,
		MsgType: Prevote,
		Height:  height,
		Round:   round,
		Value:   value,
	}
	assert.Equal(t, expected, cm)
	require.NoError(t, s.AddMessage(cm))

	// Process the prevote we expect no state change since we need to see 2 prevotes to progress.
	rc, cm, to = algo.ReceiveMessage(cm)
	assert.Nil(t, rc)
	assert.Nil(t, to)
	assert.Nil(t, cm)

	otherNodePrevote := &ConsensusMessage{
		Sender:  newNodeID(t),
		MsgType: Prevote,
		Height:  height,
		Round:   round,
		Value:   value,
	}
	require.NoError(t, s.AddMessage(otherNodePrevote))

	// Process another prevote, this should result in a precommit, since we have recieved 2 votes.
	rc, cm, to = algo.ReceiveMessage(otherNodePrevote)
	assert.Nil(t, rc)
	assert.Nil(t, to)

	expected = &ConsensusMessage{
		Sender:  nodeID,
		MsgType: Precommit,
		Height:  height,
		Round:   round,
		Value:   value,
	}
	assert.Equal(t, expected, cm)
	require.NoError(t, s.AddMessage(cm))

	// Process the precommit we expect no state change since we need to see 2 precommits to progress.
	rc, cm, to = algo.ReceiveMessage(cm)
	assert.Nil(t, rc)
	assert.Nil(t, to)
	assert.Nil(t, cm)

	otherNodePrecommit := &ConsensusMessage{
		Sender:  newNodeID(t),
		MsgType: Precommit,
		Height:  height,
		Round:   round,
		Value:   value,
	}
	require.NoError(t, s.AddMessage(otherNodePrecommit))

	// Process the second precommit we expect to see a state change because we have seen 2 precommits.
	rc, cm, to = algo.ReceiveMessage(otherNodePrecommit)
	assert.Nil(t, to)
	assert.Nil(t, cm)

	expectedRoundChange := &RoundChange{
		Round:    0,
		Decision: proposal,
	}

	require.Equal(t, expectedRoundChange, rc)
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
