// Copyright (C) 2021 Clearmatics

// Package algorithm implements the tendremint consensus protocol.
//
// This package only contains the core state transition logic for tendermint as
// described by the pseudocode in the whitepaper here -
// https://arxiv.org/pdf/1807.04938.pdf
//
// References to line numbers are referencing the line numbers of the
// whitepaper pseudocode.
package algorithm

import (
	"encoding/hex"
	"fmt"

	"github.com/piersy/tendermint-go/tendermint"
)

// NilValue  represents 'nil' in the tendermint whitepaper.
var NilValue tendermint.Hash

// NodeID represents the ID of a node, although not explicitly mentioned in the
// whitepaper, we need a way to identify ourselves if we are to ask if we are
// the proposer and it is also useful for logging puroposes.
type NodeID [20]byte

func (n NodeID) String() string {
	return hex.EncodeToString(n[:3])
}

// Step represents the different algorithm steps as described in the
// whitepaper.
type Step uint8

const (
	Propose Step = iota
	Prevote
	Precommit
)

func (s Step) String() string {
	switch s {
	case Propose:
		return "Propose"
	case Prevote:
		return "Prevote"
	case Precommit:
		return "Precommit"
	default:
		panic(fmt.Sprintf("Unrecognised step value %d", s))
	}
}

// ShortString is useful for printing compact messages.
func (s Step) ShortString() string {
	switch s {
	case Propose:
		return "pp"
	case Prevote:
		return "pv"
	case Precommit:
		return "pc"
	default:
		panic(fmt.Sprintf("Unrecognised step value %d", s))
	}
}

// In returns true if s is one of the provided steps.
func (s Step) In(steps ...Step) bool {
	for _, step := range steps {
		if s == step {
			return true
		}
	}
	return false
}

// Timeout is returned to the caller to indicate that they should schedule a
// timeout with the given delay, the Height, Round and Step are used by the
// algorithm to check whether the timout is still valid, timeouts are only
// valid if they trigger in the same height, round and step as when they were
// scheduled.
type Timeout struct {
	timeoutType Step
	Delay       uint
	height      uint64
	round       int
}

// ConsensusMessage is returned to the caller to indicate that this message
// should be broadcast to the network.
type ConsensusMessage struct {
	Sender     NodeID
	MsgType    Step
	Height     uint64
	Round      int
	Value      tendermint.Hash
	ValidRound int // This field only has meaning for propose step. For prevote and precommit this value is ignored.
}

func (cm *ConsensusMessage) String() string {
	var vr string
	if cm.MsgType == Propose {
		vr = fmt.Sprintf(" vr:%-3d", cm.ValidRound)
	}
	return fmt.Sprintf("s:%-3s h:%-3d r:%-3d v:%-6s%s", cm.MsgType.ShortString(), cm.Height, cm.Round, cm.Value.String(), vr)
}

// Oracle is used to answer questions the algorithm may have about its
// state, such as 'Am I the proposer' or 'Have i reached prevote quorum
// threshold for value with id v?'
type Oracle interface {
	// Valid returns true if the value associated with the given tendermint.Hash is
	// valid.
	Valid(*tendermint.Hash) bool
	// MatchingProposal returns a Proposal message with the given round and valueHash if it exists.
	MatchingProposal(round int, valueHash *tendermint.Hash) *ConsensusMessage
	// PrevoteQThresh returns true if a there is a quorum of prevotes for valueID.
	PrevoteQThresh(round int, valueHash *tendermint.Hash) bool
	// PrevoteQThresh returns true if a there is a quorum of precommits for valueID.
	PrecommitQThresh(round int, valueHash *tendermint.Hash) bool
	// FThresh indicates whether we have messages whose voting power exceeds
	// the failure threshold for the given round.
	FThresh(round int) bool
	// Height returns the current height.
	Height() uint64
}

// Algorithm implements the state transitions defined by the tendermint
// whitepaper. There are 2 main functions, StartRound which is called at the
// beginning of each round, and then ReceiveMessage which is called with each
// message received from the network and drives subsequent state changes.
type Algorithm struct {
	nodeID         NodeID
	round          int
	step           Step
	lockedRound    int
	lockedValue    tendermint.Hash
	validRound     int
	validValue     tendermint.Hash
	line34Executed bool
	line36Executed bool
	line47Executed bool
	oracle         Oracle
}

