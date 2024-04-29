package kuka

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/golang/geo/r3"
	eki_command "github.com/viam-soleng/viam-kuka/src/ekicommands"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
	"go.viam.com/test"
)

func TestHandleDeviceInfo(t *testing.T) {
	logger := logging.NewTestLogger(t)

	kuka := &kukaArm{
		logger:       logger,
		stateMutex:   sync.Mutex{},
		currentState: state{},
	}

	deviceInfoFunctions := []struct {
		name     string
		testFunc func([]string)
		value    string
	}{
		{name: "robot_name", testFunc: kuka.handleRobotName},
		{name: "serial_number", testFunc: kuka.handleRobotSerialNumber},
		{name: "robot_type", testFunc: kuka.handleRobotType},
		{name: "software_version", testFunc: kuka.handleRobotSoftwareVersion},
		{name: "operating_mode", testFunc: kuka.handleRobotOperatingMode},
	}

	tests := []struct {
		description  string
		expectedData []string
		success      bool
	}{
		{description: "no data", expectedData: []string{}, success: false},
		{description: "incorrect amount of data", expectedData: []string{"gibberish", "gibberish"}, success: false},
		{description: "correct amount of data", expectedData: []string{"data"}, success: true},
	}

	for _, deviceInfoFunc := range deviceInfoFunctions {
		for _, tt := range tests {
			kuka.deviceInfo = deviceInfo{}

			t.Run(fmt.Sprintf("%v given %v", deviceInfoFunc.name, tt.description), func(t *testing.T) {
				deviceInfoFunc.testFunc(tt.expectedData)

				var value string
				switch deviceInfoFunc.name {
				case "robot_name":
					value = kuka.deviceInfo.name
				case "serial_number":
					value = kuka.deviceInfo.serialNum
				case "robot_type":
					value = kuka.deviceInfo.robotType
				case "software_version":
					value = kuka.deviceInfo.softwareVersion
				case "operating_mode":
					value = kuka.deviceInfo.operatingMode
				default:
				}

				if tt.success {
					test.That(t, value, test.ShouldEqual, tt.expectedData[0])
				} else {
					test.That(t, value, test.ShouldEqual, "")
				}
			})
		}
	}
}

func TestHandleJointLimit(t *testing.T) {
	logger := logging.NewTestLogger(t)

	kuka := &kukaArm{
		logger:       logger,
		stateMutex:   sync.Mutex{},
		currentState: state{},
		responseCh:   make(chan bool, 1),
	}

	jointLimitsTests := []struct {
		description string
		data        []string
		success     bool
	}{
		{description: "incorrect amount of data", data: []string{"0", "0", "0"}, success: false},
		{description: "correct amount of data bad format", data: []string{"1", "2", "3", "hi", "0", "0", "0", "0", "0", "0", "0", "0"}, success: false},
		{description: "correct amount of data", data: []string{"1", "2", "3", "0", "0", "0", "0", "0", "0", "0", "0", "0"}, success: true},
	}

	for _, tt := range jointLimitsTests {
		kuka.currentState = state{jointLimits: make([]referenceframe.Limit, numJoints)}

		t.Run(tt.description, func(t *testing.T) {
			kuka.handleMinJointPositions(tt.data)
			kuka.handleMaxJointPositions(tt.data)
			if tt.success {
				expectedMinResult := helperStringListToFloats(tt.data[0:6])
				expectedMaxResult := helperStringListToFloats(tt.data[0:6])
				for i, jointLimit := range kuka.currentState.jointLimits {
					test.That(t, jointLimit.Min, test.ShouldResemble, expectedMinResult[i])
					test.That(t, jointLimit.Max, test.ShouldResemble, expectedMaxResult[i])
				}
			} else {
				test.That(t, kuka.currentState.jointLimits, test.ShouldResemble, make([]referenceframe.Limit, numJoints))
			}
		})
	}
}

