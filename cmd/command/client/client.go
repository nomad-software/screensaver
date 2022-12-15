package client

import (
	"net"

	"github.com/nomad-software/screensaver/output"
)

// Client is the client that sends commands to the launcher.
type Client struct {
	conn *net.TCPConn
}

// New creates a new client.
func New(port string) (*Client, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:"+port)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return nil, err
	}

	c := &Client{
		conn: conn,
	}

	return c, nil
}

// Send sends a command to the launcher.
func (c *Client) Send(cmd string) {
	_, err := c.conn.Write([]byte(cmd))
	output.OnError(err, "failed to send command to launcher")
}
