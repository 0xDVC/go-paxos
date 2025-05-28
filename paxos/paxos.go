package paxos

import (
	"fmt"
	"time"
)

// message types for paxos protocol 
type MsgType int

const (
	MsgPrepare MsgType = iota + 1
	MsgPromise
	MsgPropose
	MsgAccept
)

// message between nodes 
type Message struct {
	From int
	To   int
	Type MsgType
	Seq  int
	Val  string
}

// get the value being proposed
func (m *Message) GetProposeVal() string {
	return m.Val
}

// get the sequence number
func (m *Message) GetProposeSeq() int {
	return m.Seq
}

// network layer between nodes 
type Network struct {
	RecvQueue map[int]chan Message
}

// create network with channels for each node 
func CreateNetwork(nodes ...int) *Network {
	nt := Network{RecvQueue: make(map[int]chan Message)}
	for _, node := range nodes {
		nt.RecvQueue[node] = make(chan Message, 1024)
	}
	return &nt
}

// send message to target node 
func (n *Network) SendTo(m Message) {
	fmt.Printf("[net] send: %d->%d type:%d seq:%d val:%s\n", m.From, m.To, m.Type, m.Seq, m.Val)
	n.RecvQueue[m.To] <- m
}

// get message for node or timeout 
func (n *Network) RecvFrom(id int) *Message {
	select {
	case retMsg := <-n.RecvQueue[id]:
		fmt.Printf("[net] recv: %d<-%d type:%d seq:%d val:%s\n", retMsg.To, retMsg.From, retMsg.Type, retMsg.Seq, retMsg.Val)
		return &retMsg
	case <-time.After(time.Second):
		return nil
	}
}

// network wrapper for a specific node 
type NodeNetwork struct {
	ID  int
	Net *Network
}

// get network for a specific node
func (n *Network) GetNodeNetwork(id int) NodeNetwork {
	return NodeNetwork{ID: id, Net: n}
}

// send message through network
func (n *NodeNetwork) Send(m Message) {
	n.Net.SendTo(m)
}

// receive message from network
func (n *NodeNetwork) Recv() *Message {
	return n.Net.RecvFrom(n.ID)
}

// proposer starts the consensus process 
type Proposer struct {
	ID         int
	Seq        int             // sequence number for proposals
	ProposeVal string          // value being proposed
	Acceptors  map[int]Message // tracks responses from acceptors
	NT         NodeNetwork     // network for communication
}

// create new proposer 
func NewProposer(id int, val string, nt NodeNetwork, acceptorIds ...int) *Proposer {
	p := Proposer{ID: id, ProposeVal: val, Seq: 1000, NT: nt}
	p.Acceptors = make(map[int]Message, len(acceptorIds))

	for _, acceptorId := range acceptorIds {
		p.Acceptors[acceptorId] = Message{}
	}
	return &p
}

// run the proposer 
func (p *Proposer) Run() {
	fmt.Printf("[proposer %d] starting with value: %s\n", p.ID, p.ProposeVal)

	// phase 1: prepare until we get promises from a majority 
	for !p.MajorityReached() {
		prepareMessages := p.Prepare()
		for _, msg := range prepareMessages {
			p.NT.Send(msg)
		}

		promise := p.NT.Recv()
		if promise == nil {
			continue
		}

		if promise.Type == MsgPromise {
			p.HandlePromise(*promise)
		}
	}

	// phase 2: propose to those who promised 
	proposeMessages := p.Propose()
	for _, msg := range proposeMessages {
		p.NT.Send(msg)
	}
}

// prepare phase 1 of paxos with new sequence number 
func (p *Proposer) Prepare() []Message {
	p.Seq++
	var msgList []Message
	sendCount := 0

	for acceptorId := range p.Acceptors {
		if sendCount >= p.Majority() {
			break
		}

		msg := Message{
			From: p.ID,
			To:   acceptorId,
			Type: MsgPrepare,
			Seq:  p.GetProposeNum(),
			Val:  p.ProposeVal,
		}
		msgList = append(msgList, msg)
		sendCount++
	}

	return msgList
}

		// propose phase 2 of paxos to acceptors that promised 
