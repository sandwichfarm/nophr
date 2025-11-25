package gemini

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/sandwich/nophr/internal/aggregates"
	"github.com/sandwich/nophr/internal/config"
	"github.com/sandwich/nophr/internal/sections"
	"github.com/sandwich/nophr/internal/storage"
)

// Server implements a Gemini protocol server
type Server struct {
	config         *config.GeminiProtocol
	fullConfig     *config.Config
	storage        *storage.Storage
	router         *Router
	host           string
	queryHelper    *aggregates.QueryHelper
	sectionManager *sections.Manager
	tlsConfig      *tls.Config

	listener net.Listener
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// New creates a new Gemini server
func New(cfg *config.GeminiProtocol, fullCfg *config.Config, st *storage.Storage, host string, aggMgr *aggregates.Manager) (*Server, error) {
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
	s.sectionManager = sections.NewManager(st, fullCfg.Identity.Npub)

	// Initialize TLS configuration
	if err := s.initTLS(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize TLS: %w", err)
	}

	// Initialize router
	s.router = NewRouter(s, host, cfg.Port)

	return s, nil
}

// Start starts the Gemini server
func (s *Server) Start() error {
	// Use Bind field for listening, fallback to Host if Bind not set
	bindAddr := s.config.Bind
	if bindAddr == "" {
		bindAddr = s.config.Host
	}
	addr := fmt.Sprintf("%s:%d", bindAddr, s.config.Port)

	listener, err := tls.Listen("tcp", addr, s.tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to start Gemini server: %w", err)
	}

	s.listener = listener
	fmt.Printf("Gemini server listening on %s\n", addr)

	// Accept connections in background
	s.wg.Add(1)
	go s.acceptConnections()

	return nil
}

// Stop stops the Gemini server
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

	// Read request line (URI + CRLF, max 1024 bytes)
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		s.sendResponse(conn, StatusBadRequest, "Error reading request", "")
		return
	}

	// Clean request (remove CRLF and trim)
	request := strings.TrimSpace(line)

	// Validate request length (max 1024 bytes per spec)
	if len(request) > 1024 {
		s.sendResponse(conn, StatusBadRequest, "Request too long", "")
		return
	}

	// Parse URL
	parsedURL, err := url.Parse(request)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		s.sendResponse(conn, StatusBadRequest, "Invalid URL", "")
		return
	}

	// Validate scheme
	if parsedURL.Scheme != "gemini" {
		s.sendResponse(conn, StatusProxyRequestRefused, "Only gemini:// URLs supported", "")
		return
	}

	// Log request
	fmt.Printf("Gemini request: %s from %s\n", request, conn.RemoteAddr())

	// Route request
	response := s.router.Route(parsedURL)

	// Write response
	conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	_, err = conn.Write(response)
	if err != nil {
		fmt.Printf("Write error: %v\n", err)
	}
}

// sendResponse sends a Gemini response
func (s *Server) sendResponse(conn net.Conn, status Status, meta string, body string) {
	response := FormatResponse(status, meta, body)
	conn.Write(response)
}

// GetStorage returns the storage instance
func (s *Server) GetStorage() *storage.Storage {
	return s.storage
}

// GetConfig returns the config
func (s *Server) GetConfig() *config.GeminiProtocol {
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
