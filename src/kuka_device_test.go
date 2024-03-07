package kuka

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/viam-soleng/viam-kuka/inject"
	"go.viam.com/rdk/logging"
	"go.viam.com/test"
)

func TestConnect(t *testing.T) {

	// conn := inject.NewTCPConn()
	// test.That(t, conn, test.ShouldBeNil)
}

func TestRead(t *testing.T) {
	logger := logging.NewTestLogger(t)

	kuka := &kukaArm{
		logger:   logger,
		tcpMutex: &sync.Mutex{},
	}

	conn := inject.NewTCPConn()
	conn.SetReadDeadlineFunc = func(t time.Time) error {
		return nil
	}

	t.Run("valid Read function", func(t *testing.T) {
		conn.ReadFunc = func(b []byte) (n int, err error) {
			b = []byte("test")
			return len(b), nil
		}
		kuka.conn = conn

		response, err := kuka.Read()
		test.That(t, err, test.ShouldBeNil)
		fmt.Println("response: ", string(response))
	})

	t.Run("valid Read function with timeout", func(t *testing.T) {
		conn.ReadFunc = func(b []byte) (n int, err error) {
			//net.Error.Timeout()
			return 0, nil //netErr.Timeout()
		}
		kuka.conn = conn

		response, err := kuka.Read()
		test.That(t, err, test.ShouldBeNil)
		fmt.Println("response: ", string(response))
	})

	t.Run("invalid read function", func(t *testing.T) {
		conn.ReadFunc = func(b []byte) (n int, err error) {
			return 0, errors.New("error")
		}
		kuka.conn = conn

		response, err := kuka.Read()
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldEqual, "error")
		test.That(t, response, test.ShouldBeNil)
	})
}

func TestWrite(t *testing.T) {
	logger := logging.NewTestLogger(t)

	kuka := &kukaArm{
		logger:   logger,
		tcpMutex: &sync.Mutex{},
	}

	conn := inject.NewTCPConn()

	t.Run("valid Write function", func(t *testing.T) {
		conn.WriteFunc = func(b []byte) (n int, err error) {
			return len(b), nil
		}
		kuka.conn = conn

		err := kuka.Write([]byte("test"))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("invalid Write function", func(t *testing.T) {
		conn.WriteFunc = func(b []byte) (n int, err error) {
			return 0, errors.New("error")
		}
		kuka.conn = conn

		err := kuka.Write([]byte("test"))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldEqual, "error")
	})
}

func TestDisconnect(t *testing.T) {
	logger := logging.NewTestLogger(t)

	kuka := &kukaArm{
		logger:   logger,
		tcpMutex: &sync.Mutex{},
	}

	conn := inject.NewTCPConn()

	t.Run("valid Close function", func(t *testing.T) {
		conn.CloseFunc = func() error {
			return nil
		}
		kuka.conn = conn

		err := kuka.Disconnect()
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("invalid Close function", func(t *testing.T) {
		conn.CloseFunc = func() error {
			return errors.New("error")
		}
		kuka.conn = conn

		err := kuka.Disconnect()
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldEqual, "error")
	})

	kuka.Disconnect()

}
