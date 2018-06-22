package main

import (
	"GStress/fsm"
	"GStress/logger"
	"GStress/net"

	sq "github.com/yireyun/go-queue"
)

type RobotEventData struct {
	MState string
	MEvent string
	any    interface{}
}

type Robot struct {
	mFsm               *fsm.FSM
	mTaskMng           *TaskMng
	NetClient          *net.NetClient
	mEventQueue        *sq.EsQueue
	mTimers            map[int32]int32
	mCurTaskId         int32
	mCurTaskType       int32
	mCurTaskStepReuslt int32
	mRobotDate         RobotEventData
	mIsWorkEnd         bool
}

func (r *Robot) Init() error {
	logger.Log4.Info("<ENTER> gamesvr start")
	//基础数据处理

	//建立状态机
	r.mFsm = fsm.NewFSM(
		"start",
		[]string{"start", "game", "end"},
	)

	//初始化定时器组
	r.mTimers = make(map[int32]int32)
	return nil
}
