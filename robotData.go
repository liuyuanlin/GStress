package main

type RobotData struct {
	MUId           int    //配置的ID
	UserIdentifier string //服务器中实际的ID
	ActivityId     int64
	GameId         int32
	MUserName      string
	MPassWord      string
	MTaskId        []int
}
