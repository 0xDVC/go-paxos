package main

import "fmt"

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

func main() {
	// TODO: placeholder to for the logic
	msg := message{
		from: 1,
		to:   2,
		typ:  Prepare,
		seq:  1,
		val:  "test_value",
	}

	fmt.Printf("created message: from=%d, to=%d, type=%d, seq=%d, val=%s\n",
		msg.from, msg.to, msg.typ, msg.seq, msg.val)

}
