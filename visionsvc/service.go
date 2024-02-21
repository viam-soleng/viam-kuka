package visionsvc

import (
	"context"
	"image"
	"image/draw"
	"sync"

	"github.com/pkg/errors"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/vision"
	vis "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
)

var errUnimplemented = errors.New("unimplemented")
var Model = resource.NewModel("sol-eng", "vision", "cropping-service")
var PrettyName = "Viam cropping vision service"
var Description = "A module of the Viam vision service that crops an image to an initial detection then runs other models to return detections"

type Config struct {
	CameraName             string  `json:"source_camera"`
	CropDetectorName       string  `json:"crop_detector_name"`
	CropDetectorConfidence float64 `json:"crop_detector_confidence"`
	CropDetectorLabel      string  `json:"crop_detector_label"`
	AgeClassifier          string  `json:"age_classifier_name"`
	GenderClassifier       string  `json:"gender_classifier_name"`
}

type myVisionSvc struct {
	resource.Named
	logger            logging.Logger
	cam               camera.Camera
	croppingDetector  vision.Service
	croppingThreshold float64
	cropLabel         string
	ageClassifier     vision.Service
	genderClassifier  vision.Service
	mu                sync.RWMutex
	cancelCtx         context.Context
	cancelFunc        func()
	done              chan bool
}

func init() {
	resource.RegisterService(
		vision.API,
		Model,
		resource.Registration[vision.Service, *Config]{
			Constructor: newService,
		})
}

func newService(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (vision.Service, error) {
	logger.Debugf("Starting %s %s", PrettyName)
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	svc := myVisionSvc{
		Named:      conf.ResourceName().AsNamed(),
		logger:     logger,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		mu:         sync.RWMutex{},
		done:       make(chan bool),
	}

	if err := svc.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return &svc, nil
}

func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.CameraName == "" {
		return nil, errors.New("source_camera is required")
	}

	if cfg.CropDetectorName == "" {
		return nil, errors.New("crop_detector_name is required")
	}

	if cfg.CropDetectorLabel == "" {
		return nil, errors.New("crop_detector_label is required")
	}

	if cfg.CropDetectorConfidence <= 0.0 {
		return nil, errors.New("crop_detector_confidence must be greater than 0.0")
	}

	if cfg.AgeClassifier == "" {
		return nil, errors.New("age_classifier_name is required")
	}

	if cfg.GenderClassifier == "" {
		return nil, errors.New("gender_classifier_name is required")
	}

	return nil, nil
}

// Reconfigure reconfigures with new settings.
func (svc *myVisionSvc) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	svc.logger.Debugf("Reconfiguring %s", PrettyName)

	// In case the module has changed name
	svc.Named = conf.ResourceName().AsNamed()

	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	// Get the camera
	svc.cam, err = camera.FromDependencies(deps, newConf.CameraName)
	if err != nil {
		return errors.Wrapf(err, "unable to get source camera %v for image sourcing...", newConf.CropDetectorName)
	}

	// Get the face cropper
	svc.croppingDetector, err = vision.FromDependencies(deps, newConf.CropDetectorName)
	if err != nil {
		return errors.Wrapf(err, "unable to get Object Detector %v for image cropping...", newConf.CropDetectorName)
	}

	// Get the face cropper label
	svc.cropLabel = newConf.CropDetectorLabel

	// Get the face cropper threshold
	svc.croppingThreshold = newConf.CropDetectorConfidence

	// Get the age detector
	svc.ageClassifier, err = vision.FromDependencies(deps, newConf.AgeClassifier)
	if err != nil {
		return errors.Wrapf(err, "unable to get classifier %v for age detection...", newConf.AgeClassifier)
	}

	// Get the gender detector
	svc.genderClassifier, err = vision.FromDependencies(deps, newConf.GenderClassifier)
	if err != nil {
		return errors.Wrapf(err, "unable to get classifier %v for gender detection...", newConf.GenderClassifier)
	}
	svc.logger.Debug("**** Reconfigured ****")

	return nil
}

