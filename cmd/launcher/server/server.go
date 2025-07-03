package server

import (
	"io"
	"net"

	"github.com/nomad-software/screensaver/output"
)

type signal struct{}

// Server is the main server that receives commands.
type Server struct {
	listener net.Listener
	signals  map[string]chan signal
}

// New creates a new server.
func New(port string) (*Server, error) {
	listen, err := net.Listen("tcp", "localhost:"+port)
	if err != nil {
		return nil, err
	}

	server := &Server{
		listener: listen,
		signals:  make(map[string]chan signal),
	}

	return server, nil
}

// CreateSignal registers a signal channel to be used when a particular
// command is received.
func (s *Server) CreateSignal(command string) chan signal {
	c := make(chan signal)
	s.signals[command] = c
	return c
}

// Listen starts the server and listens for command.
func (s *Server) Listen() {
	go func() {
		defer s.listener.Close()

		for {
			conn, err := s.listener.Accept()
			output.OnError(err, "server listening failed")

			go s.handleRequest(conn)
		}
	}()
}

// handleRequest handles an individual command.
func (s *Server) handleRequest(conn net.Conn) {
	defer conn.Close()

	buffer := make([]byte, 1024)

	_, err := conn.Read(buffer)
	// If the error is 'end of file' it basically means no command was sent from
	// a client, so ignore it.
	if err == io.EOF {
		return
	}

	output.OnError(err, "server failed to read command")
	output.LaunchInfo("command received: %s", buffer)

	for cmd, c := range s.signals {
		if cmd == string(buffer[:len(cmd)]) {
			c <- signal{}
		}
	}
}