// New creates a new instance of Algorithm.
func New(nodeID NodeID, oracle Oracle) *Algorithm {
	return &Algorithm{
		nodeID: nodeID,
		// We set round to be -1 so we can enforce the check that start round
		// is always called with a round greater than, the current round.
		round:       -1,
		lockedRound: -1,
		lockedValue: NilValue,
		validRound:  -1,
		validValue:  NilValue,
		oracle:      oracle,
	}
}

func (a Algorithm) height() uint64 {
	return a.oracle.Height()
}

func (a *Algorithm) msg(msgType Step, value tendermint.Hash) *ConsensusMessage {
	cm := &ConsensusMessage{
		Sender:  a.nodeID,
		MsgType: msgType,
		Height:  a.height(),
		Round:   a.round,
		Value:   value,
	}
	if a.step == Propose {
		cm.ValidRound = a.validRound
	}
	return cm
}

func (a *Algorithm) timeout(timeoutType Step) *Timeout {
	if a.round < 0 {
		panic(fmt.Sprintf("at this point round should be greater than or eaqual to zero instead got: %d", a.round))
	}
	return &Timeout{
		timeoutType: timeoutType,
		height:      a.height(),
		round:       a.round,
		Delay:       1 + uint(a.round),
	}
}

// Start round takes a round to start and clears the first time flags. If this
// node is a proposer (indicated by a non nil proposalValue) it retures a
// proposal ConsensusMessage to be broadcast, otherwise it returns a Timeout to
// be scheduled.
func (a *Algorithm) StartRound(proposalValue tendermint.Hash, round int) (*ConsensusMessage, *Timeout) {
	// println(a.nodeID.String(), height, "isproposer", a.oracle.Proposer(round, a.nodeID))

	// sanity check
	if round <= a.round {
		panic(fmt.Sprintf("New round must be more than the current round. Previous round: %-3d, new round: %-3d", a.round, round))
	}

	// Reset first time flags
	a.line34Executed = false
	a.line36Executed = false
	a.line47Executed = false

	a.round = round
	a.step = Propose
	if proposalValue != NilValue {
		if a.validValue != NilValue {
			proposalValue = a.validValue
		}
		// println(a.nodeID.String(), a.height(), "returning message", value.String())
		return a.msg(Propose, proposalValue), nil
	} else { //nolint
		return nil, a.timeout(Propose)
	}
}

// RoundChange indicates that the caller should initiate a round change by
// calling StartRound with the enclosed Height and Round. If Decision is set
// this indicates that a decision has been reached it will contain the proposal
// that was decided upon, Decision can only be set when Round is 0.
type RoundChange struct {
	Round    int
	Decision *ConsensusMessage
}

