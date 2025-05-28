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
	net := CreateNetwork(1, 2, 3)

	node1 := net.getNodeNetwork(1)
	node2 := net.getNodeNetwork(2)

	testMsg := message{from: 1, to: 2, typ: Prepare, seq: 1, val: "hello"}
	node1.send(testMsg)

	receivedMsg := node2.recv()
	if receivedMsg != nil {
		fmt.Printf("received: %+v\n", *receivedMsg)
	} else {
		fmt.Println("no msg received")
	}
}
