package kuka

import (
	"context"
	"net"

	"github.com/pkg/errors"

	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

const (
	defaultTCPPort int = 7000
)

var (
	errUnimplemented = errors.New("unimplemented")
	Model            = resource.NewModel("sol-eng", "arm", "kuka")
)

type Config struct {
	Port int `json:"port"`
}

type kukaArm struct {
	logger   logging.Logger
	tcp_port int
	conn     net.Conn
}

func init() {
	resource.RegisterService(
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
	// cancelCtx, cancelFunc := context.WithCancel(context.Background())

	kuka := kukaArm{
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
	if newConf.Port != 0 {
		kuka.tcp_port = newConf.Port
	} else {
		kuka.logger.Debugf("No port given, attempting to connect on default port %v", defaultTCPPort)
		kuka.tcp_port = defaultTCPPort
	}

	// Attempt to connect to hardware
	if err := kuka.Connect(); err != nil {
		return err
	}

	return errUnimplemented
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
	return nil, errUnimplemented
}

// GoToInputs TBD
func (kuka *kukaArm) GoToInputs(context.Context, ...[]referenceframe.Input) error {
	return errUnimplemented
}

// EndPosition returns the current position of the arm.
func (kuka *kukaArm) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	return nil, errUnimplemented
}

// JointPositions returns the current joint positions of the arm.
func (kuka *kukaArm) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	return nil, errUnimplemented
}

// MoveToPosition moves the arm to the given absolute position. This will block until done or a new operation cancels this one.
func (kuka *kukaArm) MoveToPosition(ctx context.Context, pose spatialmath.Pose, extra map[string]interface{}) error {
	return errUnimplemented
}

// MoveToJointPositions moves the arm's joints to the given positions. This will block until done or a new operation cancels this one.
func (kuka *kukaArm) MoveToJointPositions(ctx context.Context, positionDegs *pb.JointPositions, extra map[string]interface{}) error {
	return errUnimplemented
}

// CurrentInputs TBD
func (kuka *kukaArm) IsMoving(context.Context) (bool, error) {
	return false, errUnimplemented
}

// Stop TBD
func (kuka *kukaArm) Stop(context.Context, map[string]interface{}) error {
	return errUnimplemented
}

// DoCommand can be implemented to extend functionality but returns unimplemented currently.
func (kuka *kukaArm) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, errUnimplemented
}

// ModelFrame TBD
func (kuka *kukaArm) ModelFrame() referenceframe.Model {
	return nil
}

// Geometries TBD
func (kuka *kukaArm) Geometries(context.Context, map[string]interface{}) ([]spatialmath.Geometry, error) {
	return nil, errUnimplemented
}
