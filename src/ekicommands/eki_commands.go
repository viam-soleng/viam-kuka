package eki_command

import "github.com/pkg/errors"

/* EKI Request and Response Details:
-   a1-a6: degrees, the robot joint values
-   e1-e6: degrees, the external axes joint values
-   mode: T1, T2, Auto or Extern
	- T1: Safe standstill monitoring is activated as soon as all axes are at a standstill or after 680 ms at the latest.
	- T2, AUT (KSS), AUT EXT (KSS), EXT (VSS): After the configured braking time (default: 1.5 s), safe standstill monitoring
	  is activated for all axes.
-   x,y,z: meters, point in space of the end position in space
-   a,b,c: degrees, orientation in space of end position in space
-   status/turn: information regarding robot's position when returning end position as multiple robot poses can lead to
				 same end position
-   program_state: returns the current state of the program either: "Free", "Running", "Reset", "Ended" or "Stopped"
*/

var (
	// Info Commands
	GetRobotName            string = "getrobotname"       // Response: <robot_name>
	GetRobotSoftwareVersion string = "getsoftwareversion" // Response: <sw_version>
	GetRobotSerialNum       string = "getrobotserialnum"  // Response: <robot_serial_number>
	GetRobotType            string = "getrobottype"       // Response: <robot_type>
	GetRobotOperatingMode   string = "getoperatingmode"   // Response: <mode>
	GetEKIProgramState      string = "getprograminfo"     // Response: <program_name,program_state>
	GetJointPosLimit        string = "getposjntlim"       // Response: <a1,a2,a3,a4,a5,a6,e1,e2,e3,e4,e5,e6>
	GetJointNegLimit        string = "getnegjntlim"       // Response: <a1,a2,a3,a4,a5,a6,e1,e2,e3,e4,e5,e6>
	GetEndPosition          string = "getcurrentpos"      // Response: <x,y,z,a,b,c,status,turn,e1,e2,e3,e4,e5,e6>
	GetJointPosition        string = "getcurrentjoints"   // Response: <a1,a2,a3,a4,a5,a6,e1,e2,e3,e4,e5,e6>

	// Motion Commands
	SetJointPosition string = "ptptojointpos" // Request: <a1,a2,a3,a4,a5,a6,e1,e2,e3,e4,e5,e6>, Response: <status>
	SetStop          string = "setstop"       // Response: success
)

type ProgramStatus int64

const (
	StatusFree ProgramStatus = iota
	StatusReset
	StatusRunning
	StatusStopped
	StatusEnded
	StatusUnknown
)

func ProgramStatusToString(status ProgramStatus) (string, error) {
	switch status {
	case StatusFree:
		return "Free", nil
	case StatusReset:
		return "Reset", nil
	case StatusRunning:
		return "Running", nil
	case StatusStopped:
		return "Stopped", nil
	case StatusEnded:
		return "Ended", nil
	default:
		return "", errors.Errorf("unknown program status (%v) returned", status)
	}
}

func StringToProgramStatus(status string) ProgramStatus {
	switch status {
	case "Free":
		return StatusFree
	case "Reset":
		return StatusReset
	case "Running":
		return StatusRunning
	case "Stopped":
		return StatusStopped
	case "Ended":
		return StatusEnded
	default:
		return StatusUnknown
	}
}
