package kuka

import (
	"sync"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/viam-soleng/viam-kuka/inject"
	eki_command "github.com/viam-soleng/viam-kuka/src/ekicommands"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
)

func TestParseConfig(t *testing.T) {
	logger := logging.NewTestLogger(t)

	conf := resource.Config{
		Name: "testKukaArm",
	}

	kuka := &kukaArm{
		Named:      conf.ResourceName().AsNamed(),
		logger:     logger,
		stateMutex: sync.Mutex{},
		tcpConn:    tcpConn{},
		deviceInfo: deviceInfo{},
	}

	t.Run("IP Address", func(t *testing.T) {
		cfg := &Config{
			IPAddress: "0.0.0.0",
		}
		err := kuka.parseConfig(cfg)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, kuka.tcpConn.ipAddress, test.ShouldEqual, cfg.IPAddress)

		cfg = &Config{}
		err = kuka.parseConfig(cfg)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, kuka.tcpConn.ipAddress, test.ShouldEqual, defaultIPAddress)
	})

	t.Run("Port", func(t *testing.T) {
		cfg := &Config{
			Port: 2,
		}
		err := kuka.parseConfig(cfg)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, kuka.tcpConn.port, test.ShouldEqual, cfg.Port)

		cfg = &Config{}
		err = kuka.parseConfig(cfg)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, kuka.tcpConn.port, test.ShouldEqual, defaultTCPPort)
	})

	t.Run("Model", func(t *testing.T) {
		cfg := &Config{
			SafeMode: true,
		}
		err := kuka.parseConfig(cfg)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, kuka.safeMode, test.ShouldBeTrue)

		cfg = &Config{}
		err = kuka.parseConfig(cfg)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, kuka.safeMode, test.ShouldBeFalse)
	})
}

func TestResetInformation(t *testing.T) {
	kuka := &kukaArm{
		stateMutex: sync.Mutex{},
		deviceInfo: deviceInfo{
			name:            "name",
			robotType:       "type",
			softwareVersion: "software_version",
			serialNum:       "serial_number",
			operatingMode:   "mode",
		},
		currentState: state{
			endEffectorPose: spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3}),
			joints:          []float64{1, 2, 3, 4, 5, 6},
			jointLimits:     []referenceframe.Limit{{Min: 0, Max: 1}, {Min: 2, Max: 3}, {Min: 4, Max: 5}},
			isMoving:        true,
			programState:    eki_command.StatusRunning,
			programName:     "program_name",
		},
	}

	kuka.resetCurrentStateAndDeviceInfo()

	test.That(t, kuka.deviceInfo.name, test.ShouldEqual, "")
	test.That(t, kuka.deviceInfo.robotType, test.ShouldEqual, "")
	test.That(t, kuka.deviceInfo.serialNum, test.ShouldEqual, "")
	test.That(t, kuka.deviceInfo.softwareVersion, test.ShouldEqual, "")
	test.That(t, kuka.deviceInfo.operatingMode, test.ShouldEqual, "")

	test.That(t, kuka.currentState.joints, test.ShouldBeNil)
	test.That(t, kuka.currentState.endEffectorPose, test.ShouldBeNil)
	test.That(t, kuka.currentState.jointLimits, test.ShouldResemble, make([]referenceframe.Limit, numJoints))
	test.That(t, kuka.currentState.isMoving, test.ShouldBeFalse)
	test.That(t, kuka.currentState.programName, test.ShouldEqual, "")
	test.That(t, kuka.currentState.programState, test.ShouldResemble, eki_command.StatusUnknown)
}

func TestCommunication(t *testing.T) {
	logger := logging.NewTestLogger(t)

	kuka := &kukaArm{
		logger: logger,
		tcpConn: tcpConn{
			mu: sync.Mutex{},
		},
	}

	conn := inject.NewTCPConn()

	t.Run("Send Commands", func(t *testing.T) {
		conn.WriteFunc = func(b []byte) (n int, err error) {
			return 0, nil
		}
		kuka.tcpConn.conn = conn

		err := kuka.sendCommand("command", "arguments")
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("Get Device Info", func(t *testing.T) {
		conn.WriteFunc = func(b []byte) (n int, err error) {
			return 0, nil
		}
		kuka.tcpConn.conn = conn

		err := kuka.getDeviceInfo()
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("Check EKI Program State", func(t *testing.T) {
		conn.WriteFunc = func(b []byte) (n int, err error) {
			kuka.currentState.programState = eki_command.StatusRunning
			return 0, nil
		}
		kuka.tcpConn.conn = conn

		status, err := kuka.checkEKIProgramState()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status, test.ShouldResemble, eki_command.StatusRunning)
	})
}

func TestCheckJointLimits(t *testing.T) {
	logger := logging.NewTestLogger(t)

	kuka := &kukaArm{
		logger:     logger,
		stateMutex: sync.Mutex{},
		currentState: state{
			jointLimits: []referenceframe.Limit{
				{Min: 0, Max: 2},
				{Min: 0, Max: 2},
				{Min: 0, Max: 2},
				{Min: 0, Max: 2},
				{Min: 0, Max: 2},
				{Min: 0, Max: 2},
			},
		},
	}

	t.Run("none in range", func(t *testing.T) {
		desiredJointPositions := []float64{-1, -1, -1, -1, -1, -1}
		err := kuka.checkDesiredJointPositions(desiredJointPositions)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "invalid joint position specified")
	})

	t.Run("some in range", func(t *testing.T) {
		desiredJointPositions := []float64{1, 1, -1, 1, 1, 1}
		err := kuka.checkDesiredJointPositions(desiredJointPositions)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "invalid joint position specified")
	})

	t.Run("all in range", func(t *testing.T) {
		desiredJointPositions := []float64{1, 1, 1, 1, 1, 1}
		err := kuka.checkDesiredJointPositions(desiredJointPositions)
		test.That(t, err, test.ShouldBeNil)
	})
}
