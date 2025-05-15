package raft

import (
	"context"
	"time"

	"github.com/harshitster/Distributed-Database-for-SQL/src/pb"
	"google.golang.org/grpc"
)

func (r *RaftNode) startElection() {
	r.mu.Lock()

	r.state = Candidate
	r.term += 1
	r.votedFor = &r.id
	r.votesReceived = map[int]bool{r.id: true}
	currentTerm := r.term
	logLength := len(r.log)
	lastLogTerm := 0
	if logLength > 0 {
		lastLogTerm = r.log[logLength-1].Term
	}
	r.electionResetEvent = time.Now()

	r.mu.Unlock()

	for peerID, addr := range r.peers {
		if peerID == r.id {
			continue
		}

		go func(peerID int, addr string) {
			conn, err := grpc.Dial(addr, grpc.WithInsecure())
			if err != nil {
				return
			}
			defer conn.Close()

			client := pb.NewRaftClient(conn)

			request := &pb.VoteRequest{
				NodeId:    int32(r.id),
				Term:      int32(currentTerm),
				LogLength: int32(logLength),
				LogTerm:   int32(lastLogTerm),
			}

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			resp, err := client.RequestVote(ctx, request)
			if err != nil {
				return
			}

			r.handleVoteResponse(resp)
		}(peerID, addr)
	}
}

func (r *RaftNode) handleVoteResponse(resp *pb.VoteResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if int(resp.Term) > r.term {
		r.term = int(resp.Term)
		r.state = Follower
		r.votedFor = nil
		return
	}

	if r.state != Candidate || r.term != int(resp.Term) {
		return
	}

	if resp.VoteGranted {
		if !r.votesReceived[int(resp.NodeID)] {
			r.votesReceived[int(resp.NodeID)] = true
			if len(r.votesReceived) > len(r.peers)/2 {
				r.becomeLeader()
			}
		}
	}
}

func (r *RaftNode) becomeLeader() {
	r.state = Leader
	r.leaderID = &r.id
	r.nextIndex = make(map[int]int)
	r.matchIndex = make(map[int]int)
	r.ackedLength = make(map[int]int)

	lastLogIndex := len(r.log)
	for peerID := range r.peers {
		r.nextIndex[peerID] = lastLogIndex
		r.matchIndex[peerID] = 0
		r.ackedLength[peerID] = 0
	}

	r.ackedLength[r.id] = lastLogIndex

	go r.sendHeartBeats()
}

func (r *RaftNode) sendHeartBeats() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for peerID, addr := range r.peers {
		if peerID == r.id {
			continue
		}

		conn, err := grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			return
		}
		defer conn.Close()

		client := pb.NewRaftClient(conn)

		nextIdx := r.nextIndex[peerID]
		var entries []*pb.LogEntry
		for i := nextIdx; i < len(r.log); i += 1 {
			entries = append(entries, &pb.LogEntry{
				Term:    int32(r.log[i].Term),
				Command: r.log[i].Command,
			})
		}

		prevLogIndex := nextIdx - 1
		prevLogTerm := 0
		if prevLogIndex >= 0 && prevLogIndex < len(r.log) {
			prevLogTerm = r.log[prevLogIndex].Term
		}

		req := &pb.LogRequest{
			LeaderID:     int32(r.id),
			Term:         int32(r.term),
			LogLength:    int32(nextIdx),
			LogTerm:      int32(prevLogTerm),
			LeaderCommit: int32(r.commitIndex),
			Entries:      entries,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		_, _ = client.AppendEntries(ctx, req)
	}
}
