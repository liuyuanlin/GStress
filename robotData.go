package main

type ClubData struct {
	MClubId   int    //微游戏ID
	MClubName string //微游戏名称
	MUserLv   int    //玩家权限 0 普通 1 管理员 2 创建者

}

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
	MXzmjRoomId      int
	MXzmjTableId     int
	MWantClubId      int
	MToken           string
	MUserGold        int
	MUserDiamond     int
	MClubData        ClubData
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
	MXzmjRoomId      int
	MXzmjTableId     int
	MWantClubId      int
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
	RD.MXzmjRoomId = robotAttr.MXzmjRoomId
	RD.MXzmjTableId = robotAttr.MXzmjTableId
	RD.MWantClubId = robotAttr.MWantClubId

	return nil

}