// ReceiveMessage processes a consensus message and returns 3 values of which
// at most one can be non nil, although all can be nil, which indicates no
// state change.
//
// The values that can be returned are as follows:
//
//   - *ConsensusMessage - This should be broadcast to the rest of the network,
//     including ourselves. This action can be taken asynchronously.
//
//   - *RoundChange - This indicates that we need to progress to the next round,
//     and possibly next height, ultimately leading to calling StartRound with the
//     enclosed Height and Round. The call to StartRound must be executed by the
//     calling goroutine before any other call to ReceiveMessage.
//
//   - *Timeout - This should be scheduled based to call the corresponding OnTimeout*
//     method after the Delay with the enclosed Height and Round. This action can be
//     taken asynchronously.
func (a *Algorithm) ReceiveMessage(cm *ConsensusMessage) (*RoundChange, *ConsensusMessage, *Timeout) {
	r := a.round
	s := a.step
	o := a.oracle
	t := cm.MsgType

	// look up matching proposal, in the case of ost message with msgType
	// proposal the matching proposal is the message.
	p := o.MatchingProposal(cm.Round, &cm.Value)

	// Some of the checks in these upon conditions are omitted because they have already been checked.
	//
	// - We do not check height because we only execute this code when the
	// message height matches the current height.
	//
	// - We do not check whether the message comes from a proposer since this
	// is checked before calling this method and we do not process proposals
	// from non proposers.
	//
	// The upon conditions have been re-ordered such that those with outcomes
	// that would supersede the outcome of others come before the others.
	// Specifically the upon conditions for a given step that schedule
	// timeouts, have been moved after the upon conditions for that step that
	// would result in broadcasting a message for a value other than nil or
	// deciding on a value. This ensures that we are able to return when we
	// find a condition that has been met, because we know that the result of
	// this condition will supersede results from other later conditions that
	// may have been met. This approach will hopefully go someway to cutting
	// down unnecessary network traffic between nodes.

	// Line 22
	if t.In(Propose) && cm.Round == r && cm.ValidRound == -1 && s == Propose {
		a.step = Prevote
		if o.Valid(&cm.Value) && a.lockedRound == -1 || a.lockedValue == cm.Value {
			// println(a.nodeID.String(), a.height(), cm.String(), "line 22 val")
			return nil, a.msg(Prevote, cm.Value), nil
		} else { //nolint
			// println(a.nodeID.String(), a.height(), cm.String(), "line 22 nil")
			return nil, a.msg(Prevote, NilValue), nil
		}
	}

	// Line 28
	if t.In(Propose, Prevote) && p != nil && p.Round == r && o.PrevoteQThresh(p.ValidRound, &p.Value) && s == Propose && (p.ValidRound >= 0 && p.ValidRound < r) {
		a.step = Prevote
		if o.Valid(&p.Value) && (a.lockedRound <= p.ValidRound || a.lockedValue == p.Value) {
			// println(a.nodeID.String(), a.height(), cm.String(), "line 28 val")
			return nil, a.msg(Prevote, p.Value), nil
		} else { //nolint
			// println(a.nodeID.String(), a.height(), cm.String(), "line 28 nil")
			return nil, a.msg(Prevote, NilValue), nil
		}
	}

	////println(a.nodeId.String(), a.height(), t.In(Propose, Prevote), p != nil, p.Round == r, o.PrevoteQThresh(r, &p.Value), o.Valid(p.Value), s >= Prevote, !a.line36Executed)
	// Line 36
	if t.In(Propose, Prevote) && p != nil && p.Round == r && o.PrevoteQThresh(r, &p.Value) && o.Valid(&p.Value) && s >= Prevote && !a.line36Executed {
		a.line36Executed = true
		if s == Prevote {
			a.lockedValue = p.Value
			a.lockedRound = r
			a.step = Precommit
		}
		a.validValue = p.Value
		a.validRound = r
		// println(a.nodeID.String(), a.height(), cm.String(), "line 36 val")
		return nil, a.msg(Precommit, p.Value), nil
	}

	// Line 44
	if t.In(Prevote) && cm.Round == r && o.PrevoteQThresh(r, &NilValue) && s == Prevote {
		a.step = Precommit
		// println(a.nodeID.String(), a.height(), cm.String(), "line 44 nil")
		return nil, a.msg(Precommit, NilValue), nil
	}

	// Line 34
	if t.In(Prevote) && cm.Round == r && o.PrevoteQThresh(r, nil) && s == Prevote && !a.line34Executed {
		a.line34Executed = true
		// println(a.nodeID.String(), a.height(), cm.String(), "line 34 timeout")
		return nil, nil, a.timeout(Prevote)
	}

	// Line 49
	if t.In(Propose, Precommit) && p != nil && o.PrecommitQThresh(p.Round, &p.Value) {
		if o.Valid(&p.Value) {
			a.lockedRound = -1
			a.lockedValue = NilValue
			a.validRound = -1
			a.validValue = NilValue
		}
		// println(a.nodeID.String(), a.height(), cm.String(), "line 49 decide")
		// Return the decided proposal
		return &RoundChange{Round: 0, Decision: p}, nil, nil
	}

	// Line 47
	if t.In(Precommit) && cm.Round == r && o.PrecommitQThresh(r, nil) && !a.line47Executed {
		a.line47Executed = true
		// println(a.nodeID.String(), a.height(), cm.String(), "line 47 timeout")
		return nil, nil, a.timeout(Precommit)
	}

	// Line 55
	if cm.Round > r && o.FThresh(cm.Round) {
		return &RoundChange{Round: cm.Round}, nil, nil
	}
	// println(a.nodeID.String(), a.height(), cm.String(), "no condition match")
	return nil, nil, nil
}

func (a *Algorithm) OnTimeout(t *Timeout) (*ConsensusMessage, *RoundChange) {
	if t.height == a.height() && t.round == a.round {
		switch t.timeoutType {
		case Propose:
			a.step = Prevote
			return a.msg(Prevote, NilValue), nil
		case Prevote:
			a.step = Precommit
			return a.msg(Precommit, NilValue), nil
		case Precommit:
			return nil, &RoundChange{Round: a.round + 1}
		default:
			panic(fmt.Sprintf("unrecognized timeout type %d", t.timeoutType))
		}
	}
	return nil, nil
}