func (p *Proposer) Propose() []Message {
	var msgList []Message
	sendCount := 0

	for acceptorId, acceptorMsg := range p.Acceptors {
		if acceptorMsg.GetProposeSeq() == p.GetProposeNum() {
			msg := Message{
				From: p.ID,
				To:   acceptorId,
				Type: MsgPropose,
				Seq:  p.GetProposeNum(),
				Val:  p.ProposeVal,
			}
			msgList = append(msgList, msg)
			sendCount++

			if sendCount >= p.Majority() {
				break
			}
		}
	}

	return msgList
}

// handle promise message from acceptor 
func (p *Proposer) HandlePromise(promise Message) {
	p.Acceptors[promise.From] = promise
}

// check if we have promises from majority of acceptors 
func (p *Proposer) MajorityReached() bool {
	promiseCount := 0
	currentProposeNum := p.GetProposeNum()

	for _, acceptorMsg := range p.Acceptors {
		if acceptorMsg.GetProposeSeq() == currentProposeNum {
			promiseCount++
		}
	}

	return promiseCount >= p.Majority()
}

// get number needed for majority 
func (p *Proposer) Majority() int {
	return len(p.Acceptors)/2 + 1
}

// generate unique proposal number 
func (p *Proposer) GetProposeNum() int {
	// combine sequence and id to make globally unique proposal number 
	return p.Seq<<4 | p.ID
}

// acceptor gets messages and responds 
type Acceptor struct {
	ID          int
	PromisedSeq int    // highest sequence promised
	AcceptedSeq int    // sequence of accepted value
	AcceptedVal string // value accepted
	Learners    []int  // learners to notify on acceptance
	NT          NodeNetwork
}

// create new acceptor 
func NewAcceptor(id int, nt NodeNetwork, learners ...int) *Acceptor {
	return &Acceptor{
		ID:          id,
		NT:          nt,
		PromisedSeq: 0,
		AcceptedSeq: 0,
		Learners:    learners,
	}
}

// run the acceptor 
func (a *Acceptor) Run() {
	fmt.Printf("[acceptor %d] starting (will notify %d learners)\n", a.ID, len(a.Learners))

	for {
		msg := a.NT.Recv()
		if msg == nil {
			continue
		}

		switch msg.Type {
		case MsgPrepare:
			response := a.HandlePrepare(*msg)
			if response != nil {
				a.NT.Send(*response)
			}
		case MsgPropose:
			if a.HandlePropose(*msg) {
				// notify learners when a value is accepted
				for _, learnerId := range a.Learners {
					acceptMsg := Message{
						From: a.ID,
						To:   learnerId,
						Type: MsgAccept,
						Seq:  msg.Seq,
						Val:  msg.Val,
					}
					a.NT.Send(acceptMsg)
				}
			}
		default:
			fmt.Printf("[acceptor %d] unknown message type %d\n", a.ID, msg.Type)
		}
	}
}

// handle prepare message
func (a *Acceptor) HandlePrepare(prepare Message) *Message {
	// only promise if sequence is higher than what's been promised
	if prepare.Seq <= a.PromisedSeq {
		fmt.Printf("[acceptor %d] rejecting prepare - seq too low\n", a.ID)
		return nil
	}

	// make the promise 
	a.PromisedSeq = prepare.Seq

	// send promise back with any previously accepted value
	promise := Message{
		From: a.ID,
		To:   prepare.From,
		Type: MsgPromise,
		Seq:  prepare.Seq,
		Val:  a.AcceptedVal,
	}

	return &promise
}

// handle propose message
func (a *Acceptor) HandlePropose(propose Message) bool {
	// only accept if this matches what was promised 
	if propose.Seq != a.PromisedSeq {
		fmt.Printf("[acceptor %d] rejecting propose - seq doesn't match promise\n", a.ID)
		return false
	}

	// accept the proposal 
	a.AcceptedSeq = propose.Seq
	a.AcceptedVal = propose.Val
	fmt.Printf("[acceptor %d] accepted value '%s'\n", a.ID, a.AcceptedVal)

	return true
}

// learner watches acceptors and figures out consensus 
type Learner struct {
	ID           int
	AcceptedMsgs map[int]Message 
	NT           NodeNetwork
}

