package kuka

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/referenceframe/urdf"

	"go.viam.com/utils"

	ekiCommand "github.com/viam-soleng/viam-kuka/src/ekicommands"
)

var (
	sendInfoCommandSleep time.Duration = 200 * time.Millisecond
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
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()

	if newConf.IPAddress != "" {
		kuka.tcpConn.ipAddress = newConf.IPAddress
	} else {
		kuka.logger.Warnf("No ip address given, attempting to connect via default ip %v", defaultIPAddress)
		kuka.tcpConn.ipAddress = defaultIPAddress
	}

	if newConf.Port != 0 {
		kuka.tcpConn.port = newConf.Port
	} else {
		kuka.logger.Warnf("No port given, attempting to connect on default port %v", defaultTCPPort)
		kuka.tcpConn.port = defaultTCPPort
	}

	var model string
	if newConf.Model != "" {
		model = newConf.Model
	} else {
		kuka.logger.Warnf("No model given, attempting to connect to default model: %v", defaultModel)
		model = defaultModel
	}

	var foundModel bool
	for _, supportedModel := range supportedKukaKRModels {
		if model == supportedModel {
			foundModel = true

			urdfModel, err := urdf.ParseModelXMLFile(resolveFile(fmt.Sprintf("src/models/%v_model.urdf", model)), kuka.Name().ShortName())
			if err != nil {
				return err
			}

			kuka.logger.Infof("loading URDF model: %v", fmt.Sprintf("src/models/%v_model.urdf", model))
			kuka.model = urdfModel
		}
	}
	if !foundModel {
		return errors.Errorf("given model (%v) not in list of supported models (%v), no URDF files are available for desired model", model, supportedKukaKRModels)
	}

	kuka.safeMode = newConf.SafeMode

	return nil
}

// resetCurrentStateAndDeviceInfo resets the device's info and stored current state.
func (kuka *kukaArm) resetCurrentStateAndDeviceInfo() {
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()

	kuka.currentState = state{
		jointLimits:  make([]referenceframe.Limit, numJoints),
		programState: ekiCommand.StatusUnknown,
	}
	kuka.deviceInfo = deviceInfo{}
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
func (kuka *kukaArm) updateStateLoop(cancelCtx context.Context) {
	startTime := time.Now()

	for {
		if err := cancelCtx.Err(); err != nil {
			break
		}
		if kuka.closed || time.Now().After(startTime.Add(motionTimeout)) {
			break
		}

		if err := kuka.updateState(); err != nil {
			kuka.logger.Warnf("error updating status: %v", err)
		}
	}
}

// getCurrentStateSafe accesses and returns the current state of the robot safety.
func (kuka *kukaArm) getCurrentStateSafe() state {
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	return kuka.currentState
}

// checkEKIProgramState will ping and wait for the program state to be returned.
func (kuka *kukaArm) checkEKIProgramState(ctx context.Context) (ekiCommand.ProgramStatus, error) {
	if err := kuka.sendCommand(ekiCommand.GetEKIProgramState, ""); err != nil {
		return ekiCommand.StatusUnknown, err
	}

	// Wait until response is returned
	if !utils.SelectContextOrWaitChan(ctx, kuka.responseCh) {
		return ekiCommand.StatusUnknown, errors.New("closed")
	}

	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	return kuka.currentState.programState, nil
}

// checkDesiredJointPositions checks the desired joint positions against the limits defined by the kuka device.
func (kuka *kukaArm) checkDesiredJointPositions(desiredJointPositions []float64) error {
	kuka.stateMutex.Lock()
	currentState := kuka.currentState
	kuka.stateMutex.Unlock()

	// Note: Limits can also be imported via the URDF attached them to the referenceframe.Model. The transform
	// function can then be used as shown below. This is current not the implementation due to differences
	// between the limits in the URDF and those returned by the kuka program.
	// limits := kuka.deviceInfo.model.DoF()
	// kuka.deviceInfo.model.Transform()

	for i := 0; i < numJoints; i++ {
		tempJointPos := desiredJointPositions[i]
		if tempJointPos <= currentState.jointLimits[i].Min || tempJointPos >= currentState.jointLimits[i].Max {
			return errors.Errorf("invalid joint position specified,  %v is outside of joint[%v] limits [%v, %v]",
				desiredJointPositions[i], i, currentState.jointLimits[i].Min, currentState.jointLimits[i].Max)
		}
	}
	return nil
}
