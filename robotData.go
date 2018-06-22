package main

type RobotData struct {
	MUserId   int32
	MAccId    int32
	MUserNmae string
	MPassWord string
	MUserNick string
}
type RobotAttr struct {
	MUserId          int32
	MUserType        int32
	MUserNmae        string
	MPassWord        string
	MTencentCodeId   string
	MTencentToker    string
	MIsNeedAutoLogin bool
	MApointRoomId    int32
	MTaskId          []int32
}
