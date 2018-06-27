package main

type RobotData struct {
	MUId             int //配置的ID
	MUserId          int //服务器中实际的ID
	MAccId           int
	MUserType        int
	MUserName        string
	MPassWord        string
	MDevice          string
	MPackageflag     string
	MClientIp        string
	MUserNick        string
	MTencentCodeId   string
	MTencentToken    string
	MIsNeedAutoLogin bool
	MApointRoomId    int
}
type RobotAttr struct {
	MUId             int
	MUserId          int
	MUserType        int
	MUserName        string
	MPassWord        string
	MDevice          string
	MPackageflag     string
	MClientIp        string
	MTencentCodeId   string
	MTencentToken    string
	MIsNeedAutoLogin bool
	MApointRoomId    int
	MTaskId          []int
}

func (RD *RobotData) Init(robotAttr RobotAttr) error {
	RD.MUId = robotAttr.MUId
	RD.MUserId = robotAttr.MUserId
	RD.MUserType = robotAttr.MUserType
	RD.MUserName = robotAttr.MUserName
	RD.MPassWord = robotAttr.MPassWord

	RD.MDevice = robotAttr.MDevice
	RD.MPackageflag = robotAttr.MPackageflag
	RD.MClientIp = robotAttr.MClientIp

	RD.MTencentCodeId = robotAttr.MTencentCodeId
	RD.MTencentToken = robotAttr.MTencentToken
	RD.MIsNeedAutoLogin = robotAttr.MIsNeedAutoLogin
	RD.MApointRoomId = robotAttr.MApointRoomId

	return nil

}
