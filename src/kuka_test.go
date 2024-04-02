package kuka

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/viam-soleng/viam-kuka/inject"
	eki_command "github.com/viam-soleng/viam-kuka/src/ekicommands"
	v1 "go.viam.com/api/component/arm/v1"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/referenceframe/urdf"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
)

func TestGetterEndpoints(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	expectedPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3})
	expectedJoints := []float64{0, 1, 2, 3, 4, 5}

	urdfModel, err := urdf.ParseModelXMLFile(resolveFile(fmt.Sprintf("src/models/%v.urdf", kr10r900)), "test")
	test.That(t, err, test.ShouldBeNil)

	kuka := &kukaArm{
		logger:     logger,
		stateMutex: sync.Mutex{},
		currentState: state{
			joints:          expectedJoints,
			endEffectorPose: expectedPose,
		},
		model: urdfModel,
	}

	t.Run("joint positions", func(t *testing.T) {
		joints, err := kuka.JointPositions(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, joints.Values, test.ShouldResemble, expectedJoints)
	})

	t.Run("end positions", func(t *testing.T) {
		pose, err := kuka.EndPosition(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pose, test.ShouldResemble, expectedPose)
	})

	t.Run("is moving", func(t *testing.T) {
		isMoving, err := kuka.IsMoving(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isMoving, test.ShouldBeFalse)

		kuka.currentState.isMoving = true
		isMoving, err = kuka.IsMoving(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isMoving, test.ShouldBeTrue)
	})
}

func TestSetterEndpoints(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	kuka := &kukaArm{
		logger:     logger,
		stateMutex: sync.Mutex{},
		currentState: state{
			joints: make([]float64, numJoints),
			jointLimits: []referenceframe.Limit{
				{Min: 0, Max: 100},
				{Min: 0, Max: 100},
				{Min: 0, Max: 100},
				{Min: 0, Max: 100},
				{Min: 0, Max: 100},
				{Min: 0, Max: 100},
			},
		},
		responseCh: make(chan bool, 1),
	}

	conn := inject.NewTCPConn()

	t.Run("outside joint limits", func(t *testing.T) {
		expectedJoints := []float64{-1, 1, 2, 3, 4, 5}

		conn.WriteFunc = func(b []byte) (n int, err error) {
			kuka.currentState.joints = expectedJoints
			kuka.currentState.programState = eki_command.StatusRunning
			kuka.currentState.isMoving = false
			return 0, nil
		}

		kuka.tcpConn.conn = conn

		err := kuka.MoveToJointPositions(ctx, &v1.JointPositions{Values: expectedJoints}, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "invalid joint position specified")
	})

	t.Run("successful", func(t *testing.T) {
		expectedJoints := []float64{1, 1, 2, 3, 4, 5}
		var trackInt int

		conn.WriteFunc = func(b []byte) (n int, err error) {
			// Update required info when 5 commands come through
			if trackInt == 4 {
				kuka.currentState.joints = expectedJoints
				kuka.currentState.programState = eki_command.StatusRunning
				kuka.currentState.isMoving = false
				kuka.responseCh <- true
			}
			trackInt++
			return 0, nil
		}

		kuka.tcpConn.conn = conn

		err := kuka.MoveToJointPositions(ctx, &v1.JointPositions{Values: expectedJoints}, nil)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("successful safemode", func(t *testing.T) {
		kuka.safeMode = true
		expectedJoints := []float64{1, 1, 2, 3, 4, 5}
		var trackInt int

		conn.WriteFunc = func(b []byte) (n int, err error) {
			// Update required info when the ProgramState command comes through
			if trackInt == 0 {
				kuka.currentState.programState = eki_command.StatusRunning
				trackInt++
				kuka.responseCh <- true
				return 0, nil
			}
			// Update required info after 3 more commands have come through
			if trackInt == 4 {
				kuka.currentState.joints = expectedJoints
				kuka.currentState.isMoving = false
				kuka.responseCh <- true
			}
			trackInt++
			return 0, nil
		}

		kuka.tcpConn.conn = conn

		err := kuka.MoveToJointPositions(ctx, &v1.JointPositions{Values: expectedJoints}, nil)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("is still moving", func(t *testing.T) {
		expectedJoints := []float64{1, 1, 2, 3, 4, 5}
		kuka.currentState.isMoving = true

		err := kuka.MoveToJointPositions(ctx, &v1.JointPositions{Values: expectedJoints}, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "robot is still moving")
	})
}
