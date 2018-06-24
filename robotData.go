package main

type RobotData struct {
	MUserId   int
	MAccId    int
	MUserNmae string
	MPassWord string
	MUserNick string
}
type RobotAttr struct {
	MUserId          int
	MUserType        int
	MUserNmae        string
	MPassWord        string
	MTencentCodeId   string
	MTencentToker    string
	MIsNeedAutoLogin bool
	MApointRoomId    int
	MTaskId          []int
}