// create new learner 
func NewLearner(id int, nt NodeNetwork, acceptorIds ...int) *Learner {
	learner := &Learner{ID: id, NT: nt}
	learner.AcceptedMsgs = make(map[int]Message)

	// initialize tracking for each acceptor 
	for _, acceptorId := range acceptorIds {
		learner.AcceptedMsgs[acceptorId] = Message{}
	}

	fmt.Printf("[learner %d] created, tracking %d acceptors\n", id, len(acceptorIds))
	return learner
}

// run the learner 
func (l *Learner) Run() string {
	fmt.Printf("[learner %d] starting to listen for consensus\n", l.ID)

	for {
		msg := l.NT.Recv()
		if msg == nil {
			continue
		}

		if msg.Type == MsgAccept {
			fmt.Printf("[learner %d] received accept from acceptor %d\n", l.ID, msg.From)
			l.HandleAccept(*msg)

			// check if we have consensus 
			consensusValue, hasConsensus := l.CheckConsensus()
			if hasConsensus {
				fmt.Printf("[learner %d] consensus reached on value '%s'\n", l.ID, consensusValue)
				return consensusValue
			}
		}
	}
}

// handle accept message from acceptor 
func (l *Learner) HandleAccept(accept Message) {
	currentMsg := l.AcceptedMsgs[accept.From]

	// keep the message with highest sequence number from each acceptor 
	if currentMsg.GetProposeSeq() < accept.GetProposeSeq() {
		l.AcceptedMsgs[accept.From] = accept
	}
}

// check if majority agree on a value 
func (l *Learner) CheckConsensus() (string, bool) {
	// count how many acceptors accepted each proposal 
	proposalCounts := make(map[int]int)    // seq -> count
	proposalValues := make(map[int]string) // seq -> value

	for acceptorId, msg := range l.AcceptedMsgs {
		if msg.Seq == 0 {
			continue // this acceptor doesn't have any accepted messages yet 
		}

		proposalCounts[msg.Seq]++
		proposalValues[msg.Seq] = msg.Val

		fmt.Printf("[learner %d] acceptor %d accepted seq %d with value '%s'\n",
			l.ID, acceptorId, msg.Seq, msg.Val)
	}

	// check if any proposal has majority 
	needed := l.Majority()
	for seq, count := range proposalCounts {
		fmt.Printf("[learner %d] proposal seq %d has %d accepts (need %d)\n",
			l.ID, seq, count, needed)

		if count >= needed {
			return proposalValues[seq], true
		}
	}

	return "", false
}

// get number needed for majority 
func (l *Learner) Majority() int {
	return len(l.AcceptedMsgs)/2 + 1
}

// run complete paxos consensus simulation 
func RunConsensus(value string, acceptorCount int) (string, bool) {
	// create node IDs 
	nodeIDs := []int{100} // proposer
	for i := 1; i <= acceptorCount; i++ {
		nodeIDs = append(nodeIDs, i) // acceptors
	}
	nodeIDs = append(nodeIDs, 200) // learner

	// create network with proposer(100), acceptors(1,2,3...), learner(200) 
	net := CreateNetwork(nodeIDs...)

	// create acceptors that will notify learner 200 
	acceptors := make([]*Acceptor, acceptorCount)
	for i := 0; i < acceptorCount; i++ {
		acceptorID := i + 1
		acceptors[i] = NewAcceptor(acceptorID, net.GetNodeNetwork(acceptorID), 200)
		// start acceptors 
		go acceptors[i].Run()
	}

	// create learner 
	acceptorIDs := make([]int, acceptorCount)
	for i := 0; i < acceptorCount; i++ {
		acceptorIDs[i] = i + 1
	}
	learner := NewLearner(200, net.GetNodeNetwork(200), acceptorIDs...)

	// start learner in background and capture result 
	resultChan := make(chan string, 1)
	go func() {
		result := learner.Run()
		resultChan <- result
	}()

	// give everything time to start 
	time.Sleep(100 * time.Millisecond)

	// create and run proposer 
	proposer := NewProposer(100, value, net.GetNodeNetwork(100), acceptorIDs...)
	go proposer.Run()

	// wait for result or timeout 
	select {
	case result := <-resultChan:
		fmt.Printf("consensus_reached: '%s'\n", result)
		return result, true
	case <-time.After(5 * time.Second):
		fmt.Println("timeout: no consensus_reached")
		return "", false
	}
}
