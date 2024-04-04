package kuka

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
	ekiCommand "github.com/viam-soleng/viam-kuka/src/ekicommands"

	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"

	gutils "go.viam.com/utils"
)

const (
	numJoints         int = 6
	numExternalJoints int = 6

	defaultTCPPort int = 54610

	defaultJointSpeed float64 = 2.5
)

var (
	Model                       = resource.NewModel("sol-eng", "arm", "kuka")
	motionTimeout time.Duration = 30 * time.Second
)

// the set of supported armModels
const (
	kr10r900 = "KR10r900"
)

var supportedKukaKRModels = []string{kr10r900}

type Config struct {
	IPAddress string `json:"ip_address"`
	Port      int    `json:"port,omitempty"`
	Model     string `json:"model,omitempty"`
	SafeMode  bool   `json:"safe_mode,omitempty"`
}

type state struct {
	endEffectorPose spatialmath.Pose
	joints          []float64
	jointLimits     []referenceframe.Limit

	isMoving bool

	programState ekiCommand.ProgramStatus
	programName  string
}

type deviceInfo struct {
	name            string
	serialNum       string
	robotType       string
	softwareVersion string
	operatingMode   string
}

type tcpConn struct {
	ipAddress string
	port      int
	conn      net.Conn
	mu        sync.Mutex
}

type kukaArm struct {
	resource.Named
	logger logging.Logger

	deviceInfo   deviceInfo
	currentState state
	stateMutex   sync.Mutex
	model        referenceframe.Model

	closed                  bool
	safeMode                bool
	activeBackgroundWorkers sync.WaitGroup

	tcpConn tcpConn

	responseCh chan bool
}

func init() {
	resource.RegisterComponent(
		arm.API,
		Model,
		resource.Registration[arm.Arm, *Config]{
			Constructor: newKukaArm,
		})
}

func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.IPAddress == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "ip_address")
	}

	return nil, nil
}

