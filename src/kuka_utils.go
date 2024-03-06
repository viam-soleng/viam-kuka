package kuka

import (
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/rdk/referenceframe/urdf"
)

var (
	sendInfoCommandSleep time.Duration = 200 * time.Millisecond
	//sendActionCommandSleep time.Duration = 1 * time.Millisecond
)

// ResolveFile returns the path of the given file relative to the root
// of the codebase. For example, if this file currently
// lives in utils/file.go and ./foo/bar/baz is given, then the result
// is foo/bar/baz. This is helpful when you don't want to relatively
// refer to files when you're not sure where the caller actually
// lives in relation to the target file.
func resolveFile(fn string) string {
	//nolint:dogsled
	_, thisFilePath, _, _ := runtime.Caller(0)
	thisDirPath, err := filepath.Abs(filepath.Dir(thisFilePath))
	if err != nil {
		panic(err)
	}
	return filepath.Join(thisDirPath, "..", fn)
}

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

func (kuka *kukaArm) resetRobotData() {
	kuka.currentState = &state{}
	kuka.jointLimits = make([]jointLimit, numJoints)
}

func (kuka *kukaArm) getDeviceInfo() error {

	if err := kuka.sendCommand(getRobotNameEKICommand, ""); err != nil {
		return err
	}

	if err := kuka.sendCommand(getRobotSerialNumEKICommand, ""); err != nil {
		return err
	}

	if err := kuka.sendCommand(getRobotTypeEKICommand, ""); err != nil {
		return err
	}

	if err := kuka.sendCommand(getRobotSoftwareVersionEKICommand, ""); err != nil {
		return err
	}

	if err := kuka.sendCommand(getJointNegLimitEKICommand, ""); err != nil {
		return err
	}

	if err := kuka.sendCommand(getJointPosLimitEKICommand, ""); err != nil {
		return err
	}

	if err := kuka.updateState(); err != nil {
		return err
	}

	return nil
}

func (kuka *kukaArm) checkDesiredJointPositions(desiredJointPositions []float64) error {
	for i := 0; i < numJoints; i++ {
		tempJointPos := desiredJointPositions[i]
		kuka.logger.Warnf("JOINT CHECK: %v | min: %v | max: %v \n", tempJointPos, kuka.jointLimits[i].min, kuka.jointLimits[i].max)
		if tempJointPos <= kuka.jointLimits[i].min || tempJointPos >= kuka.jointLimits[i].max {
			return errors.Errorf("invalid joint position specified,  %v is outside of joint[%v] limits [%v, %v]",
				desiredJointPositions[i], i, kuka.jointLimits[i].min, kuka.jointLimits[i].max)
		}
	}
	return nil
}

func (kuka *kukaArm) updateState() error {
	fmt.Println("getJointPositionEKICommand")
	if err := kuka.sendCommand(getJointPositionEKICommand, ""); err != nil {
		return err
	}

	//time.Sleep(sendActionCommandSleep)

	fmt.Println("getEndPositionEKICommand")
	if err := kuka.sendCommand(getEndPositionEKICommand, ""); err != nil {
		return err
	}

	return nil
}

func (kuka *kukaArm) updateStateLoop() {
	startTime := time.Now()

	for {
		if kuka.closed || !kuka.getIsMovingSafe() || time.Now().After(startTime.Add(motionTimeout)) {
			break
		}

		if err := kuka.updateState(); err != nil {
			kuka.logger.Warnf("error updating status: %v", err)
		}
	}
}

func (kuka *kukaArm) getIsMovingSafe() bool {
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	return kuka.currentState.isMoving
}

func (kuka *kukaArm) setIsMovingSafe(isMoving bool) {
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	kuka.currentState.isMoving = isMoving
}

func (kuka *kukaArm) checkEKIProgramState() (ProgramStatus, error) {

	kuka.stateMutex.Lock()
	kuka.currentState.programState = statusUnknown
	kuka.stateMutex.Unlock()

	if err := kuka.sendCommand(getEKIProgramState, ""); err != nil {
		return statusUnknown, err
	}

	// Wait until response is returned
	startTime := time.Now()

	var programState ProgramStatus
	for {
		kuka.stateMutex.Lock()
		programState = kuka.currentState.programState
		kuka.stateMutex.Unlock()

		if programState != statusUnknown || time.Now().After(startTime.Add(1*time.Second)) {
			break
		}
	}

	return programState, nil
}
