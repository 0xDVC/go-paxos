package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/0xdvc/go-paxos/paxos"
)

// node type in paxos system 
type NodeType string

const (
	ProposerType NodeType = "proposer"
	AcceptorType NodeType = "acceptor"
	LearnerType  NodeType = "learner"
)


type MessageType int

const (
	PrepareMessage MessageType = iota + 1
	PromiseMessage
	ProposeMessage
	AcceptMessage
)

type Message struct {
	ID       int         `json:"id"`
	From     int         `json:"from"`
	To       int         `json:"to"`
	Type     MessageType `json:"type"`
	Seq      int         `json:"seq"`
	Val      string      `json:"val"`
	FromType NodeType    `json:"fromType"`
	ToType   NodeType    `json:"toType"`
}

type Node struct {
	ID    int      `json:"id"`
	Type  NodeType `json:"type"`
	Label string   `json:"label"`
	Value string   `json:"value,omitempty"`
	State string   `json:"state"`
	X     float64  `json:"x"`
	Y     float64  `json:"y"`
}

type PaxosState struct {
	Nodes            []Node    `json:"nodes"`
	Messages         []Message `json:"messages"`
	Phase            string    `json:"phase"`
	Consensus        string    `json:"consensus,omitempty"`
	PrepareCount     int       `json:"prepareCount"`
	PromiseCount     int       `json:"promiseCount"`
	ProposeCount     int       `json:"proposeCount"`
	AcceptCount      int       `json:"acceptCount"`
	EventLog         []string  `json:"eventLog"`
	ConsensusVal     string    `json:"consensusVal,omitempty"`
	MessageIdCounter int       `json:"messageIdCounter"`
}

type Server struct {
	state      PaxosState
	mutex      sync.Mutex
	network    *paxos.Network
	proposer   *paxos.Proposer
	acceptors  []*paxos.Acceptor
	learner    *paxos.Learner
	simRunning bool
	logChan    chan string
}

func NewServer() *Server {
	server := &Server{
		state: PaxosState{
			Nodes:            make([]Node, 0),
			Messages:         make([]Message, 0),
			Phase:            "Idle",
			PrepareCount:     0,
			PromiseCount:     0,
			ProposeCount:     0,
			AcceptCount:      0,
			EventLog:         make([]string, 0),
			MessageIdCounter: 0,
		},
		logChan: make(chan string, 100),
	}
	go server.handleLogs()
	return server
}

// handleLogs processes log messages and adds them to the event log
func (s *Server) handleLogs() {
	for msg := range s.logChan {
		s.mutex.Lock()
		if len(s.state.EventLog) >= 50 {
			s.state.EventLog = s.state.EventLog[1:]
		}
		s.state.EventLog = append(s.state.EventLog, msg)
		s.mutex.Unlock()
	}
}

// addLog adds a log message to the log channel
func (s *Server) addLog(msg string) {
	select {
	case s.logChan <- msg:
	default:
		log.Println("log channel full:", msg)
	}
}

