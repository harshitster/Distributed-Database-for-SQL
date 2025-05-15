package raft

import (
	"sync"
	"time"
)

type State string

const (
	Follower  State = "Follower"
	Candidate State = "Candidate"
	Leader    State = "Leader"
)

type LogEntry struct {
	Term    int
	Command string
}

type RaftNode struct {
	mu                 sync.Mutex
	id                 int
	peers              map[int]string
	state              State
	term               int
	votedFor           *int
	log                []LogEntry
	commitIndex        int
	lastApplied        int
	nextIndex          map[int]int
	matchIndex         map[int]int
	ackedLength        map[int]int
	leaderID           *int
	applyCh            chan string
	votesReceived      map[int]bool
	electionResetEvent time.Time
}

func NewRaftNode(id int, peers map[int]string, applyCh chan string) *RaftNode {
	return &RaftNode{
		id:                 id,
		peers:              peers,
		state:              Follower,
		term:               0,
		votedFor:           nil,
		log:                []LogEntry{},
		commitIndex:        0,
		lastApplied:        0,
		nextIndex:          make(map[int]int),
		matchIndex:         make(map[int]int),
		ackedLength:        make(map[int]int),
		leaderID:           nil,
		applyCh:            applyCh,
		votesReceived:      make(map[int]bool),
		electionResetEvent: time.Now(),
	}
}
