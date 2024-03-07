package kuka

import (
	"context"
	"fmt"
	"net"
	"time"
)

const (
	defaultReadBufSize int = 8192
)

var (
	connectionTimeout time.Duration = 5 * time.Second
)

// Connect to the Kuka Arm via TCP dialer
func (kuka *kukaArm) Connect(ctx context.Context) error {

	// Close any prior connections
	if kuka.tcpConn.conn != nil {
		if err := kuka.Disconnect(); err != nil {
			return err
		}
	}

	// Attempt to dial the TCP server
	ctx, ctxCancel := context.WithTimeout(ctx, connectionTimeout)
	defer ctxCancel()

	var d net.Dialer
	address := fmt.Sprintf("%v:%v", kuka.tcpConn.ipAddress, kuka.tcpConn.port)
	conn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}

	kuka.logger.Infof("Connected to device at %v", address)
	kuka.tcpConn.conn = conn

	return nil
}

// Disconnect TCP dialer
func (kuka *kukaArm) Disconnect() error {
	if err := kuka.tcpConn.conn.Close(); err != nil {
		return err
	}

	return nil
}

// Write to TCP dialer
func (kuka *kukaArm) Write(command []byte) error {
	kuka.tcpConn.mu.Lock()
	defer kuka.tcpConn.mu.Unlock()
	kuka.logger.Debugf("Sending command: %v", string(command))
	if _, err := kuka.tcpConn.conn.Write(command); err != nil {
		return err
	}

	return nil
}

// Read from TCP dialer
func (kuka *kukaArm) Read() ([]byte, error) {
	kuka.tcpConn.mu.Lock()
	defer kuka.tcpConn.mu.Unlock()

	kuka.tcpConn.conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	recv := make([]byte, defaultReadBufSize)
	n, err := kuka.tcpConn.conn.Read(recv)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, nil
		}
		return nil, err
	}

	response := recv[:n]
	kuka.logger.Debugf("Received response: %v", string(response))

	return response, nil
}