// initializeSystem sets up the Paxos system
func (s *Server) initializeSystem(proposerVal string, acceptorCount int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.state.Nodes = make([]Node, 0)
	s.state.Messages = make([]Message, 0)
	s.state.Phase = "idle"
	s.state.Consensus = ""
	s.state.PrepareCount = 0
	s.state.PromiseCount = 0
	s.state.ProposeCount = 0
	s.state.AcceptCount = 0
	s.state.EventLog = make([]string, 0)
	s.state.MessageIdCounter = 0
	s.state.ConsensusVal = ""
	nodeIDs := []int{100} // proposer
	for i := 1; i <= acceptorCount; i++ {
		nodeIDs = append(nodeIDs, i) // acceptors
	}
	nodeIDs = append(nodeIDs, 200) // learner
	net := paxos.CreateNetwork(nodeIDs...)
	s.network = net
	width := 1000.0
	height := 600.0
	s.state.Nodes = append(s.state.Nodes, Node{
		ID:    100,
		Type:  ProposerType,
		Label: "Proposer",
		Value: proposerVal,
		State: "idle",
		X:     width * 0.15,
		Y:     height * 0.5,
	})
	for i := 0; i < acceptorCount; i++ {
		acceptorID := i + 1
		yPos := height * 0.2
		if acceptorCount > 1 {
			yPos = height*0.2 + (height*0.6*float64(i))/float64(acceptorCount-1)
		}
		s.state.Nodes = append(s.state.Nodes, Node{
			ID:    acceptorID,
			Type:  AcceptorType,
			Label: fmt.Sprintf("Acceptor %d", acceptorID),
			Value: "",
			State: "idle",
			X:     width * 0.5,
			Y:     yPos,
		})
	}
	s.state.Nodes = append(s.state.Nodes, Node{
		ID:    200,
		Type:  LearnerType,
		Label: "Learner",
		Value: "",
		State: "idle",
		X:     width * 0.85,
		Y:     height * 0.5,
	})
	s.acceptors = make([]*paxos.Acceptor, acceptorCount)
	for i := 0; i < acceptorCount; i++ {
		acceptorID := i + 1
		s.acceptors[i] = paxos.NewAcceptor(acceptorID, net.GetNodeNetwork(acceptorID), 200)
	}
	acceptorIDs := make([]int, acceptorCount)
	for i := 0; i < acceptorCount; i++ {
		acceptorIDs[i] = i + 1
	}
	s.learner = paxos.NewLearner(200, net.GetNodeNetwork(200), acceptorIDs...)
	s.proposer = paxos.NewProposer(100, proposerVal, net.GetNodeNetwork(100), acceptorIDs...)
	s.addLog("System initialized")
}

// runAcceptor handles acceptor operation and state updates
func (s *Server) runAcceptor(a *paxos.Acceptor) {
	for {
		if !s.simRunning {
			return
		}
		msg := a.NT.Recv()
		if msg == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		switch msg.Type {
		case paxos.MsgPrepare:
			s.addLog(fmt.Sprintf("Acceptor %d: received PREPARE seq=%d", a.ID, msg.Seq))
			s.mutex.Lock()
			for i := range s.state.Nodes {
				if s.state.Nodes[i].ID == a.ID && s.state.Nodes[i].Type == AcceptorType {
					s.state.Nodes[i].State = "promised"
				}
			}
			s.mutex.Unlock()
			response := a.HandlePrepare(*msg)
			if response != nil {
				time.Sleep(200 * time.Millisecond)
				s.mutex.Lock()
				s.state.PromiseCount++
				s.state.MessageIdCounter++
				s.state.Messages = append(s.state.Messages, Message{
					ID:       s.state.MessageIdCounter,
					From:     a.ID,
					To:       msg.From,
					Type:     PromiseMessage,
					Seq:      response.Seq,
					Val:      response.Val,
					FromType: AcceptorType,
					ToType:   ProposerType,
				})
				s.mutex.Unlock()
				a.NT.Send(*response)
				s.addLog(fmt.Sprintf("Acceptor %d: sent PROMISE for seq=%d", a.ID, response.Seq))
			}
		case paxos.MsgPropose:
			s.addLog(fmt.Sprintf("Acceptor %d: received PROPOSE seq=%d val=%s", a.ID, msg.Seq, msg.Val))
			s.mutex.Lock()
			for i := range s.state.Nodes {
				if s.state.Nodes[i].ID == a.ID && s.state.Nodes[i].Type == AcceptorType {
					s.state.Nodes[i].State = "proposing"
				}
			}
			s.mutex.Unlock()
			if a.HandlePropose(*msg) {
				time.Sleep(200 * time.Millisecond)
				s.mutex.Lock()
				for i := range s.state.Nodes {
					if s.state.Nodes[i].ID == a.ID && s.state.Nodes[i].Type == AcceptorType {
						s.state.Nodes[i].Value = msg.Val
						s.state.Nodes[i].State = "accepted"
					}
				}
				s.mutex.Unlock()
				s.addLog(fmt.Sprintf("Acceptor %d: accepted value '%s'", a.ID, msg.Val))
				for _, learnerId := range a.Learners {
					acceptMsg := paxos.Message{
						From: a.ID,
						To:   learnerId,
						Type: paxos.MsgAccept,
						Seq:  msg.Seq,
						Val:  msg.Val,
					}
					time.Sleep(200 * time.Millisecond)
					s.mutex.Lock()
					s.state.AcceptCount++
					s.state.MessageIdCounter++
					s.state.Messages = append(s.state.Messages, Message{
						ID:       s.state.MessageIdCounter,
						From:     a.ID,
						To:       learnerId,
						Type:     AcceptMessage,
						Seq:      msg.Seq,
						Val:      msg.Val,
						FromType: AcceptorType,
						ToType:   LearnerType,
					})
					s.mutex.Unlock()
					a.NT.Send(acceptMsg)
					s.addLog(fmt.Sprintf("Acceptor %d: notified learner %d", a.ID, learnerId))
				}
			}
		}
	}
}

