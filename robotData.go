package main

type RobotData struct {
	MUId           int    //配置的ID
	UserIdentifier string //服务器中实际的ID
	ActivityId     int64
	GameId         int32
	MUserName      string
	MPassWord      string
	mEggGameInfo   EggGameInfo
	MTaskId        []int
}

//砸金蛋玩家信息
type EggGameInfo struct {
	BatchId    int64 //批次id
	State      int64 //游戏状态
	JoinCount  int64 //参与人数
	SystemTime int64 //系统时间
	StartTime  int64 //游戏开始时间
	EndTime    int64 //游戏结束时间
}
