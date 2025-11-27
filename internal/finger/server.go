package finger

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/sandwichfarm/nophr/internal/aggregates"
	"github.com/sandwichfarm/nophr/internal/config"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// Server implements a Finger protocol server (RFC 1288)
type Server struct {
	config      *config.FingerProtocol
	storage     *storage.Storage
	handler     *Handler
	queryHelper *aggregates.QueryHelper
	ownerPubkey string

	listener net.Listener
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// New creates a new Finger server
func New(cfg *config.FingerProtocol, fullCfg *config.Config, st *storage.Storage, aggMgr *aggregates.Manager) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		config:      cfg,
		storage:     st,
		ownerPubkey: fullCfg.Identity.Npub,
		ctx:         ctx,
		cancel:      cancel,
		queryHelper: aggregates.NewQueryHelper(st, fullCfg, aggMgr),
	}

	// Initialize handler
	s.handler = NewHandler(s, fullCfg)

	return s
}

// Start starts the Finger server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Bind, s.config.Port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start Finger server: %w", err)
	}

	s.listener = listener
	fmt.Printf("Finger server listening on %s\n", addr)

	// Accept connections in background
	s.wg.Add(1)
	go s.acceptConnections()

	return nil
}

// Stop stops the Finger server
func (s *Server) Stop() error {
	s.cancel()

	if s.listener != nil {
		s.listener.Close()
	}

	s.wg.Wait()
	return nil
}

// acceptConnections accepts and handles incoming connections
func (s *Server) acceptConnections() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				fmt.Printf("Accept error: %v\n", err)
				continue
			}
		}

		// Handle connection in goroutine
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection handles a single client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	// Set read timeout
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	// Read query line (terminated by CRLF)
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		s.sendResponse(conn, "Error reading query\r\n")
		return
	}

	// Clean query (remove CRLF and trim)
	query := strings.TrimSpace(line)

	// Log request
	fmt.Printf("Finger request: %q from %s\n", query, conn.RemoteAddr())

	// Handle query
	response := s.handler.Handle(query)

	// Write response
	conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	s.sendResponse(conn, response)
}

// sendResponse sends a response and ensures proper formatting
func (s *Server) sendResponse(conn net.Conn, response string) {
	// Ensure CRLF line endings per RFC 1288
	response = strings.ReplaceAll(response, "\n", "\r\n")
	conn.Write([]byte(response))
}

// GetStorage returns the storage instance
func (s *Server) GetStorage() *storage.Storage {
	return s.storage
}

// GetConfig returns the config
func (s *Server) GetConfig() *config.FingerProtocol {
	return s.config
}

// GetQueryHelper returns the query helper instance
func (s *Server) GetQueryHelper() *aggregates.QueryHelper {
	return s.queryHelper
}

// GetOwnerPubkey returns the owner's pubkey
func (s *Server) GetOwnerPubkey() string {
	return s.ownerPubkey
}
