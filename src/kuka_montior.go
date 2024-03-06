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
			fmt.Println("empty")
			continue
		}

		dataStr := string(data[:len(data)-1])

		kuka.logger.Infof("FROM KUKA: %v\n", dataStr)

		dataList := strings.Split(dataStr, ",")

		dataCommand := dataList[0]
		dataArgs := dataList[1:]

		if len(dataArgs) > 0 {
			if dataArgs[0] == "success" {
				fmt.Println("handling success")
				kuka.stateMutex.Lock()
				kuka.currentState.isMoving = false
				kuka.stateMutex.Unlock()
				continue
			}
		}

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
		kuka.logger.Warnf("incorrect amount of data returned for robot name: %v", data)
		return
	}
	kuka.deviceInfo.name = data[0]
}

func (kuka *kukaArm) handleRobotSerialNumber(data []string) {
	kuka.logger.Infof(" - Robot Serial Number: %v", data[0])
	if len(data) != 1 {
		kuka.logger.Warnf("incorrect amount of data returned for robot serial number: %v", data)
		return
	}
	kuka.deviceInfo.serialNum = data[0]
}

func (kuka *kukaArm) handleRobotType(data []string) {
	kuka.logger.Infof(" - Robot Type: %v", data[0])
	if len(data) != 1 {
		kuka.logger.Warnf("incorrect amount of data returned for robot type: %v", data)
		return
	}
	kuka.deviceInfo.robotType = data[0]
}

func (kuka *kukaArm) handleRobotSoftwareVersion(data []string) {
	kuka.logger.Infof(" - Robot Software Version: %v", data[0])
	if len(data) != 1 {
		kuka.logger.Warnf("incorrect amount of data returned for robot software version mode: %v", data)
		return
	}
	kuka.deviceInfo.softwareVersion = data[0]
}

func (kuka *kukaArm) handleRobotOperatingMode(data []string) {
	kuka.logger.Infof(" - Robot Operating Mode: %v", data[0])
	if len(data) != 1 {
		kuka.logger.Warnf("incorrect amount of data returned for robot operating mode: %v", data)
		return
	}
	kuka.deviceInfo.operatingMode = data[0]
}

// Get robot status
func (kuka *kukaArm) handleMinJointPositions(data []string) {
	if len(data) != numJoints+numExternalJoints {
		kuka.logger.Warnf("incorrect amount of data returned for negative joint position limits: %v", data)
		return
	}

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
		kuka.logger.Warnf("incorrect amount of data returned for positive joint position limits: %v", data)
		return
	}

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
		kuka.logger.Warnf("incorrect amount of data returned for joint position limits: %v", data)
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

	kuka.currentState.joints = floatList
}

func (kuka *kukaArm) handleGetEndPositions(data []string) {
	kuka.logger.Infof(" - Robot Get End Positions: %v", data)
	if len(data) != 8+numExternalJoints {
		kuka.logger.Warnf("incorrect amount of data returned for end position limits: %v", data)
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

	kuka.currentState.endEffectorPose = spatialmath.NewPose(
		r3.Vector{X: floatList[0], Y: floatList[1], Z: floatList[2]},
		&spatialmath.EulerAngles{
			Yaw:   utils.RadToDeg(floatList[3]),
			Pitch: utils.RadToDeg(floatList[4]),
			Roll:  utils.RadToDeg(floatList[5]),
		},
	)
}

// Set
func (kuka *kukaArm) handleSetJointPositions(data []string) {
	kuka.logger.Infof(" - Robot Set Joint Positions: %v", data)

	switch data[1] {
	case "success":
		kuka.stateMutex.Lock()
		defer kuka.stateMutex.Unlock()
		kuka.currentState.isMoving = false
	case "robotbusy":
		kuka.logger.Warn("warning kuka resource is busy")
	default:
	}
}
