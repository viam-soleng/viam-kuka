package kuka

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

func parseEKLResponseToFloats(resp string) ([]float64, error) {

	splitResponse := strings.Split(resp, ",")

	floatList := make([]float64, len(splitResponse))
	for i, r := range splitResponse {
		c, err := strconv.ParseFloat(r, 64)
		if err != nil {
			return nil, errors.Errorf("issue parsing response to floats, %v failed to parse", r)
		}
		floatList[i] = c
	}

	return floatList, nil
}

func parseEKLResponse(data []byte, cmd string) string {
	parsedResponse := string(data[len(cmd)+1 : len(data)-1])

	return parsedResponse
}

func (kuka *kukaArm) sendCommand(EKICommand, args string) (string, error) {

	var command string
	if args != "" {
		command = fmt.Sprintf("%v,%v;", EKICommand, args)
	} else {
		command = fmt.Sprintf("%v;", EKICommand)
	}

	if err := kuka.Write([]byte(command)); err != nil {
		return "", err
	}

	// Read response
	data, err := kuka.Read()
	if err != nil {
		return "", err
	}

	// Parse response
	return parseEKLResponse(data, EKICommand), nil
}

func (kuka *kukaArm) getDeviceInfo() (device, error) {

	var err error

	respRobotName, err := kuka.sendCommand(getRobotNameEKICommand, "")
	if err != nil {
		return device{}, err
	}

	respSerialNum, err := kuka.sendCommand(getRobotSerialNumEKICommand, "")
	if err != nil {
		return device{}, err
	}

	respRobotType, err := kuka.sendCommand(getRobotTypeEKICommand, "")
	if err != nil {
		return device{}, err
	}

	respSWVersion, err := kuka.sendCommand(getRobotSoftwareVersionEKICommand, "")
	if err != nil {
		return device{}, err
	}

	respOperatingMode, err := kuka.sendCommand(getOperatingModeEKICommand, "")
	if err != nil {
		return device{}, err
	}

	return device{
		name:            respRobotName,
		robotType:       respRobotType,
		serialNum:       respSerialNum,
		softwareVersion: respSWVersion,
		operatingMode:   respOperatingMode,
	}, nil
}

func (kuka *kukaArm) getJointLimits() ([]jointLimit, error) {

	// Min joint values
	respMinJointValues, err := kuka.sendCommand(getJointNegLimitEKICommand, "")
	if err != nil {
		return nil, err
	}
	minJointValues, err := parseEKLResponseToFloats(respMinJointValues)
	if err != nil {
		return nil, err
	}

	// Max joint values
	respMaxJointValues, err := kuka.sendCommand(getJointPosLimitEKICommand, "")
	if err != nil {
		return nil, err
	}
	maxJointValues, err := parseEKLResponseToFloats(respMaxJointValues)
	if err != nil {
		return nil, err
	}

	joints := make([]jointLimit, numJoints)
	for i := 0; i < numJoints; i++ {
		joints[i] = jointLimit{
			min: minJointValues[i],
			max: maxJointValues[i],
		}
	}

	return joints, nil
}
