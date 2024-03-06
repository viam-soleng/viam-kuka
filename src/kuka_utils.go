package kuka

import (
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/rdk/referenceframe/urdf"

	ekiCommand "github.com/viam-soleng/viam-kuka/src/ekicommands"
)

var (
	sendInfoCommandSleep time.Duration = 200 * time.Millisecond
	statusTimeout        time.Duration = 200 * time.Millisecond
)

// ResolveFile returns the path of the given file relative to the root of the codebase.
func resolveFile(fn string) string {
	//nolint:dogsled
	_, thisFilePath, _, _ := runtime.Caller(0)
	thisDirPath, err := filepath.Abs(filepath.Dir(thisFilePath))
	if err != nil {
		panic(err)
	}
	return filepath.Join(thisDirPath, "..", fn)
}

// sendCommand will send the desired command (with any and all arguments), in the proper format,
// to the kuka device via the TCP connection.
func (kuka *kukaArm) sendCommand(EKICommand, args string) error {
	var command string
	if args != "" {
		command = fmt.Sprintf("%v,%v;", EKICommand, args)
	} else {
		command = fmt.Sprintf("%v;", EKICommand)
	}

	if err := kuka.Write([]byte(command)); err != nil {
		return err
	}

	time.Sleep(sendInfoCommandSleep)

	return nil
}

// parseConfig parses the given config, updating the kuka device info as necessary.
func (kuka *kukaArm) parseConfig(newConf *Config) error {
	if newConf.IPAddress != "" {
		kuka.ip_address = newConf.IPAddress
	} else {
		kuka.logger.Warnf("No ip address given, attempting to connect via default ip %v", defaultIPAddress)
		kuka.ip_address = defaultIPAddress
	}

	if newConf.Port != 0 {
		kuka.tcp_port = newConf.Port
	} else {
		kuka.logger.Warnf("No port given, attempting to connect on default port %v", defaultTCPPort)
		kuka.tcp_port = defaultTCPPort
	}

	var model string
	if newConf.Model != "" {
		model = newConf.Model
	} else {
		kuka.logger.Warnf("No model given, attempting to connect to default model: %v", defaultModel)
		model = defaultModel
	}
	urdfModel, err := urdf.ParseModelXMLFile(resolveFile(fmt.Sprintf("src/models/%v_model.urdf", model)), kuka.Name().ShortName())
	if err != nil {
		return err
	}
	kuka.model = urdfModel

	kuka.safeMode = newConf.SafeMode

	return nil
}

// getDeviceInfo will send a series of commands to the device to gather information from robot name and model to limits on joint movement
// and starting positions.
func (kuka *kukaArm) getDeviceInfo() error {

	// List of startup commands
	startUpCommandList := []string{
		ekiCommand.GetRobotName,
		ekiCommand.GetRobotSerialNum,
		ekiCommand.GetRobotType,
		ekiCommand.GetRobotSoftwareVersion,
		ekiCommand.GetRobotOperatingMode,
		ekiCommand.GetJointNegLimit,
		ekiCommand.GetJointPosLimit,
	}

	for _, command := range startUpCommandList {
		if err := kuka.sendCommand(command, ""); err != nil {
			return err
		}
	}

	// Update current state of kuka device
	if err := kuka.updateState(); err != nil {
		return err
	}

	return nil
}

// updateState pings the kuka device for its current joint positions and end position.
func (kuka *kukaArm) updateState() error {
	if err := kuka.sendCommand(ekiCommand.GetJointPosition, ""); err != nil {
		return err
	}

	if err := kuka.sendCommand(ekiCommand.GetEndPosition, ""); err != nil {
		return err
	}

	return nil
}

// updateStateLoop repeatedly pings the kuka device for current state information when the robot is in motion.
func (kuka *kukaArm) updateStateLoop() {
	startTime := time.Now()

	for {
		if kuka.closed || !kuka.getCurrentStateSafe().isMoving || time.Now().After(startTime.Add(motionTimeout)) {
			break
		}

		if err := kuka.updateState(); err != nil {
			kuka.logger.Warnf("error updating status: %v", err)
		}
	}
}

// getCurrentStateSafe accesses and returns the current state of the robot safety.
func (kuka *kukaArm) getCurrentStateSafe() *state {
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	return kuka.currentState
}

// checkEKIProgramState will ping and wait for the program state to be returned.
func (kuka *kukaArm) checkEKIProgramState() (ekiCommand.ProgramStatus, error) {

	kuka.stateMutex.Lock()
	kuka.currentState.programState = ekiCommand.StatusUnknown
	kuka.stateMutex.Unlock()

	if err := kuka.sendCommand(ekiCommand.GetEKIProgramState, ""); err != nil {
		return ekiCommand.StatusUnknown, err
	}

	// Wait until response is returned
	startTime := time.Now()

	var programState ekiCommand.ProgramStatus
	for {
		kuka.stateMutex.Lock()
		programState = kuka.currentState.programState
		kuka.stateMutex.Unlock()

		if programState != ekiCommand.StatusUnknown || time.Now().After(startTime.Add(statusTimeout)) {
			break
		}
	}

	return programState, nil
}

// checkDesiredJointPositions checks the desried joint positions against the limits defined by the kuka device.
func (kuka *kukaArm) checkDesiredJointPositions(desiredJointPositions []float64) error {
	for i := 0; i < numJoints; i++ {
		tempJointPos := desiredJointPositions[i]
		if tempJointPos <= kuka.jointLimits[i].min || tempJointPos >= kuka.jointLimits[i].max {
			return errors.Errorf("invalid joint position specified,  %v is outside of joint[%v] limits [%v, %v]",
				desiredJointPositions[i], i, kuka.jointLimits[i].min, kuka.jointLimits[i].max)
		}
	}
	return nil
}
