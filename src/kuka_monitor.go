package kuka

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/geo/r3"
	ekiCommand "github.com/viam-soleng/viam-kuka/src/ekicommands"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"

	gutils "go.viam.com/utils"
)

// startResponseMonitor starts up a background process to monitor responses from the TCP connection.
func (kuka *kukaArm) startResponseMonitor() error {

	kuka.activeBackgroundWorkers.Add(1)
	gutils.PanicCapturingGo(func() {
		defer kuka.activeBackgroundWorkers.Done()

		kuka.responseMonitor()
	})
	return nil
}

// responseMonitor monitors the responses from the TCP connection and sends them to the associated handler.
func (kuka *kukaArm) responseMonitor() {
	for {
		if kuka.closed {
			fmt.Println("closed")
			break
		}

		// Read response
		data, err := kuka.Read()
		if err != nil {
			kuka.logger.Warnf("error reading line: %v", err)
		}
		if data == nil {
			continue
		}

		// Handle response data
		dataList := strings.Split(string(data[:len(data)-1]), ",")
		kuka.handleRobotResponses(dataList[0], dataList[1:])
	}
}

// handleRobotResponses calls the associated handler function for each possible command
func (kuka *kukaArm) handleRobotResponses(command string, args []string) {

	// Check for success status
	if len(args) > 0 {
		if args[0] == "success" {
			kuka.stateMutex.Lock()
			// Send response to channel if a move request is being processed
			if kuka.currentState.isMoving {
				kuka.responseCh <- true
			}

			kuka.currentState.isMoving = false
			kuka.stateMutex.Unlock()
			return
		}
	}

	// Handle responses to commands
	switch command {
	// Get robot info
	case ekiCommand.GetRobotName:
		kuka.handleRobotName(args)
	case ekiCommand.GetRobotSerialNum:
		kuka.handleRobotSerialNumber(args)
	case ekiCommand.GetRobotType:
		kuka.handleRobotType(args)
	case ekiCommand.GetRobotSoftwareVersion:
		kuka.handleRobotSoftwareVersion(args)
	case ekiCommand.GetRobotOperatingMode:
		kuka.handleRobotOperatingMode(args)
	case ekiCommand.GetEKIProgramState:
		kuka.handleProgramState(args)
	// Get robot status
	case ekiCommand.GetJointPosition:
		kuka.handleGetJointPositions(args)
	case ekiCommand.GetEndPosition:
		kuka.handleGetEndPositions(args)
	case ekiCommand.GetJointNegLimit:
		kuka.handleMinJointPositions(args)
	case ekiCommand.GetJointPosLimit:
		kuka.handleMaxJointPositions(args)
	// Get response from move
	case ekiCommand.SetJointPosition:
		kuka.handleSetJointPositions(args)
	default:
		kuka.logger.Infof("UNHANDLED RESPONSE: %v", args)
	}
}

// Get robot info
func (kuka *kukaArm) handleRobotName(data []string) {
	if len(data) != 1 {
		kuka.logger.Warnf("incorrect amount of data returned for robot name: %v  (should be 1)", data)
		return
	}

	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	kuka.deviceInfo.name = data[0]
}

func (kuka *kukaArm) handleRobotSerialNumber(data []string) {
	if len(data) != 1 {
		kuka.logger.Warnf("incorrect amount of data returned for robot serial number: %v  (should be 1)", data)
		return
	}

	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	kuka.deviceInfo.serialNum = data[0]
}

func (kuka *kukaArm) handleRobotType(data []string) {
	if len(data) != 1 {
		kuka.logger.Warnf("incorrect amount of data returned for robot type: %v  (should be 1)", data)
		return
	}

	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	kuka.deviceInfo.robotType = data[0]
}

func (kuka *kukaArm) handleRobotSoftwareVersion(data []string) {
	if len(data) != 1 {
		kuka.logger.Warnf("incorrect amount of data returned for robot software version mode: %v  (should be 1)", data)
		return
	}

	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	kuka.deviceInfo.softwareVersion = data[0]
}

