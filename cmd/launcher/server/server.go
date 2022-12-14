package server

import (
	"net"

	"github.com/nomad-software/screensaver/cmd/launcher/input"
	"github.com/nomad-software/screensaver/output"
)

// Server is the main server that receives commands.
type Server struct {
	listener net.Listener
	signals  map[string]chan input.Signal
}

// New creates a new server.
func New(port string) (*Server, error) {
	listen, err := net.Listen("tcp", "localhost:"+port)
	if err != nil {
		return nil, err
	}

	server := &Server{
		listener: listen,
		signals:  make(map[string]chan input.Signal),
	}

	return server, nil
}

// RegisterCommandSignal registers a signal channel to be used when a particular
// command is received.
func (s *Server) RegisterCommandSignal(command string, c chan input.Signal) {
	s.signals[command] = c
}

// Listen starts the server and listens for command.
func (s *Server) Listen() {
	defer s.listener.Close()

	for {
		conn, err := s.listener.Accept()
		output.OnError(err, "server failed")

		go s.handleRequest(conn)
	}
}

// handleRequest handles an individual command.
func (s *Server) handleRequest(conn net.Conn) {
	defer conn.Close()

	buffer := make([]byte, 1024)

	_, err := conn.Read(buffer)
	output.OnError(err, "server failed to read command")

	output.LaunchInfo("command received: %s", buffer)

	for cmd, c := range s.signals {
		if cmd == string(buffer[:len(cmd)]) {
			c <- input.Signal{}
		}
	}
}
