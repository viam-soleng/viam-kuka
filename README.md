# Viam Cropping Vision Service

This repository contains the `visionsvc` package, a module of the Viam vision service designed for image cropping and further analysis. It integrates several vision services, including object detection, age classification, and gender classification.

## Description

The Viam Cropping Vision Service (`visionsvc`) is a specialized module within the Viam vision framework. Its primary function is to crop an image to an initial detection and then utilize other models to return detailed detections, including age and gender classifications.

## Features

- Accept a "Cropping" Object Detector.
- Use the bounding box from the Cropping Object Detector to specific the bounding box to run classification against.
- Run age classifier.
- Run gender classifier.
- Return a single object detection.

## Installation

Sample config:
```json
{
  "crop_detector_confidence": 0.7,
  "crop_detector_label": "0",
  "age_classifier_name": "age-vision",
  "gender_classifier_name": "gender-vision",
  "source_camera": "camera",
  "crop_detector_name": "person-vision"
}
```