func (kuka *kukaArm) handleRobotOperatingMode(data []string) {
	if len(data) != 1 {
		kuka.logger.Warnf("incorrect amount of data returned for robot operating mode: %v  (should be 1)", data)
		return
	}

	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	kuka.deviceInfo.operatingMode = data[0]
}

// Get robot status
func (kuka *kukaArm) handleMinJointPositions(data []string) {
	if len(data) != numJoints+numExternalJoints {
		kuka.logger.Warnf("incorrect amount of data returned for negative joint position limits: %v  (should be 12)", data)
		return
	}

	// Parse data and update current state
	jointLimits := make([]referenceframe.Limit, numJoints)
	for i := 0; i < numJoints; i++ {
		val, err := strconv.ParseFloat(data[i], 64)
		if err != nil {
			kuka.logger.Warnf("issue parsing response to floats, failed to parse %v", data)
			return
		}
		jointLimits[i].Min = val
	}

	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	for i := range jointLimits {
		kuka.currentState.jointLimits[i].Min = jointLimits[i].Min
	}
}

func (kuka *kukaArm) handleMaxJointPositions(data []string) {
	if len(data) != numJoints+numExternalJoints {
		kuka.logger.Warnf("incorrect amount of data returned for positive joint position limits: %v  (should be 12)", data)
		return
	}

	// Parse data and update current state
	jointLimits := make([]referenceframe.Limit, numJoints)
	for i := 0; i < numJoints; i++ {
		val, err := strconv.ParseFloat(data[i], 64)
		if err != nil {
			kuka.logger.Warnf("issue parsing response to floats, failed to parse %v", data)
			return
		}
		jointLimits[i].Max = val
	}

	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	for i := range jointLimits {
		kuka.currentState.jointLimits[i].Max = jointLimits[i].Max
	}
}

// handleGetJointPositions is blocking
func (kuka *kukaArm) handleGetJointPositions(data []string) {
	if len(data) != numJoints+numExternalJoints {
		kuka.logger.Warnf("incorrect amount of data returned for joint position: %v (should be 12)", data)
		return
	}

	// Parse values to floats
	jointList := make([]float64, numJoints)
	for i := 0; i < numJoints; i++ {
		c, err := strconv.ParseFloat(data[i], 64)
		if err != nil {
			kuka.logger.Warnf("issue parsing response to floats, failed to parse %v", data)
			return
		}
		jointList[i] = c
	}

	// Update current state
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	kuka.currentState.joints = jointList
}

func (kuka *kukaArm) handleGetEndPositions(data []string) {
	if len(data) != 8+numExternalJoints {
		kuka.logger.Warnf("incorrect amount of data returned for end position limits: %v (should be 14)", data)
		return
	}

	// Parse values to floats
	endPositionList := make([]float64, 8)
	for i := 0; i < numJoints; i++ {
		c, err := strconv.ParseFloat(data[i], 64)
		if err != nil {
			kuka.logger.Warnf("issue parsing response to floats, failed to parse %v", data)
			return
		}
		endPositionList[i] = c
	}

	// Update current state
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	kuka.currentState.endEffectorPose = spatialmath.NewPose(
		r3.Vector{X: endPositionList[0], Y: endPositionList[1], Z: endPositionList[2]},
		&spatialmath.EulerAngles{
			Yaw:   utils.RadToDeg(endPositionList[3]),
			Pitch: utils.RadToDeg(endPositionList[4]),
			Roll:  utils.RadToDeg(endPositionList[5]),
		},
	)
}

// handleProgramState is blocking
func (kuka *kukaArm) handleProgramState(data []string) {
	if len(data) != 2 {
		kuka.logger.Warnf("incorrect amount of data returned for robot programming state: %v (should be 2)", data)
		return
	}

	// Update current state
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	kuka.currentState.programName = data[0]
	kuka.currentState.programState = ekiCommand.StringToProgramStatus(data[1])

	kuka.responseCh <- false
}

// Set
func (kuka *kukaArm) handleSetJointPositions(data []string) {
	// Nothing to do currently
}