// newKukaArm creates a new Kuka arm.
func newKukaArm(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (arm.Arm, error) {

	kuka := kukaArm{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
		tcpConn: tcpConn{
			mu: sync.Mutex{},
		},
		deviceInfo: deviceInfo{},

		activeBackgroundWorkers: sync.WaitGroup{},
		stateMutex:              sync.Mutex{},
		responseCh:              make(chan bool, 1),
	}

	if err := kuka.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return &kuka, nil
}

// Reconfigure reconfigures with new settings.
func (kuka *kukaArm) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {

	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	// Reset robot
	kuka.resetCurrentStateAndDeviceInfo()

	// Parse config
	if err := kuka.parseConfig(newConf); err != nil {
		return err
	}

	// Attempt to connect to hardware
	if err := kuka.Connect(ctx); err != nil {
		return err
	}

	// Start background monitor of logs from kuka device
	if err := kuka.startResponseMonitor(); err != nil {
		return err
	}

	// Get device info
	if err := kuka.getDeviceInfo(); err != nil {
		return err
	}

	kuka.logger.Debugf("Device Info: %v", kuka.deviceInfo)

	// Set initial values
	if err := kuka.setInitialValues(); err != nil {
		return err
	}

	// Check program state
	programState, err := kuka.checkEKIProgramState(ctx)
	if err != nil {
		return err
	}
	if programState != ekiCommand.StatusRunning {
		return errors.Errorf("associated program on your kuka device is %v, please get the program running before continuing", programState)
	}

	return nil
}

// The close method is executed when the component is shut down.
func (kuka *kukaArm) Close(ctx context.Context) error {
	kuka.closed = true

	// Wait for mutexes
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()

	kuka.tcpConn.mu.Lock()
	defer kuka.tcpConn.mu.Unlock()

	// Wait for background process to end
	kuka.activeBackgroundWorkers.Wait()

	// Disconnect tcp connection
	if err := kuka.Disconnect(); err != nil {
		return err
	}
	return nil
}

// CurrentInputs returns the current joint positions in the form of Inputs.
func (kuka *kukaArm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	return kuka.model.InputFromProtobuf(&pb.JointPositions{Values: kuka.getCurrentStateSafe().joints}), nil
}

// GoToInputs moves through the given inputSteps using sequential calls to MoveJointPosition.
func (kuka *kukaArm) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	for _, goal := range inputSteps {
		if !kuka.getCurrentStateSafe().isMoving {

			kuka.stateMutex.Lock()
			jointPositions := kuka.model.ProtobufFromInput(goal)
			kuka.stateMutex.Unlock()

			if err := kuka.MoveToJointPositions(ctx, jointPositions, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// EndPosition returns the current position of the arm.
func (kuka *kukaArm) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	return kuka.getCurrentStateSafe().endEffectorPose, nil
}

// JointPositions returns the current joint positions of the arm.
func (kuka *kukaArm) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	return &pb.JointPositions{Values: kuka.getCurrentStateSafe().joints}, nil
}

// MoveToPosition moves the arm to the given absolute position. This will block until done or a new operation cancels this one.
// This calls arm Move command that uses motion planning to make subsequent MoveToJointPositions to reach goal position.
func (kuka *kukaArm) MoveToPosition(ctx context.Context, pose spatialmath.Pose, extra map[string]interface{}) error {
	return arm.Move(ctx, kuka.logger, kuka, pose)
}

// MoveToJointPositions moves the arm's joints to the given positions. This will block until done or a new operation cancels this one.
func (kuka *kukaArm) MoveToJointPositions(ctx context.Context, positionDegs *pb.JointPositions, extra map[string]interface{}) error {

	desiredJointPositions := positionDegs.Values

	// Check validity of action based on joint limit
	if err := kuka.checkDesiredJointPositions(desiredJointPositions); err != nil {
		return err
	}

	if isMoving, _ := kuka.IsMoving(ctx); isMoving {
		return errors.New("robot is still moving, please try again after previous movement is complete")
	}

	stringifyJoints := fmt.Sprintf("%v,%v,%v,%v,%v,%v",
		desiredJointPositions[0],
		desiredJointPositions[1],
		desiredJointPositions[2],
		desiredJointPositions[3],
		desiredJointPositions[4],
		desiredJointPositions[5],
	)

	// Check EKI program state before issuing move command
	if kuka.safeMode {
		programState, err := kuka.checkEKIProgramState(ctx)
		if err != nil {
			return err
		}
		if programState != ekiCommand.StatusRunning {
			return errors.Errorf("associated program on your kuka device is %v, please get the program running before continuing", programState)
		}
	}

	// Send command
	kuka.stateMutex.Lock()
	kuka.currentState.isMoving = true
	kuka.stateMutex.Unlock()
	if err := kuka.sendCommand(ekiCommand.SetJointPosition, fmt.Sprintf("%v,0,0,0,0,0,0", stringifyJoints)); err != nil {
		return err
	}

	// Loop until operation ends
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()
	kuka.activeBackgroundWorkers.Add(1)
	gutils.PanicCapturingGo(func() {
		kuka.updateStateLoop(cancelCtx)
	})

	<-kuka.responseCh

	return nil
}

// IsMoving returns if the arm is in motion.
func (kuka *kukaArm) IsMoving(context.Context) (bool, error) {
	return kuka.getCurrentStateSafe().isMoving, nil
}

// Stop stops and ongoing actions
func (kuka *kukaArm) Stop(ctx context.Context, extra map[string]interface{}) error {
	if err := kuka.sendCommand(ekiCommand.SetStop, ""); err != nil {
		return err
	}

	// Get joint and eng effector position after stop action has occurred
	kuka.updateState()
	if err := kuka.updateState(); err != nil {
		return err
	}
	return nil
}

// DoCommand can be implemented to extend functionality but returns unimplemented currently.
func (kuka *kukaArm) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	command, ok := cmd["cmd"].(string)
	if !ok {
		return nil, errors.Errorf("error, request value (%v) was not a string", cmd["cmd"])
	}

	if err := kuka.Write([]byte(command)); err != nil {
		return nil, err
	}

	return nil, nil
}

// ModelFrame returns a simple model frame for the Kuka arm.
func (kuka *kukaArm) ModelFrame() referenceframe.Model {
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	return kuka.model
}

// Geometries returns a list of geometries associated with the specified kuka arm.
func (kuka *kukaArm) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	model := kuka.model

	inputs, err := kuka.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}

	geometries, err := model.Geometries(inputs)
	if err != nil {
		return nil, err
	}

	return geometries.Geometries(), nil
}
