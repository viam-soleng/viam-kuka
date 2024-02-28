package kuka

import (
	"fmt"
	"net"
)

// Connect to the Kuka Arm via TCP dialer
func (kuka *kukaArm) Connect() error {

	if kuka.conn != nil {
		if err := kuka.Disconnect(); err != nil {
			return err
		}
	}

	// Dial the tcp server at the given (or default) address
	conn, err := net.Dial("tcp", fmt.Sprintf(":%v", kuka.tcp_port))
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
