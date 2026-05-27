package ipc

import (
	"context"
	"errors"
	"sync"

	"github.com/yuu-th/projwm/internal/next/store"
)

type Method string

const (
	MethodMutateDesiredWorld Method = "mutateDesiredWorld"
	MethodReadWorld          Method = "readWorld"
	MethodEventHint          Method = "eventHint"
)

type ErrorKind string

const (
	ErrorSocketAbsent      ErrorKind = "socket-absent"
	ErrorConnectionRefused ErrorKind = "connection-refused"
	ErrorTimeout           ErrorKind = "timeout"
	ErrorDaemonBusy        ErrorKind = "daemon-busy"
	ErrorProtocolMismatch  ErrorKind = "protocol-mismatch"
	ErrorIntentRejected    ErrorKind = "intent-rejected"
	ErrorTransactionFailed ErrorKind = "transaction-failed"
	ErrorUnsupported       ErrorKind = "unsupported"
)

type Handshake struct {
	ProtocolVersion              int
	DaemonVersion                string
	ManagedEnvironmentGeneration string
	StoreSchemaVersion           int
}

type Request struct {
	Method             Method
	ExpectedGeneration store.Generation
	Actor              string
	Direct             bool
	Handshake          Handshake
}

type Response struct {
	Generation store.Generation
}

type GenerationStore interface {
	Current() (store.Generation, error)
	BeginCommit(expected store.Generation) (*store.Commit, error)
}

type Server struct {
	mu        sync.Mutex
	store     GenerationStore
	handshake Handshake
}

var (
	ErrUnknownMethod    = errors.New("unknown method")
	ErrDirectMutation   = errors.New("direct mutation is not allowed")
	ErrSidecarWrite     = errors.New("sidecar cannot mutate")
	ErrProtocolMismatch = errors.New("protocol mismatch")
)

func NewServer(s GenerationStore) *Server {
	return &Server{
		store: s,
		handshake: Handshake{
			ProtocolVersion:              1,
			DaemonVersion:                "projwmd-next-test",
			ManagedEnvironmentGeneration: "env-test",
			StoreSchemaVersion:           1,
		},
	}
}

func (s *Server) Handle(_ context.Context, req Request) (Response, error) {
	if err := s.validateHandshake(req.Handshake); err != nil {
		return Response{}, err
	}
	if req.Direct && req.Method == MethodMutateDesiredWorld {
		return Response{}, ErrDirectMutation
	}
	if req.Actor == "sidecar" && req.Method != MethodEventHint {
		return Response{}, ErrSidecarWrite
	}
	switch req.Method {
	case MethodReadWorld:
		g, err := s.store.Current()
		return Response{Generation: g}, err
	case MethodEventHint:
		g, err := s.store.Current()
		return Response{Generation: g}, err
	case MethodMutateDesiredWorld:
		s.mu.Lock()
		defer s.mu.Unlock()
		c, err := s.store.BeginCommit(req.ExpectedGeneration)
		if err != nil {
			return Response{}, err
		}
		for _, artifact := range []string{
			"desired_world.json",
			"accepted_layout.json",
			"browser_snapshot.json",
			"checkpoint.json",
			"journal.jsonl",
		} {
			if err := c.WriteArtifact(artifact, map[string]string{"mutatedBy": req.Actor, "artifact": artifact}); err != nil {
				_ = c.Abort()
				return Response{}, err
			}
		}
		next, err := c.Commit("ipc-intent")
		if err != nil {
			return Response{}, err
		}
		return Response{Generation: next}, nil
	default:
		return Response{}, ErrUnknownMethod
	}
}

func (s *Server) validateHandshake(h Handshake) error {
	if h.ProtocolVersion != s.handshake.ProtocolVersion ||
		h.ManagedEnvironmentGeneration != s.handshake.ManagedEnvironmentGeneration ||
		h.StoreSchemaVersion != s.handshake.StoreSchemaVersion {
		return ErrProtocolMismatch
	}
	return nil
}
