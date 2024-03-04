package kuka

/* EKI Request and Response Details:
-   a1-a6: the robot joint values
-   e1-e6: the external axes joint values
-   mode: T1, T2, Auto or Extern
-   x,y,z: point in space of the end effector
-   a,b,c: orientation in space of end effector
-   status/turn: information regarding robot's position when returning end position as multiple robot poses can lead to
				 same end position
*/

var (
	// Robot information
	getRobotNameEKICommand            string = "getrobotname"       // Response: <robot_name>
	getRobotSoftwareVersionEKICommand string = "getsoftwareversion" // Response: <sw_version>
	getRobotSerialNumEKICommand       string = "getrobotserialnum"  // Response: <robot_serial_number>
	getRobotTypeEKICommand            string = "getrobottype"       // Response: <robot_type>
	getOperatingModeEKICommand        string = "getoperatingmode"   // Response: <mode>

	getJointPosLimitEKICommand string = "getposjntlim" // Response: <a1,a2,a3,a4,a5,a6,e1,e2,e3,e4,e5,e6>
	getJointNegLimitEKICommand string = "getnegjntlim" // Response: <a1,a2,a3,a4,a5,a6,e1,e2,e3,e4,e5,e6>

	// Get current joints and end position information
	getEndPositionEKICommand   string = "getcurrentpos"    // Response: <x,y,z,a,b,c,status,turn,e1,e2,e3,e4,e5,e6>
	getJointPositionEKICommand string = "getcurrentjoints" // Response: <a1,a2,a3,a4,a5,a6,e1,e2,e3,e4,e5,e6>

	// Set current joints
	setJointPositionEKICommand string = "ptptojointpos" // Request: <a1,a2,a3,a4,a5,a6,e1,e2,e3,e4,e5,e6>, Response: <status>
)