// runLearner handles learner operation and consensus detection
func (s *Server) runLearner() {
	resultChan := make(chan string, 1)
	go func() {
		result := s.learner.Run()
		resultChan <- result
	}()
	select {
	case result := <-resultChan:
		s.mutex.Lock()
		s.state.Consensus = "Reached"
		s.state.ConsensusVal = result
		s.state.Phase = "Consensus Reached"
		for i := range s.state.Nodes {
			if s.state.Nodes[i].ID == 200 && s.state.Nodes[i].Type == LearnerType {
				s.state.Nodes[i].Value = result
				s.state.Nodes[i].State = "consensus"
			}
		}
		s.mutex.Unlock()
		s.addLog(fmt.Sprintf("CONSENSUS REACHED on value: '%s'", result))
	case <-time.After(30 * time.Second):
		s.addLog("TIMEOUT: No consensus reached")
	}
	s.mutex.Lock()
	s.simRunning = false
	s.mutex.Unlock()
}

// runProposer handles proposer operation
func (s *Server) runProposer() {
	s.mutex.Lock()
	s.state.Phase = "Phase 1: Prepare"
	for i := range s.state.Nodes {
		if s.state.Nodes[i].ID == 100 && s.state.Nodes[i].Type == ProposerType {
			s.state.Nodes[i].State = "preparing"
		}
	}
	s.mutex.Unlock()
	s.addLog(fmt.Sprintf("Proposer %d starting with value: %s", s.proposer.ID, s.proposer.ProposeVal))
	for !s.proposer.MajorityReached() {
		if !s.simRunning {
			return
		}
		prepareMessages := s.proposer.Prepare()
		s.mutex.Lock()
		for _, msg := range prepareMessages {
			s.state.PrepareCount++
			s.state.MessageIdCounter++
			s.state.Messages = append(s.state.Messages, Message{
				ID:       s.state.MessageIdCounter,
				From:     msg.From,
				To:       msg.To,
				Type:     PrepareMessage,
				Seq:      msg.Seq,
				Val:      msg.Val,
				FromType: ProposerType,
				ToType:   AcceptorType,
			})
		}
		s.mutex.Unlock()
		for _, msg := range prepareMessages {
			s.proposer.NT.Send(msg)
			s.addLog(fmt.Sprintf("Proposer %d: sent PREPARE seq=%d to %d", s.proposer.ID, msg.Seq, msg.To))
			time.Sleep(200 * time.Millisecond)
		}
		promise := s.proposer.NT.Recv()
		if promise == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if promise.Type == paxos.MsgPromise {
			s.addLog(fmt.Sprintf("Proposer %d: received PROMISE from %d", s.proposer.ID, promise.From))
			s.proposer.HandlePromise(*promise)
			if s.proposer.MajorityReached() {
				s.mutex.Lock()
				s.state.Phase = "Phase 2: Propose"
				s.mutex.Unlock()
				s.addLog(fmt.Sprintf("Proposer %d: received majority of promises", s.proposer.ID))
			}
		}
	}
	proposeMessages := s.proposer.Propose()
	s.mutex.Lock()
	for _, msg := range proposeMessages {
		s.state.ProposeCount++
		s.state.MessageIdCounter++
		s.state.Messages = append(s.state.Messages, Message{
			ID:       s.state.MessageIdCounter,
			From:     msg.From,
			To:       msg.To,
			Type:     ProposeMessage,
			Seq:      msg.Seq,
			Val:      msg.Val,
			FromType: ProposerType,
			ToType:   AcceptorType,
		})
	}
	s.mutex.Unlock()
	for _, msg := range proposeMessages {
		s.proposer.NT.Send(msg)
		s.addLog(fmt.Sprintf("Proposer %d: sent PROPOSE seq=%d val=%s to %d", s.proposer.ID, msg.Seq, msg.Val, msg.To))
		time.Sleep(200 * time.Millisecond)
	}
}