func TestHandleJointPosition(t *testing.T) {
	logger := logging.NewTestLogger(t)

	kuka := &kukaArm{
		logger:       logger,
		stateMutex:   sync.Mutex{},
		currentState: state{},
		responseCh:   make(chan bool, 1),
	}

	jointPositionTests := []struct {
		description string
		data        []string
		success     bool
	}{
		{description: "incorrect amount of data", data: []string{"0", "0", "0"}, success: false},
		{description: "correct amount of data bad format", data: []string{"1", "2", "3", "hi", "0", "0", "0", "0", "0", "0", "0", "0"}, success: false},
		{description: "correct amount of data", data: []string{"1", "2", "3", "0", "0", "0", "0", "0", "0", "0", "0", "0"}, success: true},
	}

	for _, tt := range jointPositionTests {
		kuka.currentState = state{}

		t.Run(tt.description, func(t *testing.T) {
			kuka.handleGetJointPositions(tt.data)
			if tt.success {
				expectedResult := helperStringListToFloats(tt.data[0:6])
				test.That(t, kuka.currentState.joints, test.ShouldResemble, expectedResult)
			} else {
				test.That(t, kuka.currentState.joints, test.ShouldBeNil)
			}
		})
	}
}

func TestHandleEndPosition(t *testing.T) {
	logger := logging.NewTestLogger(t)

	kuka := &kukaArm{
		logger:       logger,
		stateMutex:   sync.Mutex{},
		currentState: state{},
		responseCh:   make(chan bool, 1),
	}

	endPositionTests := []struct {
		description string
		data        []string
		success     bool
	}{
		{description: "incorrect amount of data", data: []string{"0", "0", "0"}, success: false},
		{description: "correct amount of data bad format", data: []string{"1", "2", "3", "hi", "0", "0", "0", "0"}, success: false},
		{description: "correct amount of data", data: []string{"1", "2", "3", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0"}, success: true},
	}

	for _, tt := range endPositionTests {
		kuka.currentState = state{}

		t.Run(tt.description, func(t *testing.T) {
			kuka.handleGetEndPositions(tt.data)
			if tt.success {
				dataFloats := helperStringListToFloats(tt.data)
				expectedResult := spatialmath.NewPose(
					r3.Vector{X: dataFloats[0], Y: dataFloats[1], Z: dataFloats[2]},
					&spatialmath.EulerAngles{
						Yaw:   utils.RadToDeg(dataFloats[3]),
						Pitch: utils.RadToDeg(dataFloats[4]),
						Roll:  utils.RadToDeg(dataFloats[5]),
					})
				test.That(t, kuka.currentState.endEffectorPose, test.ShouldResemble, expectedResult)
			} else {
				test.That(t, kuka.currentState.endEffectorPose, test.ShouldBeNil)
			}
		})
	}
}

func TestHandleProgramState(t *testing.T) {
	logger := logging.NewTestLogger(t)

	kuka := &kukaArm{
		logger:       logger,
		stateMutex:   sync.Mutex{},
		currentState: state{},
		responseCh:   make(chan bool, 1),
	}

	programStateTests := []struct {
		description string
		data        []string
		success     bool
	}{
		{description: "incorrect amount of data", data: []string{"0", "0", "0"}, success: false},
		{description: "correct amount of data", data: []string{"ekiMain", "Running"}, success: true},
	}

	for _, tt := range programStateTests {
		kuka.currentState = state{}

		t.Run(tt.description, func(t *testing.T) {
			kuka.handleProgramState(tt.data)
			if tt.success {
				test.That(t, kuka.currentState.programName, test.ShouldResemble, tt.data[0])
				test.That(t, kuka.currentState.programState, test.ShouldResemble, eki_command.StatusRunning)
			} else {
				test.That(t, kuka.currentState.programName, test.ShouldEqual, "")
				test.That(t, kuka.currentState.programState, test.ShouldEqual, 0)
			}
		})
	}
}

func TestHandleSuccess(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	kuka := &kukaArm{
		logger:       logger,
		stateMutex:   sync.Mutex{},
		currentState: state{isMoving: true},
		responseCh:   make(chan bool, 1),
	}

	isMoving, err := kuka.IsMoving(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, isMoving, test.ShouldBeTrue)

	// Send 'success'
	kuka.handleRobotResponses("command", []string{"success"})

	isMoving, err = kuka.IsMoving(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, isMoving, test.ShouldBeFalse)
}

// helperStringListToFloats
func helperStringListToFloats(data []string) []float64 {
	floatList := make([]float64, len(data))
	for i := 0; i < len(data); i++ {
		c, err := strconv.ParseFloat(data[i], 64)
		if err != nil {
			return nil
		}
		floatList[i] = c
	}
	return floatList
}
