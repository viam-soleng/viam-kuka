package kuka

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func (kuka *kukaArm) startResponseMonitor() error {
	kuka.activeBackgroundWorkers.Add(1)
	go func() {
		defer kuka.activeBackgroundWorkers.Done()

		kuka.responseMonitor()
	}()
	return nil
}

func (kuka *kukaArm) responseMonitor() {

	for {
		if kuka.closed {
			kuka.logger.Info("closing......")
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

		// Extract command and arguments from response
		dataList := strings.Split(string(data[:len(data)-1]), ",")
		dataCommand := dataList[0]
		dataArgs := dataList[1:]

		// Check for success status
		if len(dataArgs) > 0 {
			if dataArgs[0] == "success" {
				kuka.setIsMovingSafe(false)
				continue
			}
		}

		// Handle responses to commands
		switch dataCommand {
		// Get robot info
		case getRobotNameEKICommand:
			kuka.handleRobotName(dataArgs)
		case getRobotSerialNumEKICommand:
			kuka.handleRobotSerialNumber(dataArgs)
		case getRobotTypeEKICommand:
			kuka.handleRobotType(dataArgs)
		case getRobotSoftwareVersionEKICommand:
			kuka.handleRobotSoftwareVersion(dataArgs)
		case getOperatingModeEKICommand:
			kuka.handleRobotOperatingMode(dataArgs)
		case getEKIProgramState:
			kuka.handleProgramState(dataArgs)
		// Get robot status
		case getJointPositionEKICommand:
			kuka.handleGetJointPositions(dataArgs)
		case getEndPositionEKICommand:
			kuka.handleGetEndPositions(dataArgs)
		case getJointNegLimitEKICommand:
			kuka.handleMinJointPositions(dataArgs)
		case getJointPosLimitEKICommand:
			kuka.handleMaxJointPositions(dataArgs)
		// Get response from move
		case setJointPositionEKICommand:
			kuka.handleSetJointPositions(dataArgs)
		default:
			fmt.Println("UNHANDLED RESPONSE: ", dataList)
		}
	}
}

// Get robot info
func (kuka *kukaArm) handleRobotName(data []string) {
	kuka.logger.Infof(" - Robot Name: %v", data[0])
	if len(data) != 1 {
		kuka.logger.Warnf("incorrect amount of data returned for robot name: %v  (should be 1)", data)
		return
	}
	kuka.deviceInfo.name = data[0]
}

func (kuka *kukaArm) handleRobotSerialNumber(data []string) {
	kuka.logger.Infof(" - Robot Serial Number: %v", data[0])
	if len(data) != 1 {
		kuka.logger.Warnf("incorrect amount of data returned for robot serial number: %v  (should be 1)", data)
		return
	}
	kuka.deviceInfo.serialNum = data[0]
}

func (kuka *kukaArm) handleRobotType(data []string) {
	kuka.logger.Infof(" - Robot Type: %v", data[0])
	if len(data) != 1 {
		kuka.logger.Warnf("incorrect amount of data returned for robot type: %v  (should be 1)", data)
		return
	}
	kuka.deviceInfo.robotType = data[0]
}

func (kuka *kukaArm) handleRobotSoftwareVersion(data []string) {
	kuka.logger.Infof(" - Robot Software Version: %v", data[0])
	if len(data) != 1 {
		kuka.logger.Warnf("incorrect amount of data returned for robot software version mode: %v  (should be 1)", data)
		return
	}
	kuka.deviceInfo.softwareVersion = data[0]
}

func (kuka *kukaArm) handleRobotOperatingMode(data []string) {
	kuka.logger.Infof(" - Robot Operating Mode: %v", data[0])
	if len(data) != 1 {
		kuka.logger.Warnf("incorrect amount of data returned for robot operating mode: %v  (should be 1)", data)
		return
	}
	kuka.deviceInfo.operatingMode = data[0]
}

// Get robot status
func (kuka *kukaArm) handleMinJointPositions(data []string) {
	if len(data) != numJoints+numExternalJoints {
		kuka.logger.Warnf("incorrect amount of data returned for negative joint position limits: %v  (should be 12)", data)
		return
	}

	// Parse data and update current state
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	for i := 0; i < numJoints; i++ {
		c, err := strconv.ParseFloat(data[i], 64)
		if err != nil {
			kuka.logger.Warnf("issue parsing response to floats, failed to parse %v", data)
		}
		kuka.jointLimits[i].min = c
	}
}

func (kuka *kukaArm) handleMaxJointPositions(data []string) {
	if len(data) != numJoints+numExternalJoints {
		kuka.logger.Warnf("incorrect amount of data returned for positive joint position limits: %v  (should be 12)", data)
		return
	}

	// Parse data andpdate current state
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	for i := 0; i < numJoints; i++ {
		c, err := strconv.ParseFloat(data[i], 64)
		if err != nil {
			kuka.logger.Warnf("issue parsing response to floats, failed to parse %v", data)
		}
		kuka.jointLimits[i].max = c
	}
}

func (kuka *kukaArm) handleGetJointPositions(data []string) {
	kuka.logger.Infof(" - Robot Get Joint Positions: %v", data)
	if len(data) != numJoints+numExternalJoints {
		kuka.logger.Warnf("incorrect amount of data returned for joint position limits: %v (should be 12)", data)
		return
	}

	// Parse values to floats
	floatList := make([]float64, numJoints)
	for i := 0; i < numJoints; i++ {
		c, err := strconv.ParseFloat(data[i], 64)
		if err != nil {
			kuka.logger.Warnf("issue parsing response to floats, failed to parse %v", data)
		}
		floatList[i] = c
	}

	// Update current state
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	kuka.currentState.joints = floatList
}

func (kuka *kukaArm) handleGetEndPositions(data []string) {
	kuka.logger.Infof(" - Robot Get End Positions: %v", data)
	if len(data) != 8+numExternalJoints {
		kuka.logger.Warnf("incorrect amount of data returned for end position limits: %v (should be 14)", data)
		return
	}

	// Parse values to floats
	floatList := make([]float64, 8)
	for i := 0; i < numJoints; i++ {
		c, err := strconv.ParseFloat(data[i+1], 64)
		if err != nil {
			kuka.logger.Warnf("issue parsing response to floats, failed to parse %v", data)
		}
		floatList[i] = c
	}

	// Update current state
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	kuka.currentState.endEffectorPose = spatialmath.NewPose(
		r3.Vector{X: floatList[0], Y: floatList[1], Z: floatList[2]},
		&spatialmath.EulerAngles{
			Yaw:   utils.RadToDeg(floatList[3]),
			Pitch: utils.RadToDeg(floatList[4]),
			Roll:  utils.RadToDeg(floatList[5]),
		},
	)
}

func (kuka *kukaArm) handleProgramState(data []string) {
	kuka.logger.Infof(" - Robot Program State: %v", data)
	if len(data) != 2 {
		kuka.logger.Warnf("incorrect amount of data returned for robot programming state: %v (should be 2)", data)
		return
	}

	// Update current state
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	kuka.currentState.programName = data[0]
	kuka.currentState.programState = StringToProgramStatus(data[1])
}

// Set
func (kuka *kukaArm) handleSetJointPositions(data []string) {
	kuka.logger.Infof(" - Robot Set Joint Positions: %v", data)

	// switch data[1] {
	// case "success":
	// 	kuka.setIsMovingSafe(false)
	// case "robotbusy":
	// 	kuka.logger.Warn("warning kuka resource is busy")
	// default:
	// }
}
