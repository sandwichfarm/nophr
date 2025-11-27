package gopher

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
	"github.com/sandwichfarm/nophr/internal/sections"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// Server implements a Gopher protocol server (RFC 1436)
type Server struct {
	config         *config.GopherProtocol
	fullConfig     *config.Config
	storage        *storage.Storage
	router         *Router
	host           string
	queryHelper    *aggregates.QueryHelper
	sectionManager *sections.Manager

	listener net.Listener
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// New creates a new Gopher server
func New(cfg *config.GopherProtocol, fullCfg *config.Config, st *storage.Storage, host string, aggMgr *aggregates.Manager) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		config:      cfg,
		fullConfig:  fullCfg,
		storage:     st,
		host:        host,
		ctx:         ctx,
		cancel:      cancel,
		queryHelper: aggregates.NewQueryHelper(st, fullCfg, aggMgr),
	}

	// Initialize sections manager (opt-in for custom filtered views)
	// Sections are available but not auto-registered
	// Users can configure custom sections via config for filtered views
	s.sectionManager = sections.NewManager(st, fullCfg.Identity.Npub)

	// Initialize router
	s.router = NewRouter(s, host, cfg.Port)

	return s
}

// Start starts the Gopher server
func (s *Server) Start() error {
	// Use Bind field for listening, fallback to Host if Bind not set
	bindAddr := s.config.Bind
	if bindAddr == "" {
		bindAddr = s.config.Host
	}
	addr := fmt.Sprintf("%s:%d", bindAddr, s.config.Port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start Gopher server: %w", err)
	}

	s.listener = listener
	fmt.Printf("Gopher server listening on %s\n", addr)

	// Accept connections in background
	s.wg.Add(1)
	go s.acceptConnections()

	return nil
}

// Stop stops the Gopher server
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

	// Read selector line (terminated by CRLF)
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Read error: %v\n", err)
		return
	}

	// Clean selector (remove CRLF and trim)
	selector := strings.TrimSpace(line)

	// Log request
	fmt.Printf("Gopher request: %q from %s\n", selector, conn.RemoteAddr())

	// Route request
	response := s.router.Route(selector)

	// Write response
	conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	_, err = conn.Write(response)
	if err != nil {
		fmt.Printf("Write error: %v\n", err)
	}
}

// GetStorage returns the storage instance
func (s *Server) GetStorage() *storage.Storage {
	return s.storage
}

// GetConfig returns the config
func (s *Server) GetConfig() *config.GopherProtocol {
	return s.config
}

// GetHost returns the server hostname
func (s *Server) GetHost() string {
	return s.host
}

// GetQueryHelper returns the query helper instance
func (s *Server) GetQueryHelper() *aggregates.QueryHelper {
	return s.queryHelper
}

// GetSectionManager returns the section manager instance
func (s *Server) GetSectionManager() *sections.Manager {
	return s.sectionManager
}
