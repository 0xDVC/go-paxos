package main

import (
	"fmt"
	"log"
	"time"
)

// so much to learn, so much to do.
// TODO: write a blog about this whole shit
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

// NEW: Acceptor implementation
// This is my attempt at implementing the acceptor logic
// TODO: Still figuring out exactly how the promise/accept logic should work
type acceptor struct {
	id          int
	promisedSeq int    // highest sequence number I've promised
	acceptedSeq int    // sequence number of value I've accepted
	acceptedVal string // the actual value I've accepted
	nt          nodeNetwork
}

func NewAcceptor(id int, nt nodeNetwork) *acceptor {
	return &acceptor{
		id:          id,
		nt:          nt,
		promisedSeq: 0, // start with no promises
		acceptedSeq: 0, // start with no accepted values
	}
}

// Main acceptor loop - handle incoming messages
// TODO: Need to handle both Prepare and Propose messages
func (a *acceptor) run() {
	log.Printf("Acceptor %d starting...", a.id)

	for {
		msg := a.nt.recv()
		if msg == nil {
			continue // timeout, keep waiting
		}

		log.Printf("Acceptor %d received message type %d", a.id, msg.typ)

		switch msg.typ {
		case Prepare:
			response := a.handlePrepare(*msg)
			if response != nil {
				a.nt.send(*response)
			}
		case Propose:
			accepted := a.handlePropose(*msg)
			log.Printf("Acceptor %d proposal result: %t", a.id, accepted)
			// TODO: If accepted, need to notify learners
		default:
			log.Printf("Acceptor %d: Unknown message type %d", a.id, msg.typ)
		}
	}
}

// Handle prepare message - core Paxos logic
// TODO: This is my understanding of the promise logic, hope it's right
func (a *acceptor) handlePrepare(prepare message) *message {
	log.Printf("Acceptor %d handling prepare with seq %d (current promised: %d)",
		a.id, prepare.seq, a.promisedSeq)

	// Only promise if this sequence number is higher than what I've seen
	if prepare.seq <= a.promisedSeq {
		log.Printf("Acceptor %d rejecting prepare - seq too low", a.id)
		return nil // ignore lower sequence numbers
	}

	// Make the promise
	a.promisedSeq = prepare.seq
	log.Printf("Acceptor %d promising seq %d", a.id, prepare.seq)

	// Send promise back to proposer
	promise := message{
		from: a.id,
		to:   prepare.from,
		typ:  Promise,
		seq:  prepare.seq,
		val:  a.acceptedVal, // tell proposer what I previously accepted (if anything)
	}

	return &promise
}

// Handle propose message
// TODO: Only accept if it matches my promise
func (a *acceptor) handlePropose(propose message) bool {
	log.Printf("Acceptor %d handling propose with seq %d (promised: %d)",
		a.id, propose.seq, a.promisedSeq)

	// Only accept if this matches what I promised
	if propose.seq != a.promisedSeq {
		log.Printf("Acceptor %d rejecting propose - doesn't match promise", a.id)
		return false
	}

	// Accept the proposal
	a.acceptedSeq = propose.seq
	a.acceptedVal = propose.val
	log.Printf("Acceptor %d accepted value: %s", a.id, a.acceptedVal)

	return true
}

func main() {
	// Create network with proposer (100) and acceptors (1, 2, 3)
	net := CreateNetwork(100, 1, 2, 3)

	// Create acceptors
	acceptor1 := NewAcceptor(1, net.getNodeNetwork(1))
	acceptor2 := NewAcceptor(2, net.getNodeNetwork(2))
	acceptor3 := NewAcceptor(3, net.getNodeNetwork(3))

	// Start acceptors in goroutines
	go acceptor1.run()
	go acceptor2.run()
	go acceptor3.run()

	// Give acceptors time to start
	time.Sleep(100 * time.Millisecond)

	// Create and run proposer
	proposer := NewProposer(100, "test_consensus_value", net.getNodeNetwork(100), 1, 2, 3)

	// Run proposer in goroutine so main doesn't block
	go proposer.run()

	// Let it run for a bit
	time.Sleep(5 * time.Second)

}
