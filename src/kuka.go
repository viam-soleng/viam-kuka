package kuka

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"

	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

const (
	numJoints         int    = 6
	numExternalJoints int    = 6
	defaultTCPPort    int    = 54610
	defaultIPAddress  string = "10.1.4.212"
	defaultModel      string = "KR5_ACR"
)

var (
	errUnimplemented = errors.New("unimplemented")
	Model            = resource.NewModel("sol-eng", "arm", "kuka")

	sendInfoCommandSleep time.Duration = 200 * time.Millisecond
	//sendActionCommandSleep time.Duration = 1 * time.Millisecond
)

type Config struct {
	IPAddress string `json:"ip_address,omitempty"`
	Port      int    `json:"port,omitempty"`
	Model     string `json:"model,omitempty"`
}

type jointLimit struct {
	min float64
	max float64
}

type state struct {
	endEffectorPose spatialmath.Pose
	joints          []float64
	isMoving        bool
}

type deviceInfo struct {
	name            string
	serialNum       string
	robotType       string
	softwareVersion string
	operatingMode   string
}

type kukaArm struct {
	resource.Named
	logger logging.Logger

	deviceInfo              deviceInfo
	model                   referenceframe.Model
	jointLimits             []jointLimit
	closed                  bool
	activeBackgroundWorkers *sync.WaitGroup

	currentState *state
	stateMutex   *sync.Mutex

	ip_address string
	tcp_port   int
	conn       net.Conn
	tcpMutex   *sync.Mutex
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
	return nil, nil
}

// newKukaArm creates a new Kuka arm.
func newKukaArm(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (arm.Arm, error) {

	kuka := kukaArm{
		Named:      conf.ResourceName().AsNamed(),
		logger:     logger,
		deviceInfo: deviceInfo{},

		activeBackgroundWorkers: &sync.WaitGroup{},
		tcpMutex:                &sync.Mutex{},
		stateMutex:              &sync.Mutex{},
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
	kuka.resetRobotData()

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

	// // Get device info
	if err := kuka.getDeviceInfo(); err != nil {
		return err
	}

	return nil
}

// The close method is executed when the component is shut down.
func (kuka *kukaArm) Close(ctx context.Context) error {
	kuka.closed = true

	kuka.activeBackgroundWorkers.Wait()

	if err := kuka.Disconnect(); err != nil {
		return err
	}

	return nil
}

// CurrentInputs returns the current joint positions in the form of Inputs.
func (kuka *kukaArm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	jointPositions := kuka.currentState.joints

	return kuka.model.InputFromProtobuf(&pb.JointPositions{Values: jointPositions}), nil

}

// GoToInputs moves through the given inputSteps using sequential calls to MoveJointPosition.
func (kuka *kukaArm) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	for _, goal := range inputSteps {
		kuka.stateMutex.Lock()
		isMoving := kuka.currentState.isMoving
		kuka.stateMutex.Unlock()

		if !isMoving {
			if err := kuka.MoveToJointPositions(ctx, kuka.model.ProtobufFromInput(goal), nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// EndPosition returns the current position of the arm.
func (kuka *kukaArm) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	fmt.Println("EndPosition (currentState):", kuka.currentState)

	return kuka.currentState.endEffectorPose, nil
}

// JointPositions returns the current joint positions of the arm.
func (kuka *kukaArm) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	fmt.Println("JointPositions (currentState):", kuka.currentState)

	return &pb.JointPositions{Values: kuka.currentState.joints}, nil
}

// MoveToPosition moves the arm to the given absolute position. This will block until done or a new operation cancels this one.
func (kuka *kukaArm) MoveToPosition(ctx context.Context, pose spatialmath.Pose, extra map[string]interface{}) error {
	fmt.Println("MoveToPosition")
	return arm.Move(ctx, kuka.logger, kuka, pose)
}

// MoveToJointPositions moves the arm's joints to the given positions. This will block until done or a new operation cancels this one.
func (kuka *kukaArm) MoveToJointPositions(ctx context.Context, positionDegs *pb.JointPositions, extra map[string]interface{}) error {

	desiredJointPositions := positionDegs.Values

	// Check validity of action based on joint limit
	if err := kuka.checkDesiredJointPositions(desiredJointPositions); err != nil {
		return err
	}

	stringifyJoints := fmt.Sprintf("%v,%v,%v,%v,%v,%v",
		desiredJointPositions[0],
		desiredJointPositions[1],
		desiredJointPositions[2],
		desiredJointPositions[3],
		desiredJointPositions[4],
		desiredJointPositions[5],
	)

	// Send command
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()
	kuka.currentState.isMoving = true
	if err := kuka.sendCommand(setJointPositionEKICommand, fmt.Sprintf("%v,0,0,0,0,0,0", stringifyJoints)); err != nil {
		return err
	}

	// Start update state loop
	kuka.activeBackgroundWorkers.Add(1)
	go func() {
		defer kuka.activeBackgroundWorkers.Done()
		kuka.updateStateLoop()
	}()

	return nil
}

// IsMoving returns if the arm is in motion.
func (kuka *kukaArm) IsMoving(context.Context) (bool, error) {
	fmt.Println("IsMoving")
	kuka.stateMutex.Lock()
	defer kuka.stateMutex.Unlock()

	return kuka.currentState.isMoving, nil
}

// Stop TBD
func (kuka *kukaArm) Stop(ctx context.Context, extra map[string]interface{}) error {
	fmt.Println("Stop")
	return errUnimplemented
}

// DoCommand can be implemented to extend functionality but returns unimplemented currently.
func (kuka *kukaArm) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {

	command, ok := cmd["cmd"].(string)
	if !ok {
		return nil, errors.Errorf("error, request value (%v) was not a string", cmd["cmd"])
	}

	if command == "settestjointposition" {
		kuka.MoveToJointPositions(ctx, &pb.JointPositions{Values: []float64{0, -50, 120, 0, 0, 0}}, nil)
	} else {
		if err := kuka.Write([]byte(command)); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// ModelFrame returns a simple model frame for the Kuka arm.
func (kuka *kukaArm) ModelFrame() referenceframe.Model {
	return kuka.model
}

// Geometries TBD
func (kuka *kukaArm) Geometries(context.Context, map[string]interface{}) ([]spatialmath.Geometry, error) {
	fmt.Println("Geometries")
	return nil, errUnimplemented
}
