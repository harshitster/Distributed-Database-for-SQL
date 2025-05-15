package raft

import (
	"context"

	"github.com/harshitster/Distributed-Database-for-SQL/src/pb"
)

func (r *RaftNode) RequestVote(ctx context.Context, request *pb.VoteRequest) (*pb.VoteResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cID := int(request.NodeId)
	cTerm := int(request.Term)
	cLogLength := int(request.LogLength)
	cLogTerm := int(request.LogTerm)

	lastIndex := len(r.log) - 1
	lastTerm := 0
	if lastIndex >= 0 {
		lastTerm = r.log[lastIndex].Term
	}

	logOk := (cLogTerm > lastTerm) || (cLogTerm == lastTerm && cLogLength >= len(r.log))
	termOk := (cTerm > r.term) || (cTerm == r.term && (r.votedFor == nil || *r.votedFor == cID))

	if cTerm > r.term {
		r.term = cTerm
		r.state = Follower
		r.votedFor = nil
	}

	resp := &pb.VoteResponse{
		NodeID:      int32(r.id),
		Term:        int32(r.term),
		VoteGranted: false,
	}

	if logOk && termOk {
		r.votedFor = &cID
		resp.VoteGranted = true
	}

	return resp, nil
}

func (r *RaftNode) AppendEntries(ctx context.Context, req *pb.LogRequest) (*pb.LogResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	leaderID := int(req.LeaderID)
	leaderTerm := int(req.Term)

	resp := &pb.LogResponse{
		FollowerID: int32(r.id),
		Term:       int32(r.term),
		Ack:        int32(0),
		Success:    false,
	}

	if leaderTerm < r.term {
		return resp, nil
	}

	if leaderTerm > r.term {
		r.term = leaderTerm
		r.state = Follower
		r.leaderID = &leaderID
		r.votedFor = nil
	}

	if r.term == leaderTerm && r.state == Candidate {
		r.state = Follower
		r.leaderID = &leaderID
	}

	prevLogIndex := int(req.LogLength) - 1
	logOk := (len(r.log) >= int(req.LogLength)) &&
		(int(req.LogLength) == 0 || r.log[prevLogIndex].Term == int(req.LogTerm))

	if int(req.LogTerm) == r.term && logOk {
		if len(req.Entries) > 0 && len(r.log) > int(req.LogLength) {
			if r.log[int(req.LogLength)].Term != int(req.Entries[0].Term) {
				r.log = r.log[:int(req.LogLength)]
			}
		}

		start := len(r.log) - int(req.LogLength)
		if start < len(req.Entries) {
			for i := start; i < len(req.Entries); i++ {
				r.log = append(r.log, LogEntry{
					Term:    int(req.Entries[i].Term),
					Command: req.Entries[i].Command,
				})
			}
		}

		leaderCommit := int(req.LeaderCommit)
		if leaderCommit > r.commitIndex {
			lastNewIndex := len(r.log)
			newCommitIndex := leaderCommit

			if leaderCommit > lastNewIndex {
				newCommitIndex = lastNewIndex
			}

			for i := r.commitIndex + 1; i <= newCommitIndex; i++ {
				r.applyCh <- r.log[i-1].Command
			}
			r.commitIndex = newCommitIndex
		}

		ack := int(req.LogLength) + len(req.Entries)
		resp.Ack = int32(ack)
		resp.Success = true

		return resp, nil
	} else {
		return resp, nil
	}
}
