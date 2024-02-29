package kuka

import (
	"context"
	"fmt"
	"net"
	"time"
)

var (
	connectionTimeout time.Duration = 5 * time.Second
)

// Connect to the Kuka Arm via TCP dialer
func (kuka *kukaArm) Connect(ctx context.Context) error {

	if kuka.conn != nil {
		if err := kuka.Disconnect(); err != nil {
			return err
		}
	}

	ctx, ctxCancel := context.WithTimeout(ctx, connectionTimeout)
	defer ctxCancel()

	// Dial the tcp server at the given (or default) address
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf(":%v", kuka.tcp_port))
	if err != nil {
		return err
	}
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
	if _, err := kuka.conn.Write(command); err != nil {
		return err
	}

	return nil
}
