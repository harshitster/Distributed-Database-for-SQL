package raft

import (
	"context"
	"time"

	"github.com/harshitster/Distributed-Database-for-SQL/src/pb"
	"google.golang.org/grpc"
)

func (r *RaftNode) replicateLog(peerID int, addr string) {
	r.mu.Lock()

	term := r.term
	commitLength := r.commitIndex
	nextIdx := r.nextIndex[peerID]

	var entries []*pb.LogEntry
	for i := nextIdx; i < len(r.log); i++ {
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

	r.mu.Unlock()

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return
	}
	defer conn.Close()

	client := pb.NewRaftClient(conn)

	request := &pb.LogRequest{
		LeaderID:     int32(r.id),
		Term:         int32(term),
		LogLength:    int32(nextIdx),
		LogTerm:      int32(prevLogTerm),
		LeaderCommit: int32(commitLength),
		Entries:      entries,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	resp, err := client.AppendEntries(ctx, request)
	if err != nil {
		return
	}

	r.handleAppendEntriesResponse(resp, peerID, addr)
}

func (r *RaftNode) handleAppendEntriesResponse(resp *pb.LogResponse, followerID int, addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if int(resp.Term) > r.term {
		r.term = int(resp.Term)
		r.state = Follower
		r.votedFor = nil
		return
	}

	if r.term != int(resp.Term) || r.state != Leader {
		return
	}

	if resp.Success && int(resp.Ack) >= r.ackedLength[followerID] {
		r.nextIndex[followerID] = int(resp.Ack)
		r.matchIndex[followerID] = int(resp.Ack) - 1
		r.ackedLength[followerID] = int(resp.Ack)
		r.commitLogEntries()
	} else if r.nextIndex[followerID] > 0 {
		r.nextIndex[followerID]--
		go r.replicateLog(followerID, addr)
	}
}

func (r *RaftNode) commitLogEntries() {
	for i := r.commitIndex + 1; i <= len(r.log); i++ {
		count := 1
		for peerID := range r.peers {
			if peerID != r.id && r.ackedLength[peerID] >= i {
				count++
			}
		}
		if count > len(r.peers)/2 && r.log[i-1].Term == r.term {
			for j := r.commitIndex + 1; j <= i; j++ {
				r.applyCh <- r.log[j-1].Command
			}
			r.commitIndex = i
		}
	}
}
