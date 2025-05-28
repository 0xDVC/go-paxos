package main

import (
	"fmt"
	"time"
)

// TODO: figure out what message types Paxos actually needs
type msgType int

const (
	Prepare msgType = iota + 1 // proposer asks acceptors for permission
	Promise                    // acceptor promises to proposer
	Propose                    // proposer sends actual value
	Accept                     // acceptor accepts the value
)

// TODO: might need to add more fields as I understand the algorithm better
type message struct {
	from int
	to   int
	typ  msgType
	seq  int
	val  string
}

// TODO: work on these helper methods later
func (m *message) getProposeVal() string {
	return m.val
}

func (m *message) getProposeSeq() int {
	return m.seq
}

// TODO: expand this simulation later on
type network struct {
	recvQueue map[int]chan message // each node recievs its own message
}

func CreateNetwork(nodes ...int) *network {
	nt := network{recvQueue: make(map[int]chan message, 0)}

	for _, node := range nodes {
		nt.recvQueue[node] = make(chan message, 1024) //big enough and safe buffer size tbf
	}

	return &nt
}

// send message to specific node
// TODO: add error handling for invalid node IDs
func (n *network) sendTo(m message) {
	fmt.Printf("sending message from %d to %d, type %d, value: %s", m.from, m.to, m.typ, m.val)
	n.recvQueue[m.to] <- m
}

// receive message for specific node with timeout
// TODO: timeout set to 1 sec, might have to come back to this though, works for now
func (n *network) recvFrom(id int) *message {
	select {
	case retMsg := <-n.recvQueue[id]:
		fmt.Printf("node %d received message from %d, type %d, value: %s", id, retMsg.from, retMsg.typ, retMsg.val)
		return &retMsg
	case <-time.After(time.Second):
		return nil //timeout here
	}
}

type nodeNetwork struct {
	id  int
	net *network
}

func (n *network) getNodeNetwork(id int) nodeNetwork {
	return nodeNetwork{id: id, net: n}
}

func (n *nodeNetwork) send(m message) {
	n.net.sendTo(m)
}

func (n *nodeNetwork) recv() *message {
	return n.net.recvFrom(n.id)
}

// magic happens here
type proposer struct {
	id         int
	seq        int
	proposeNum int
	proposeVal string
	acceptors  map[int]message
	nt         nodeNetwork
}

func NewProposer(id int, val string, nt nodeNetwork, acceptorIds ...int) *proposer {
	pro := proposer{id: id, proposeVal: val, seq: 0, nt: nt}
	pro.acceptors = make(map[int]message, len(acceptorIds))

	fmt.Printf("creating proposer %d, val '%s', acceptors %d", id, pro.proposeVal, len(acceptorIds))

	for _, acceptorId := range acceptorIds {
		pro.acceptors[acceptorId] = message{}
	}
	return &pro
}

func (p *proposer) run() {
	fmt.Printf("proposer %d, value: %s", p.id, p.proposeVal)

	for !p.majorityReached() {
		fmt.Println("phase0: sending prepare messages...")

		prepareMessages := p.prepare()
		for _, msg := range prepareMessages {
			p.nt.send(msg)
		}

		// wait for promise responses
		// TODO: Should I wait for multiple responses or just one at a time?
		promise := p.nt.recv()
		if promise == nil {
			fmt.Println("no response recvd, retrying...")
			continue
		}

		if promise.typ == Promise {
			fmt.Printf("got promise from acceptor %d", promise.from)
			p.handlePromise(*promise)
		}
	}

	fmt.Println("phase1: majority reached, sending propose messages...")
	proposeMessages := p.propose()
	for _, msg := range proposeMessages {
		p.nt.send(msg)
	}

	fmt.Printf("proposer %d finished proposing value: %s", p.id, p.proposeVal)
}

func (p *proposer) prepare() []message {
	p.seq++

	var msgList []message
	sendCount := 0

	for acceptorId := range p.acceptors {
		if sendCount >= p.majority() {
			break
		}

		msg := message{
			from: p.id,
			to:   acceptorId,
			typ:  Prepare,
			seq:  p.getProposeNum(),
			val:  p.proposeVal,
		}
		msgList = append(msgList, msg)
		sendCount++
	}

	fmt.Printf("prepared %d prepare messages", len(msgList))
	return msgList
}

// Generate propose messages for Phase 2
// TODO: Only send to acceptors that promised?
func (p *proposer) propose() []message {
	var msgList []message
	sendCount := 0

	for acceptorId, acceptorMsg := range p.acceptors {
		// Only propose to acceptors that promised with current sequence
		if acceptorMsg.getProposeSeq() == p.getProposeNum() {
			msg := message{
				from: p.id,
				to:   acceptorId,
				typ:  Propose,
				seq:  p.getProposeNum(),
				val:  p.proposeVal,
			}
			msgList = append(msgList, msg)
			sendCount++

			if sendCount >= p.majority() {
				break
			}
		}
	}

	return msgList
}

// handle promise from acceptor
// TODO: Need to understand what to do if acceptor has already accepted something else
func (p *proposer) handlePromise(promise message) {
	fmt.Printf("Processing promise from acceptor %d", promise.from)
	p.acceptors[promise.from] = promise
}

// Check if we have majority of promises
// TODO: Make sure this logic is correct
func (p *proposer) majorityReached() bool {
	promiseCount := 0
	currentProposeNum := p.getProposeNum()

	for _, acceptorMsg := range p.acceptors {
		if acceptorMsg.getProposeSeq() == currentProposeNum {
			promiseCount++
		}
	}

	needed := p.majority()
	fmt.Printf("Promise count: %d, majority needed: %d", promiseCount, needed)
	return promiseCount >= needed
}

func (p *proposer) majority() int {
	return len(p.acceptors)/2 + 1
}

// TODO: Need to ensure these are unique across all proposers
// Using bit shifting trick I saw in examples: seq << 4 | id
func (p *proposer) getProposeNum() int {
	return p.seq<<4 | p.id
}

func main() {
	//msg := message{
	//	from: 1,
	//	to:   2,
	//	typ:  Prepare,
	//	seq:  1,
	//	val:  "test_value",
	//}

	//fmt.Printf("created message: from=%d, to=%d, type=%d, seq=%d, val=%s\n",
	//	msg.from, msg.to, msg.typ, msg.seq, msg.val)

	// create network with 3 nodes
	net := CreateNetwork(100, 1, 2, 3)

	//node1 := net.getNodeNetwork(1)
	//node2 := net.getNodeNetwork(2)

	//testMsg := message{from: 1, to: 2, typ: Prepare, seq: 1, val: "hello"}
	//node1.send(testMsg)

	//receivedMsg := node2.recv()
	//if receivedMsg != nil {
	//	fmt.Printf("received: %+v\n", *receivedMsg)
	//} else {
	//	fmt.Println("no msg received")
	//}

	proposer := NewProposer(100, "my_test_value", net.getNodeNetwork(100), 1, 2, 3)

	fmt.Printf("Created proposer with ID %d\n", proposer.id)
	fmt.Printf("Proposal number generation test: %d\n", proposer.getProposeNum())

	// Test prepare message generation
	prepareMessages := proposer.prepare()
	fmt.Printf("Generated %d prepare messages\n", len(prepareMessages))

	for i, msg := range prepareMessages {
		fmt.Printf("Prepare message %d: %+v\n", i, msg)
	}

}
