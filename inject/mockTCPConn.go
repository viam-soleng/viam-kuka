package inject

import (
	"net"
	"time"
)

// SLAMService represents a fake instance of a slam service.
type TCPConn struct {
	net.Conn
	ReadFunc            func(b []byte) (n int, err error)
	WriteFunc           func(b []byte) (n int, err error)
	CloseFunc           func() error
	SetReadDeadlineFunc func(t time.Time) error
}

// NewSLAMService returns a new injected SLAM service.
func NewTCPConn() *TCPConn {
	return &TCPConn{}
}

func (conn *TCPConn) Read(b []byte) (n int, err error) {
	if conn.ReadFunc == nil {
		return conn.Read(b)
	}
	return conn.ReadFunc(b)
}

func (conn *TCPConn) Write(b []byte) (n int, err error) {
	if conn.WriteFunc == nil {
		return conn.Write(b)
	}
	return conn.WriteFunc(b)
}

func (conn *TCPConn) SetReadDeadline(t time.Time) error {
	if conn.SetReadDeadlineFunc == nil {
		return conn.SetReadDeadline(t)
	}
	return conn.SetReadDeadlineFunc(t)
}

func (conn *TCPConn) Close() error {
	if conn.CloseFunc == nil {
		return conn.Close()
	}
	return conn.CloseFunc()
}