func (svc *myVisionSvc) Detections(ctx context.Context, img image.Image, extra map[string]interface{}) ([]objectdetection.Detection, error) {
	svc.logger.Debug("**** Detections from Image... ****")
	// First, get detections from the croppingDetector
	detections, err := svc.croppingDetector.Detections(ctx, img, nil)
	if err != nil {
		return nil, err
	}

	var finalDetections []objectdetection.Detection
	svc.logger.Debug("**** Detection checked ****")

	for _, detection := range detections {
		if detection.Label() == svc.cropLabel {
			svc.logger.Debug("**** Label Match ****")
			// Check if the detection score is above your threshold
			if detection.Score() >= svc.croppingThreshold {
				// Crop the image to the bounding box of the detection
				croppedImg, err := cropImage(img, detection.BoundingBox())
				if err != nil {
					return nil, err
				}

				// Pass the cropped image to the age and gender detectors
				ageClassification, err := svc.ageClassifier.Classifications(ctx, croppedImg, 1, nil)
				if err != nil {
					return nil, err
				}
				genderClassification, err := svc.genderClassifier.Classifications(ctx, croppedImg, 1, nil)
				if err != nil {
					return nil, err
				}

				// Assume that each detector returns exactly one detection
				// Calculate the average score and create the label
				if len(ageClassification) > 0 && len(genderClassification) > 0 {
					avgScore := (ageClassification[0].Score() + genderClassification[0].Score()) / 2
					label := genderClassification[0].Label() + ", " + ageClassification[0].Label()

					finalDetections = append(finalDetections, NewDetection(detection.BoundingBox(), avgScore, label))

					// Break out of the loop after processing the first detection that exceeds the score threshold
				}
				break
			}
		}
	}

	return finalDetections, nil
}

func (svc *myVisionSvc) DetectionsFromCamera(ctx context.Context, camera string, extra map[string]interface{}) ([]objectdetection.Detection, error) {
	svc.logger.Debug("**** Detections from Camera... ****")

	// gets the stream from a camera
	stream, _ := svc.cam.Stream(context.Background())
	// gets an image from the camera stream
	img, release, _ := stream.Next(context.Background())
	defer release()

	return svc.Detections(ctx, img, nil)
}

// Classifications can be implemented to extend functionality but returns unimplemented currently.
func (s *myVisionSvc) Classifications(ctx context.Context, img image.Image, n int, extra map[string]interface{}) (classification.Classifications, error) {
	return nil, errUnimplemented
}

// ClassificationsFromCamera can be implemented to extend functionality but returns unimplemented currently.
func (s *myVisionSvc) ClassificationsFromCamera(context.Context, string, int, map[string]interface{}) (classification.Classifications, error) {
	return nil, errUnimplemented
}

// ObjectPointClouds can be implemented to extend functionality but returns unimplemented currently.
func (s *myVisionSvc) GetObjectPointClouds(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*vis.Object, error) {
	return nil, errUnimplemented
}

// DoCommand can be implemented to extend functionality but returns unimplemented currently.
func (s *myVisionSvc) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, errUnimplemented
}

// The close method is executed when the component is shut down
func (s *myVisionSvc) Close(ctx context.Context) error {
	s.logger.Debugf("Shutting down %s", PrettyName)
	return nil
}

func cropImage(img image.Image, rect *image.Rectangle) (image.Image, error) {
	// The cropping operation is done by creating a new image of the size of the rectangle
	// and drawing the relevant part of the original image onto the new image.
	cropped := image.NewRGBA(rect.Bounds())
	draw.Draw(cropped, rect.Bounds(), img, rect.Min, draw.Src)
	return cropped, nil
}

type SimpleDetection struct {
	bbox  *image.Rectangle
	score float64
	label string
}

func (d SimpleDetection) BoundingBox() *image.Rectangle {
	return d.bbox
}

func (d SimpleDetection) Score() float64 {
	return d.score
}

func (d SimpleDetection) Label() string {
	return d.label
}

func NewDetection(bbox *image.Rectangle, score float64, label string) objectdetection.Detection {
	return SimpleDetection{
		bbox:  bbox,
		score: score,
		label: label,
	}
}
