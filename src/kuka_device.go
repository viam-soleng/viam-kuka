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
	if kuka.conn != nil { // we should check this out of the state structure if we can, I'll look
		if err := kuka.Disconnect(); err != nil {
			return err
		}
	}

	// Attempt to dial the TCP server
	ctx, ctxCancel := context.WithTimeout(ctx, connectionTimeout)
	defer ctxCancel()

	// I noticed in their large pdf that they have a streaming server avaialble, it seemed to indicate that you could 
	// connect to it directly, without running the program on the interface machine, I will re-check.
	var d net.Dialer
	address := fmt.Sprintf("%v:%v", kuka.ip_address, kuka.tcp_port)
	conn, err := d.DialContext(ctx, "tcp", address) // might need a keepalive.
	if err != nil {
		return err
	}

	kuka.logger.Infof("Connected to device at %v", address)
	kuka.conn = conn

	return nil
}

// Disconnect TCP dialer
func (kuka *kukaArm) Disconnect() error {
	if err := kuka.conn.Close(); err != nil {
		return err
	}

	return nil
}

// Write to TCP dialer
func (kuka *kukaArm) Write(command []byte) error {
	kuka.tcpMutex.Lock()
	defer kuka.tcpMutex.Unlock()

	kuka.logger.Debugf("Sending command: %v", string(command))
	if _, err := kuka.conn.Write(command); err != nil {
		return err
	}

	return nil
}

// Read from TCP dailer
func (kuka *kukaArm) Read() ([]byte, error) {
	kuka.tcpMutex.Lock()
	defer kuka.tcpMutex.Unlock()

	kuka.conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	recv := make([]byte, defaultReadBufSize)
	n, err := kuka.conn.Read(recv)
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