// RunPaxos runs a complete Paxos consensus algorithm directly
func (s *Server) RunPaxos(value string, acceptorCount int) (string, bool) {
	result, ok := paxos.RunConsensus(value, acceptorCount)
	return result, ok
}

// startSimulation starts the Paxos simulation
func (s *Server) startSimulation(proposerVal string, acceptorCount int) {
	s.mutex.Lock()
	if s.simRunning {
		s.addLog("Simulation already running, ignoring start request")
		s.mutex.Unlock()
		return
	}
	s.simRunning = true
	s.mutex.Unlock()
	s.initializeSystem(proposerVal, acceptorCount)
	s.addLog("=== PAXOS SIMULATION STARTED ===")

	for _, acceptor := range s.acceptors {
		go s.runAcceptor(acceptor)
	}

	go s.runLearner()
	time.Sleep(100 * time.Millisecond)
	go s.runProposer()
}

// stopSimulation stops the current simulation
func (s *Server) stopSimulation() {
	s.mutex.Lock()
	if !s.simRunning {
		s.mutex.Unlock()
		return
	}
	s.simRunning = false
	if s.network != nil {
		for _, ch := range s.network.RecvQueue {
			close(ch)
		}
	}
	s.network = nil
	s.proposer = nil
	s.acceptors = nil
	s.learner = nil
	s.mutex.Unlock()
	s.addLog("Simulation stopped")
}

// getState returns the current state of the Paxos system
func (s *Server) getState() PaxosState {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.state
}

// HTTP Handlers
// HandleGetState handles GET /api/state
func (s *Server) HandleGetState(w http.ResponseWriter, r *http.Request) {
	state := s.getState()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(state)
}

// HandleStartSimulation handles POST /api/start
func (s *Server) HandleStartSimulation(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Value         string `json:"value"`
		AcceptorCount int    `json:"acceptorCount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	go s.startSimulation(req.Value, req.AcceptorCount)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "started",
	})
}

// HandleStopSimulation handles POST /api/stop
func (s *Server) HandleStopSimulation(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.stopSimulation()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "stopped",
	})
}

func (s *Server) HandleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.WriteHeader(http.StatusOK)
}

// ServeHTTP starts the HTTP server
func (s *Server) ServeHTTP(addr string) error {
	// API endpoints
	http.HandleFunc("/api/state", s.HandleGetState)
	http.HandleFunc("/api/start", s.HandleStartSimulation)
	http.HandleFunc("/api/stop", s.HandleStopSimulation)
	http.HandleFunc("/api/options", s.HandleOptions)
	// Serve static files for the web UI
	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)
	fmt.Printf("Server listening on %s\n", addr)
	return http.ListenAndServe(addr, nil)
}
