package main

type RobotData struct {
	MUserId          int
	MAccId           int
	MUserType        int
	MUserName        string
	MPassWord        string
	MUserNick        string
	MTencentCodeId   string
	MTencentToken    string
	MIsNeedAutoLogin bool
	MApointRoomId    int
}
type RobotAttr struct {
	MUserId          int
	MUserType        int
	MUserName        string
	MPassWord        string
	MTencentCodeId   string
	MTencentToken    string
	MIsNeedAutoLogin bool
	MApointRoomId    int
	MTaskId          []int
}

func (RD *RobotData) Init(robotAttr RobotAttr) error {
	RD.MUserId = robotAttr.MUserId
	RD.MUserType = robotAttr.MUserType
	RD.MUserName = robotAttr.MUserName
	RD.MPassWord = robotAttr.MPassWord
	RD.MTencentCodeId = robotAttr.MTencentCodeId
	RD.MTencentToken = robotAttr.MTencentToken
	RD.MIsNeedAutoLogin = robotAttr.MIsNeedAutoLogin
	RD.MApointRoomId = robotAttr.MApointRoomId

	return nil

}
