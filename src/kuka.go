package kuka

import (
	"context"
	"fmt"
	"net"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

const (
	numJoints        int    = 6
	defaultTCPPort   int    = 54610
	defaultIPAddress string = "10.1.4.212"
)

var (
	errUnimplemented = errors.New("unimplemented")
	Model            = resource.NewModel("sol-eng", "arm", "kuka")
)

type Config struct {
	IPAddress string `json:"ip_address,omitempty"`
	Port      int    `json:"port,omitempty"`
}

type jointLimit struct {
	min float64
	max float64
}

type device struct {
	name            string
	serialNum       string
	robotType       string
	softwareVersion string
	operatingMode   string
}

type kukaArm struct {
	Named  resource.Named
	logger logging.Logger

	deviceInfo  device
	jointLimits []jointLimit
	isMoving    bool

	ip_address string
	tcp_port   int
	conn       net.Conn
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

func newKukaArm(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (arm.Arm, error) {

	kuka := kukaArm{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
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

	// Parse config
	if newConf.IPAddress != "" {
		kuka.ip_address = newConf.IPAddress
	} else {
		kuka.logger.Debugf("No ip address given, attempting to connect via default ip %v", defaultIPAddress)
		kuka.ip_address = defaultIPAddress
	}

	if newConf.Port != 0 {
		kuka.tcp_port = newConf.Port
	} else {
		kuka.logger.Debugf("No port given, attempting to connect on default port %v", defaultTCPPort)
		kuka.tcp_port = defaultTCPPort
	}

	// Attempt to connect to hardware
	if err := kuka.Connect(ctx); err != nil {
		return err
	}

	// Get device info
	deviceInfo, err := kuka.getDeviceInfo()
	if err != nil {
		return err
	}
	fmt.Println("deviceInfo: ", deviceInfo)
	kuka.deviceInfo = deviceInfo

	// Get joint limits
	jointLimits, err := kuka.getJointLimits()
	if err != nil {
		return err
	}
	fmt.Println("jointLimitsInfo: ", jointLimits)
	kuka.jointLimits = jointLimits

	return nil
}

// The close method is executed when the component is shut down
func (kuka *kukaArm) Close(ctx context.Context) error {
	if err := kuka.Disconnect(); err != nil {
		return err
	}

	return nil
}

// Name TBD
func (kuka *kukaArm) Name() resource.Name {
	return resource.Name{}
}

// CurrentInputs TBD
func (kuka *kukaArm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	fmt.Println("currentinputs")
	return nil, errUnimplemented
}

// GoToInputs TBD
func (kuka *kukaArm) GoToInputs(context.Context, ...[]referenceframe.Input) error {
	fmt.Println("gotoinputs")
	return errUnimplemented
}

// EndPosition returns the current position of the arm.
func (kuka *kukaArm) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {

	// Send command
	response, err := kuka.sendCommand(getEndPositionEKICommand, "")
	if err != nil {
		return nil, err
	}

	// Parse response
	endPosData, err := parseEKLResponseToFloats(response)
	if err != nil {
		return nil, err
	}

	pose := spatialmath.NewPose(r3.Vector{X: endPosData[0], Y: endPosData[1], Z: endPosData[2]},
		&spatialmath.EulerAngles{
			Yaw:   utils.RadToDeg(endPosData[3]),
			Pitch: utils.RadToDeg(endPosData[4]),
			Roll:  utils.RadToDeg(endPosData[5]),
		},
	)

	return pose, nil
}

// JointPositions returns the current joint positions of the arm.
func (kuka *kukaArm) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {

	// Send command
	response, err := kuka.sendCommand(getJointPositionEKICommand, "")
	if err != nil {
		return nil, err
	}

	// Parse response
	jointPosData, err := parseEKLResponseToFloats(response)
	if err != nil {
		return nil, err
	}

	jointPos := make([]float64, numJoints)
	for i := 1; i < numJoints; i++ {
		jointPos[i] = utils.RadToDeg(jointPosData[i])
	}

	return &pb.JointPositions{Values: jointPos}, nil
}

// MoveToPosition moves the arm to the given absolute position. This will block until done or a new operation cancels this one.
func (kuka *kukaArm) MoveToPosition(ctx context.Context, pose spatialmath.Pose, extra map[string]interface{}) error {
	fmt.Println("MoveToPosition")
	return errUnimplemented
}

// MoveToJointPositions moves the arm's joints to the given positions. This will block until done or a new operation cancels this one.
func (kuka *kukaArm) MoveToJointPositions(ctx context.Context, positionDegs *pb.JointPositions, extra map[string]interface{}) error {
	fmt.Println("HIIIII")
	desiredJointPositions := positionDegs.Values

	// Check validity of action based on joint limit
	for i := 0; i < numJoints; i++ {
		tempJointPos := utils.RadToDeg(desiredJointPositions[i])
		if tempJointPos <= kuka.jointLimits[i].min || tempJointPos >= kuka.jointLimits[i].max {
			return errors.Errorf("invalid joint position specified,  %v is outside of joint[%v] limits [%v, %v]",
				desiredJointPositions[i], i, kuka.jointLimits[i].min, kuka.jointLimits[i].max)
		}
	}

	// Send command
	stringifyJoints := fmt.Sprintf("%v,%v,%v,%v,%v,%v",
		utils.RadToDeg(desiredJointPositions[0]),
		utils.RadToDeg(desiredJointPositions[1]),
		utils.RadToDeg(desiredJointPositions[2]),
		utils.RadToDeg(desiredJointPositions[3]),
		utils.RadToDeg(desiredJointPositions[4]),
		utils.RadToDeg(desiredJointPositions[5]),
	)

	kuka.isMoving = true
	response, err := kuka.sendCommand(setJointPositionEKICommand, fmt.Sprintf("%v,0,0,0,0,0,0", stringifyJoints))
	if err != nil {
		return err
	}

	if response != "success" {
		return errors.Errorf("move command was sent but response was unsuccessful: %v", response)
	}

	kuka.isMoving = false

	return nil
}

// CurrentInputs TBD
func (kuka *kukaArm) IsMoving(context.Context) (bool, error) {
	return kuka.isMoving, nil
}

// Stop TBD
func (kuka *kukaArm) Stop(ctx context.Context, extra map[string]interface{}) error {

	err := kuka.MoveToJointPositions(ctx, &pb.JointPositions{Values: []float64{0, utils.DegToRad(-80), utils.DegToRad(90), 0, 0, 0}}, extra)
	if err != nil {
		return err
	}
	fmt.Println("Stop")
	return errUnimplemented
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

	_, err := kuka.Read()
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// ModelFrame returns a simple model frame for the Kuka arm.
func (kuka *kukaArm) ModelFrame() referenceframe.Model {
	model := referenceframe.NewSimpleModel(kuka.Named.Name().ShortName())
	return model
}

// Geometries TBD
func (kuka *kukaArm) Geometries(context.Context, map[string]interface{}) ([]spatialmath.Geometry, error) {
	fmt.Println("Geometries")
	return nil, errUnimplemented
}
