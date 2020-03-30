package main

import (
	"GStress/fsm"
	"GStress/logger"
	"GStress/msg/ClubSubCmd"
	"GStress/msg/GameSubUserCmd"
	"GStress/msg/GateSubCmd"
	"GStress/msg/LobbySubCmd"
	"GStress/msg/LoginSubCmd"
	"GStress/msg/Main"

	//"GStress/msg/XueZhanMj"
	"GStress/net"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"

	sq "github.com/yireyun/go-queue"
)

const (
	RequestTimeOut = time.Second * 60
)

type TimerType int

const (
	TimerLoginRegister = iota
	TimerLoginLoginSvr
	TimerLoginLobbySvr
	TimerClubCreateClub
	TimerClubAddGold
	TimerClubJoinClub
	TimerClubCreateRoom
	TimerClubEnterRoom
)

type FsmState string

const (
	RobotStateNone    = "RobotStateNone"
	RobotStateInit    = "RobotStateInit"
	RobotStateTaskMng = "RobotStateTaskMng"
	RobotStateLogin   = "RobotStateLogin"
	RobotStateClub    = "RobotStateClub"
	RobotStateXzmj    = "RobotStateXzmj"
	RobotStateDdz     = "RobotStateDdz"
	RobotStateNewDdz  = "RobotStateNewDdz"
)

type FsmStateEvent string

const (
	RobotEventNone           = "RobotEventNone"
	RobotEventInit           = "enter"
	RobotEventQuit           = "leave"
	RobotEventDispatch       = "RobotEventDispatch"
	RobotEventRemoteMsg      = "RobotEventRemoteMsg"
	RobotEventSocketAbnormal = "RobotEventSocketAbnormal"
	RobotEventTimer          = "RobotEventTimer"
	RobotEventTaskAnalysis   = "RobotEventTaskAnalysis"

	//登陆
	RobotEventRegister       = "RobotEventRegister"
	RobotEventLoginLoginSvrd = "RobotEventLoginLoginSvrd"
	RobotEventLoginLobbySvrd = "RobotEventLoginLobbySvrd"

	//俱乐部
	RobotEventClubEnter   = "RobotEventClubEnter"
	RobotEventClubAddGold = "RobotEventClubAddGold"

	//血战麻将
	RobotEventXzmjCreateRoom = "RobotEventXzmjCreateRoom"
	RobotEventXzmjEnterRoom  = "RobotEventXzmjEnterRoom"
	RobotEventXzmjStartGame  = "RobotEventXzmjStartGame"
	RobotEventXzmjOperate    = "RobotEventXzmjOperate"

	//斗地主
	RobotEventDdzCreateRoom = "RobotEventDdzCreateRoom"
	RobotEventDdzEnterRoom  = "RobotEventDdzEnterRoom"
	RobotEventDdzStartGame  = "RobotEventDdzStartGame"
	RobotEventDdzOperate    = "RobotEventDdzOperate"

	//斗地主-新
	RobotEventDDZEnterRoomNew = "RobotEventDDZEnterRoomNew"
	RobotEventDdzStartGameNew = "RobotEventDdzStartGameNew"
	RobotEventDdzOperateNew   = "RobotEventDdzOperateNew"
)

type SystemConfig struct {
	MLoginSvrAddr string
	MLoginSvrPort int
	MGateSvrAddr  string
	MGateSvrPort  int
}

type RobotEventData struct {
	MState FsmState
	MEvent FsmStateEvent
	MData  interface{}
}

type TimertData struct {
	MTimerType TimerType
	MTimeValue time.Duration
}

type Robot struct {
	mFsm               *fsm.FSM
	mTaskMng           TaskMng
	mNetClient         *net.NetClient
	mEventQueue        *sq.EsQueue
	mTimers            map[TimerType]*time.Timer
	mCurTaskId         int
	mCurTaskType       TaskType
	mCurTaskStep       TaskStep
	mCurTaskStepReuslt TaskResult
	mRobotData         RobotData
	mIsWorkEnd         bool
	mSystemCfg         SystemConfig
	mIsLoginOk         bool
	mXzmjTable         XzmjTable
	mDdzTable          DdzTable
	mIsEnterRoom       bool
	mWg                sync.WaitGroup
}

func (r *Robot) Init(robotAttr RobotAttr, taskMap TaskMap, systemCfg ExcelCfg, startState string) error {
	logger.Log4.Debug("<ENTER> ")
	defer logger.Log4.Debug("<Leave> ")
	var lRetErr error
	//获取系统配置
	r.mSystemCfg.MLoginSvrAddr = systemCfg.MExcelRows[0]["LoginSvrAddr"]
	//获取端口
	loginSvrPort, err := strconv.Atoi(systemCfg.MExcelRows[0]["LoginSvrPort"])
	if err != nil {
		logger.Log4.Error("err:%s", err)
		lRetErr = errors.New("ERR_LoginPort")
		return lRetErr
	}
	r.mSystemCfg.MLoginSvrPort = loginSvrPort
	logger.Log4.Debug("UserName-%s: mSystemCfg:%v", robotAttr.MUserName, r.mSystemCfg)

	//初始化相关数据
	r.mCurTaskId = 0
	r.mCurTaskType = TaskTypeLogin
	r.mCurTaskStep = TaskStepNone
	r.mCurTaskStepReuslt = TaskResultNone
	r.mIsWorkEnd = false
	r.mIsLoginOk = false
	r.mIsEnterRoom = false
	r.mRobotData.Init(robotAttr)

	//初始化事件队列
	r.mEventQueue = sq.NewQueue(512)

	//初始任务管理器
	lRetErr = r.mTaskMng.Init(taskMap, robotAttr)
	if lRetErr != nil {
		logger.Log4.Error("lRetErr:%s", lRetErr)
		return lRetErr
	}
	//初始化状态机
	lRetErr = r.FsmInit(RobotStateInit)
	if lRetErr != nil {
		logger.Log4.Error("lRetErr:%s", lRetErr)
		return lRetErr
	}

	//初始化血战麻将
	r.mXzmjTable.Init(r)

	//初始化斗地主
	r.mDdzTable.Init(r)

	//初始化定时器组
	r.mTimers = make(map[TimerType]*time.Timer)

	//状态转移
	r.FsmTransferState(RobotStateTaskMng)
	return nil
}

func (r *Robot) Work() {
	logger.Log4.Debug("<ENTER> ")
	defer logger.Log4.Debug("<LEAVE> ")
	for {
		//派发事件
		r.DispatchEvent()
		if r.mIsWorkEnd == true {
			//任务结束
			logger.Log4.Debug("UserId-%d:work end", r.mRobotData.MUId)
			//test
			break
			//处理资源,如：关闭socket
			if r.mNetClient != nil {
				r.mNetClient.Close()
				r.mNetClient = nil
			}
			break
		}
	}
	r.mWg.Wait()

	return
}
func (r *Robot) DispatchEvent() {
	lEventCount := 0
	for {
		event, ok, quantity := r.mEventQueue.Get()
		if !ok {
			logger.Log4.Info("UserId-%d:no event,the mEventQueue Size is %d", r.mRobotData.MUId, quantity)
			break
		}
		logger.Log4.Info("UserId-%d:Get event success,the mEventQueue Size is %d", r.mRobotData.MUId, quantity)
		lEventCount++
		eventData := event.(*RobotEventData)
		if eventData.MState == RobotStateNone {
			//事件触发
			r.mFsm.Event(string(eventData.MEvent), eventData.MData)
		} else {
			r.mFsm.Transition(string(eventData.MState))
		}

	}
	if lEventCount == 0 {
		time.Sleep(500 * time.Millisecond)
		logger.Log4.Debug("UserId-%d:wait evet", r.mRobotData.MUId)
	}
}
func (r *Robot) FsmSendEvent(event FsmStateEvent, data interface{}) error {
	var eventdata RobotEventData
	eventdata.MState = RobotStateNone
	eventdata.MEvent = event
	eventdata.MData = data

	//插入事件到队列
	r.mEventQueue.Put(&eventdata)
	return nil
}
func (r *Robot) FsmTransferState(state FsmState) error {
	var eventdata RobotEventData
	eventdata.MState = state
	eventdata.MEvent = RobotEventNone
	eventdata.MData = nil
	//插入事件到队列
	r.mEventQueue.Put(&eventdata)
	/*
		_, ok, quantity := r.mEventQueue.Get()
		if !ok {
			logger.Log4.Info("UserId-%d:Get event Fail,the mEventQueue Size is %d", r.mRobotData.MUId, quantity)
		}
	*/
	return nil
}
func (r *Robot) FsmInit(startState FsmState) error {
	//建立状态机
	r.mFsm = fsm.NewFSM(
		RobotStateInit,
		[]string{
			RobotStateInit,
			RobotStateTaskMng,
			RobotStateLogin,
			RobotStateClub,
		},
	)
	//初始化状态计划机事件

	//RobotStateInit 状态
	r.mFsm.AddStateEvent(RobotStateInit, RobotEventInit, r.RobotStateInitEventInit)
	r.mFsm.AddStateEvent(RobotStateInit, RobotEventQuit, r.RobotStateInitEventQuit)
	//任务分配状态
	r.mFsm.AddStateEvent(RobotStateTaskMng, RobotEventInit, r.RobotStateTaskMngEventInit)
	r.mFsm.AddStateEvent(RobotStateTaskMng, RobotEventQuit, r.RobotStateTaskMngEventQuit)
	r.mFsm.AddStateEvent(RobotStateTaskMng, RobotEventDispatch, r.RobotStateTaskMngEventDispatch)
	//登陆状态
	r.mFsm.AddStateEvent(RobotStateLogin, RobotEventInit, r.RobotStateLoginEventInit)
	r.mFsm.AddStateEvent(RobotStateLogin, RobotEventQuit, r.RobotStateLoginEventQuit)
	r.mFsm.AddStateEvent(RobotStateLogin, RobotEventRemoteMsg, r.RobotStateLoginEventRemoteMsg)
	r.mFsm.AddStateEvent(RobotStateLogin, RobotEventSocketAbnormal, r.RobotStateLoginEventSocketAbnormal)
	r.mFsm.AddStateEvent(RobotStateLogin, RobotEventTimer, r.RobotStateLoginEventTimer)
	r.mFsm.AddStateEvent(RobotStateLogin, RobotEventTaskAnalysis, r.RobotStateLoginEventTaskAnalysis)
	r.mFsm.AddStateEvent(RobotStateLogin, RobotEventRegister, r.RobotStateLoginEventRegister)
	r.mFsm.AddStateEvent(RobotStateLogin, RobotEventLoginLoginSvrd, r.RobotStateLoginEventLoginLoginSvrd)
	r.mFsm.AddStateEvent(RobotStateLogin, RobotEventLoginLobbySvrd, r.RobotStateLoginEventLoginLobbySvrd)

	//微游戏
	r.mFsm.AddStateEvent(RobotStateClub, RobotEventInit, r.RobotStateClubEventInit)
	r.mFsm.AddStateEvent(RobotStateClub, RobotEventQuit, r.RobotStateClubEventQuit)
	r.mFsm.AddStateEvent(RobotStateClub, RobotEventRemoteMsg, r.RobotStateClubEventRemoteMsg)
	r.mFsm.AddStateEvent(RobotStateClub, RobotEventSocketAbnormal, r.RobotStateClubEventSocketAbnormal)
	r.mFsm.AddStateEvent(RobotStateClub, RobotEventTimer, r.RobotStateClubEventTimer)
	r.mFsm.AddStateEvent(RobotStateClub, RobotEventTaskAnalysis, r.RobotStateClubEventTaskAnalysis)
	r.mFsm.AddStateEvent(RobotStateClub, RobotEventClubEnter, r.RobotStateClubEventClubEnter)
	r.mFsm.AddStateEvent(RobotStateClub, RobotEventClubAddGold, r.RobotStateClubEventAddGold)

	//血战麻将
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventInit, r.RobotStateXzmjEventInit)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventQuit, r.RobotStateXzmjEventQuit)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventRemoteMsg, r.RobotStateXzmjEventRemoteMsg)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventSocketAbnormal, r.RobotStateXzmjEventSocketAbnormal)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventTimer, r.RobotStateXzmjEventTimer)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventTaskAnalysis, r.RobotStateXzmjEventTaskAnalysis)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventXzmjCreateRoom, r.RobotStateXzmjEventCreateRoom)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventXzmjEnterRoom, r.RobotStateXzmjEventEnterRoom)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventXzmjStartGame, r.RobotStateXzmjEventStartRoom)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventXzmjOperate, r.RobotStateXzmjEventOperate)

	//斗地主
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventInit, r.RobotStateDdzEventInit)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventQuit, r.RobotStateDdzEventQuit)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventRemoteMsg, r.RobotStateDdzEventRemoteMsg)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventSocketAbnormal, r.RobotStateDdzEventSocketAbnormal)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventTimer, r.RobotStateDdzEventTimer)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventTaskAnalysis, r.RobotStateDdzEventTaskAnalysis)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventDdzCreateRoom, r.RobotStateDdzEventCreateRoom)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventDdzEnterRoom, r.RobotStateDdzEventEnterRoom)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventDdzStartGame, r.RobotStateDdzEventStartRoom)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventDdzOperate, r.RobotStateDdzEventOperate)
	//......

	//新版斗地主
	r.mFsm.AddStateEvent(RobotStateNewDdz, RobotEventInit, r.RobotStateDdzEventInitNew)
	r.mFsm.AddStateEvent(RobotStateNewDdz, RobotEventQuit, r.RobotStateDdzEventQuitNew)
	r.mFsm.AddStateEvent(RobotStateNewDdz, RobotEventRemoteMsg, r.RobotStateDdzEventRemoteMsgNew)
	r.mFsm.AddStateEvent(RobotStateNewDdz, RobotEventSocketAbnormal, r.RobotStateDdzEventSocketAbnormalNew)
	r.mFsm.AddStateEvent(RobotStateNewDdz, RobotEventTimer, r.RobotStateDdzEventTimerNew)
	r.mFsm.AddStateEvent(RobotStateNewDdz, RobotEventTaskAnalysis, r.RobotStateDdzEventTaskAnalysisNew)
	r.mFsm.AddStateEvent(RobotStateNewDdz, RobotEventDDZEnterRoomNew, r.RobotStateDdzEventEnterRoomNew)
	r.mFsm.AddStateEvent(RobotStateNewDdz, RobotEventDdzStartGameNew, r.RobotStateDdzEventStartGameNew)
	r.mFsm.AddStateEvent(RobotStateNewDdz, RobotEventDdzOperateNew, r.RobotStateDdzEventOperateNew)

	return nil
}

//初始化状态
func (r *Robot) RobotStateInitEventInit(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

}

func (r *Robot) RobotStateInitEventQuit(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

}

//任务管理状态
func (r *Robot) RobotStateTaskMngEventInit(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	r.FsmSendEvent(RobotEventDispatch, nil)

}

func (r *Robot) RobotStateTaskMngEventQuit(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

}
func (r *Robot) RobotStateTaskMngEventDispatch(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	taskType, err := r.mTaskMng.DispatchTask()
	if taskType == TaskTypeNone {
		r.mIsWorkEnd = true
		logger.Log4.Debug("<ENTER> :UserId-%d: work end,err:%s", r.mRobotData.MUId, err)
		return
	}

	r.mCurTaskId, _ = r.mTaskMng.GetCurTaskId()
	r.mCurTaskType = taskType
	r.mCurTaskStep = TaskStepNone
	r.mCurTaskStepReuslt = TaskResultNone

	logger.Log4.Debug("<ENTER> :UserId-%d:get task-%d", r.mRobotData.MUId, r.mCurTaskId)
	switch taskType {
	case TaskTypeLogin:
		r.FsmTransferState(RobotStateLogin)
		break
	case TaskTypeClub:
		r.FsmTransferState(RobotStateClub)
		break
	case TaskTypeXzmj:
		r.FsmTransferState(RobotStateXzmj)
		break
	case TaskTypeDdz:
		r.FsmTransferState(RobotStateDdz)
		break
	case TaskTypeDdznNew:
		r.FsmTransferState(RobotStateNewDdz)
		break
	default:
		r.FsmSendEvent(RobotEventDispatch, nil)
		break

	}
	return
}

//登陆状态
func (r *Robot) RobotStateLoginEventInit(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	r.FsmSendEvent(RobotEventTaskAnalysis, nil)
}

func (r *Robot) RobotStateLoginEventQuit(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mIsLoginOk == false {
		//处理资源,如：关闭socket
		if r.mNetClient != nil {
			r.mNetClient.Close()
			r.mNetClient = nil
		}
	}

}

//登陆状态：处理远程消息
func (r *Robot) RobotStateLoginEventRemoteMsg(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if len(e.Args) == 0 {
		logger.Log4.Error("UserId-%d: no Remote Msg", r.mRobotData.MUId)
		return
	}
	msgHead := e.Args[0].(*net.MsgHead)
	switch msgHead.MMainCmd {
	case int16(Main.EnMainCmdID_LOGIN_MAIN_CMD):
		r.HandelLoginMainMsg(msgHead)
		break
	case int16(Main.EnMainCmdID_GATE_MAIN_CMD):
		r.HandelGateMainMsg(msgHead)
		break
	case int16(Main.EnMainCmdID_LOBBY_MAIN_CMD):
		r.HandelLobbyMainMsg(msgHead)
		break
	case int16(Main.EnMainCmdID_CLUB_MAIN_CMD):
		r.HandelClubMainMsg(msgHead)
		break
	default:
		break
	}

}

func (r *Robot) HandelGameMainUserMsg(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}
	switch msgHead.MSubCmd {
	case int16(GameSubUserCmd.EnSubCmdID_GAME_SUB_SC_USER_LEAVE_ROOM_NOTIFY):
		r.HandelUserLeaveRoomNotify(msgHead)
		break
	case int16(GameSubUserCmd.EnSubCmdID_GAME_SUB_SC_USER_ROOM_INFO_NOTIFY):
		r.HandelUserLeaveRoom(msgHead)
		break
	default:
		break
	}

	return
}

func (r *Robot) HandelUserLeaveRoomNotify(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}

	leaveRoomNotify := &GameSubUserCmd.SCUserLeaveRoomNotify{}
	err := proto.Unmarshal(msgHead.MData, leaveRoomNotify)
	if err != nil {
		logger.Log4.Debug("unmarshal SCUserLeaveRoomNotify error: %s", err)
	}
	logger.Log4.Debug("leaveRoomNotify: %+v", leaveRoomNotify)

	if r.mCurTaskType == TaskTypeXzmj || r.mCurTaskType == TaskTypeDdz {

		r.mCurTaskStepReuslt = TaskResultSuccess
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	}

	return
}

func (r *Robot) HandelClubMainMsg(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}
	switch msgHead.MSubCmd {
	case int16(ClubSubCmd.EnServerSubCmdID_SC_CLUB_BASE_INFO_ON_LONGIN):
		r.HandelClubBaseInfo(msgHead)
		break
	case int16(ClubSubCmd.EnServerSubCmdID_SC_CREATE_CLUB):
		r.HandelClubCreateClubResult(msgHead)
		break
	case int16(ClubSubCmd.EnServerSubCmdID_SC_ADD_CLUB_USER_NOTIFY):
		r.HandelClubAddUserNotify(msgHead)
		break
	case int16(ClubSubCmd.EnServerSubCmdID_SC_JOIN_CLUB):
		r.HandelClubJoinClubResult(msgHead)
		break
	case int16(ClubSubCmd.EnServerSubCmdID_SC_CLUB_CREATE_TABLE):
		r.HandelClubCreateTableResult(msgHead)
		break
	case int16(ClubSubCmd.EnServerSubCmdID_SC_CLUB_ENTER_ROOM):
		r.HandelClubEnterTableResult(msgHead)
		break
	default:
		break
	}

	return
}

/*
//SC_CLUB_BASE_INFO_ON_LONGIN         = 114;   //服务端主动推送俱乐部基本信息在登陆时
message SC_ClubBaseInfoOnLogin
{
	required int64             iClubID         = 1;         //创建的俱乐部ID
	required string            strClubName     = 2;         //俱乐部名称
	required int64             iUserLv         = 4;         //玩家权限 0 普通 1 管理员 2 创建者
	required string            strNotice       = 5;       //公告
	optional string            strQrTicket     = 6;       //俱乐部二维码票据
	optional string            strQrUrl        = 7;       //俱乐部二维码地址
	optional int64             iQrExpire       = 8;       //俱乐部二维码有效期
	optional int64             iQrCreateTime   = 9;       //俱乐部二维码创建时间
	optional int32             iCheatLimit     = 10;      //俱乐部聊天限制
	optional string            strClubKeFu     = 11;      //俱乐部客服
	optional string            strClubBrief    = 12;      //俱乐部简介
};
*/

func (r *Robot) HandelClubBaseInfo(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}

	clubBaseInfo := &ClubSubCmd.SC_ClubBaseInfoOnLogin{}
	err := proto.Unmarshal(msgHead.MData, clubBaseInfo)
	if err != nil {
		logger.Log4.Debug("unmarshal SC_ClubBaseInfoOnLogin error: %s", err)
	}
	logger.Log4.Debug("clubBaseInfo: %+v", clubBaseInfo)

	r.mRobotData.MClubData.MClubId = int(clubBaseInfo.GetIClubID())
	r.mRobotData.MClubData.MClubName = clubBaseInfo.GetStrClubName()
	r.mRobotData.MClubData.MUserLv = int(clubBaseInfo.GetIUserLv())

	if r.mCurTaskType == TaskTypeLogin {
		//登陆状态：微游戏基础信息收到后，则整个登陆完成，取消定时器
		r.CancelTimer(TimerLoginLobbySvr)
		r.mCurTaskStepReuslt = TaskResultSuccess
		r.mIsLoginOk = true
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	}

	return
}

/*
//SC_CREATE_CLUB						= 101;	//服务端创建俱乐部返回
message SC_CreateClub
{
	optional ST_ClubInfo        stClubInfo = 1;    //俱乐部信息
	required string             strMsg     = 2;    //创建返回消息
};
*/
func (r *Robot) HandelClubCreateClubResult(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}

	//取消定时器
	r.CancelTimer(TimerClubCreateClub)
	createClubReslut := &ClubSubCmd.SC_CreateClub{}
	err := proto.Unmarshal(msgHead.MData, createClubReslut)
	if err != nil {
		logger.Log4.Debug("unmarshal SC_CreateClub error: %s", err)
	}
	logger.Log4.Debug("clubBaseInfo: %+v", createClubReslut)

	stClubInfo := createClubReslut.GetStClubInfo()
	if stClubInfo == nil {

		r.mCurTaskStepReuslt = TaskResultClub_SendRequestCreateClubReponseFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Debug("UserId-%d::create Club Fail: %s", r.mRobotData.MUId, createClubReslut.GetStrMsg())
		return
	}

	lBaseInfo := stClubInfo.GetStClubBaseInfo()
	r.mRobotData.MClubData.MClubId = int(lBaseInfo.GetIClubID())
	r.mRobotData.MClubData.MClubName = lBaseInfo.GetStrClubName()
	r.mRobotData.MClubData.MUserLv = 2

	r.mCurTaskStepReuslt = TaskResultSuccess
	r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	logger.Log4.Debug("UserId-%d::create Club success: %s", r.mRobotData.MUId, createClubReslut.GetStrMsg())

	return
}

/*
//SC_ADD_CLUB_USER_NOTIFY             = 117;   //服务端通知申请加入俱乐部玩家管理员审核的结果，同意的
message SC_AddClubUserNotify
{
	optional SC_ClubBaseInfoOnLogin   stInfo  = 1;         //登录基本信息
	required int32                    iResult = 2;         //结果ID：0 加入成功 其它：错误读取消息
	required string                   strMsg  = 3;         //结果消息
};


*/
func (r *Robot) HandelClubAddUserNotify(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}

	addClubUserNotify := &ClubSubCmd.SC_AddClubUserNotify{}
	err := proto.Unmarshal(msgHead.MData, addClubUserNotify)
	if err != nil {
		logger.Log4.Debug("unmarshal SC_AddClubUserNotify error: %s", err)
	}
	logger.Log4.Debug("clubBaseInfo: %+v", addClubUserNotify)

	if addClubUserNotify.GetIResult() != 0 {
		logger.Log4.Debug("<ENTER> :UserId-%d: enter club fail,reason:%s", r.mRobotData.MUId, addClubUserNotify.GetStrMsg())
		return
	}

	stInfo := addClubUserNotify.GetStInfo()

	r.mRobotData.MClubData.MClubId = int(stInfo.GetIClubID())
	r.mRobotData.MClubData.MClubName = stInfo.GetStrClubName()
	r.mRobotData.MClubData.MUserLv = int(stInfo.GetIUserLv())
	logger.Log4.Debug("<ENTER> :UserId-%d: enter club success", r.mRobotData.MUId)

	return
}

/*
 message SC_JoinClub
{
	required int32             iResult = 1;         //结果ID：0 加入成功 其它：错误读取消息
	required string            strMsg  = 2;         //结果消息
};
*/

func (r *Robot) HandelClubJoinClubResult(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}

	//取消定时器
	r.CancelTimer(TimerClubJoinClub)

	joinClubReslut := &ClubSubCmd.SC_JoinClub{}
	err := proto.Unmarshal(msgHead.MData, joinClubReslut)
	if err != nil {
		logger.Log4.Debug("unmarshal SC_JoinClub error: %s", err)
	}
	logger.Log4.Debug("clubBaseInfo: %+v", joinClubReslut)

	lresult := joinClubReslut.GetIResult()
	if lresult != 0 {

		r.mCurTaskStepReuslt = TaskResultClub_SendRequestJoinClubReponseFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Debug("UserId-%d::join Club Fail: %s", r.mRobotData.MUId, joinClubReslut.GetStrMsg())
		return
	}

	r.mCurTaskStepReuslt = TaskResultSuccess
	r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	logger.Log4.Debug("UserId-%d::join Club Fail: %s", r.mRobotData.MUId, joinClubReslut.GetStrMsg())

	return
}

//创建桌子结果返回
func (r *Robot) HandelClubCreateTableResult(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}

	if r.mCurTaskType != TaskTypeXzmj && r.mCurTaskType != TaskTypeDdz {
		return
	}

	//取消定时器
	r.CancelTimer(TimerClubCreateRoom)

	createTableReslut := &ClubSubCmd.SC_ClubCreateTable{}
	err := proto.Unmarshal(msgHead.MData, createTableReslut)
	if err != nil {
		logger.Log4.Debug("unmarshal SC_ClubCreateTable error: %s", err)
	}
	logger.Log4.Debug("createTableReslut: %+v", createTableReslut)

	if createTableReslut.GetTableId() < 1000000 {
		logger.Log4.Debug("UserId-%d: not myself create table", r.mRobotData.MUId)
		return
	}
	lresult := createTableReslut.GetResult()
	if lresult != 0 {

		r.mCurTaskStepReuslt = TaskResultClub_SendRequestCreateRoomReponseFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Debug("UserId-%d::create table Fail: %s", r.mRobotData.MUId, createTableReslut.GetResultStr())
		return
	}

	r.mCurTaskStepReuslt = TaskResultSuccess
	r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	logger.Log4.Debug("UserId-%d::create table Success: %s", r.mRobotData.MUId, createTableReslut.GetResultStr())

	return
}

//创建桌子结果返回
func (r *Robot) HandelClubEnterTableResult(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}

	if r.mCurTaskStep != TaskStepXzmjEnterRoom && r.mCurTaskStep != TaskStepDdzEnterRoom {
		return
	}
	if r.mIsEnterRoom == true {
		return
	}

	//取消定时器
	r.CancelTimer(TimerClubEnterRoom)

	enterTableReslut := &ClubSubCmd.SC_ClubEnterRoom{}
	err := proto.Unmarshal(msgHead.MData, enterTableReslut)
	if err != nil {
		logger.Log4.Debug("unmarshal SC_ClubEnterRoom error: %s", err)
	}
	logger.Log4.Debug("enterTableReslut: %+v", enterTableReslut)

	r.mIsEnterRoom = true
	lresult := enterTableReslut.GetResult()
	if lresult != 0 {

		r.mCurTaskStepReuslt = TaskResultClub_SendRequestEnterRoomReponseFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Debug("UserId-%d::enter table Fail: %s", r.mRobotData.MUId, enterTableReslut.GetResultStr())
		return
	}

	r.mCurTaskStepReuslt = TaskResultSuccess
	r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	logger.Log4.Debug("UserId-%d::enter table Success: %s", r.mRobotData.MUId, enterTableReslut.GetResultStr())

	return
}

func (r *Robot) HandelLobbyMainMsg(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}
	switch msgHead.MSubCmd {
	case int16(LobbySubCmd.EnSubCmdID_USER_MAIN_INFO_SUB_CMD):
		r.HandelUserMainInfo(msgHead)
		break
	case int16(LobbySubCmd.EnSubCmdID_RETURN_DIAMOND_TO_GOLD_SUB_CMD):
		r.HandelUserDiamondToGoldReturn(msgHead)
		break
	case int16(LobbySubCmd.EnSubCmdID_UPDATE_USER_ATT_SUB_CMD):
		r.HandelUserUpdateAtt(msgHead)
		break
	default:
		break
	}

	return
}

/*
//USER_MAIN_INFO_SUB_CMD                      	              = 1;	//服务器发送大厅用户主要信息给客户端
//登陆成功
message SC_Logon_Success
{
	required string szNickName = 1;	             							//昵称
	required string iFaceID = 2;									//头像索引
	required int32 iGender = 3;									//用户性别(0保密, 1男, 2女)
	required int32 iUserID = 5;									//用户 ID
	required int32 iAccid = 6 ;	        						//用户别名id，用于显示
    required int32 iTicket = 9;								        //用户彩票
	required int64 iGold = 11;									//金币数量
	required int32 iDiamond = 13;									//用户钻石
	required int64 i64RegistDate = 14;								//注册日期
	required string szDescription = 17 [default = ""];                                              //个人说明
	required string szImage = 18 [default = ""];                                                   //头像
	required string szMobile = 19 [default = ""];                                                   //手机号
	repeated int32 iKindID = 26;							                //游戏ID列表
	required int64 iServerTime = 27;						        	//当前服务器时间(utc时间)
	required string szLocal = 34;                                                                   //所在地
	required string szLastLogonIP = 35;								 //最后登录IP
	optional int32 iScoreRoomID = 44;								 //创建房间的ID
	required int64 iInsureGold = 45;													 	//保险箱金币
	required int32 iModifyNickTimes = 46;					 	//修改昵称次数
	required int32 iGetRelifTimes = 47;							//今日已领取的救济金次数
	required int32 iGetRelifTotalTimes = 48;					//总共领取的救济金次数
	required int32 iMaxGetRelifTimes = 49;					//用户最大可领取次数
	optional int32 iFirstRecharge = 50;					//用户是否有第一次充值 1 没有充值过 2 已经失去首充奖励机会 其它无效
	optional int64 SignCount  = 51;  //签到次数
	optional int64 LastSignTime	  = 52;  //最后一次签到时间
	optional int32 SignEnable  = 53;  //签到功能开关,1 开启，0 关闭。
	optional int32 RechargeFirstEnable  = 54;  //首充功能开关,1 开启，0 关闭。
	optional int32 RechargeExtraEnable  = 55;  //额外赠送功能开关,1 开启，0 关闭。
	optional int32 iVipLv  = 56;  //玩家VIP等级
	optional string strRealName  = 57;  //玩家真实姓名
	optional string strIDCard  = 58;  //玩家身份证号
}


*/

func (r *Robot) HandelUserMainInfo(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}

	logonSuccessInfo := &LobbySubCmd.SC_Logon_Success{}
	err := proto.Unmarshal(msgHead.MData, logonSuccessInfo)
	if err != nil {
		logger.Log4.Debug("unmarshal SC_Logon_Success error: %s", err)
	}
	logger.Log4.Debug("logonSuccessInfo: %+v", logonSuccessInfo)

	r.mRobotData.MUserGold = int(logonSuccessInfo.GetIGold())
	r.mRobotData.MUserDiamond = int(logonSuccessInfo.GetIDiamond())

	return

}
func (r *Robot) HandelUserDiamondToGoldReturn(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}

	returnDiamondToGold := &LobbySubCmd.SC_ReturnDiamondToGold{}
	err := proto.Unmarshal(msgHead.MData, returnDiamondToGold)
	if err != nil {
		logger.Log4.Debug("unmarshal SC_ReturnDiamondToGold error: %s", err)
	}
	logger.Log4.Debug("SC_ReturnDiamondToGold: %+v", returnDiamondToGold)

	if returnDiamondToGold.GetResult() == 0 {
		r.mCurTaskStepReuslt = TaskResultSuccess
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	} else {
		r.mCurTaskStepReuslt = TaskResultClub_SendRequestAddGoldReponseFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	}
	logger.Log4.Debug("UserId-%d: add gold result:%d, retsultStr:%s",
		r.mRobotData.MUId, returnDiamondToGold.GetResult(), returnDiamondToGold.GetResultStr())

	return

}
func (r *Robot) HandelUserUpdateAtt(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}

	userAtt := &LobbySubCmd.SC_Update_User_Att{}
	err := proto.Unmarshal(msgHead.MData, userAtt)
	if err != nil {
		logger.Log4.Debug("unmarshal SC_Update_User_Att error: %s", err)
	}
	logger.Log4.Debug("userAtt: %+v", userAtt)

	r.mRobotData.MUserGold = int(userAtt.GetIGold())
	r.mRobotData.MUserDiamond = int(userAtt.GetIDiamond())
	logger.Log4.Debug("UserId-%d: Gold:%d, Diamond:%d",
		r.mRobotData.MUId, r.mRobotData.MUserGold, r.mRobotData.MUserDiamond)

	return

}

func (r *Robot) HandelGateMainMsg(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}
	switch msgHead.MSubCmd {
	case int16(GateSubCmd.EnSubCmdID_LOGIN_GATE_RESULT_SUB_CMD):
		r.HandelLoginGateResult(msgHead)
		break
	default:
		break
	}

	return
}

func (r *Robot) HandelLoginGateResult(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}

	loginGateResultInfo := &GateSubCmd.LoginGateResultInfo{}
	err := proto.Unmarshal(msgHead.MData, loginGateResultInfo)
	if err != nil {
		logger.Log4.Debug("unmarshal LoginGateResultInfo error: %s", err)
	}
	logger.Log4.Debug("loginGateResultInfo: %+v", loginGateResultInfo)
	lRet := loginGateResultInfo.GetRetcode()
	if lRet == 0 {
		//网关登陆成功，还需要等待大厅玩家基础信息和微游戏的基础信息
		logger.Log4.Debug("UserId-%d: login gate  success", r.mRobotData.MUId)

	} else {

		//确认登陆网关失败后，则关闭定时器，
		r.CancelTimer(TimerLoginLobbySvr)
		r.mCurTaskStepReuslt = TaskResultLogin_Lobbysvr_LoginResponseFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Debug("UserId-%d: login gate fail, the record:%d", r.mRobotData.MUId, lRet)
	}

	return

}

func (r *Robot) HandelLoginMainMsg(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}
	switch msgHead.MSubCmd {
	case int16(LoginSubCmd.EnSubCmdID_RETURN_LOGININFO_SUB_CMD):
		r.HandelReturnLoginInfo(msgHead)
		break
	case int16(LoginSubCmd.EnSubCmdID_LOGIN_ERROR_CODE_SUB_CMD):
		r.HandelLoginErrorCode(msgHead)
		break
	case int16(LoginSubCmd.EnSubCmdID_RETURN_CREATE_ACCOUNT_RESULT_SUB_CMD):
		r.HandelCreateAccountResult(msgHead)
		break
	default:
		break
	}

	return
}
func (r *Robot) HandelCreateAccountResult(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}

	//收到响应取消定时器
	r.CancelTimer(TimerLoginRegister)

	createAccountResult := &LoginSubCmd.SC_Create_Account_Result{}
	err := proto.Unmarshal(msgHead.MData, createAccountResult)
	if err != nil {
		logger.Log4.Debug("unmarshal SC_Create_Account_Result error: %s", err)
	}
	logger.Log4.Debug("loginErrorCode: %+v", createAccountResult)
	lRet := createAccountResult.GetRetCode()
	if lRet == 0 {
		logger.Log4.Debug("UserId-%d: register success", r.mRobotData.MUId)
	}
	if lRet == 5 {
		logger.Log4.Debug("UserId-%d: register exist", r.mRobotData.MUId)
	}
	if lRet != 0 && lRet != 5 {
		r.mCurTaskStepReuslt = TaskResultLogin_Loginsvr_RegisterResponseFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	} else {
		r.mCurTaskStepReuslt = TaskResultSuccess
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	}

	return

}

func (r *Robot) HandelReturnLoginInfo(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}
	//收到响应取消定时器
	r.CancelTimer(TimerLoginLoginSvr)

	loginReturnInfo := &LoginSubCmd.LoginReturnInfo{}
	err := proto.Unmarshal(msgHead.MData, loginReturnInfo)
	if err != nil {
		logger.Log4.Debug("unmarshal LoginReturnInfo error: %s", err)
	}
	logger.Log4.Debug("loginReturnInfo: %+v", loginReturnInfo)

	/*
	   message LoginReturnInfo
	   {
	   	required int32  userid = 2 ;	        	//账号对应id
	   	required string lobbyAddress = 3 ;		//网关地址
	   	required int32 lobbyPort = 4 ;          //网关端口端口
	   	required string token = 5;	        	//登录token
	   	required string account  = 6;	        // 登录对应的账号
	   	required int32 accountType  = 7;	        // 登录类型,0-普通用户， 1-游客用户， 2-微信用户
	   }
	*/
	r.mRobotData.MUserId = int(loginReturnInfo.GetUserid())
	r.mRobotData.MToken = loginReturnInfo.GetToken()
	r.mSystemCfg.MGateSvrAddr = loginReturnInfo.GetLobbyAddress()
	r.mSystemCfg.MGateSvrPort = int(loginReturnInfo.GetLobbyPort())

	r.mCurTaskStepReuslt = TaskResultSuccess
	r.FsmSendEvent(RobotEventTaskAnalysis, nil)
}

func (r *Robot) HandelLoginErrorCode(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}

	//收到响应取消定时器
	r.CancelTimer(TimerLoginLoginSvr)

	loginErrorCode := &LoginSubCmd.LoginErrorCode{}
	err := proto.Unmarshal(msgHead.MData, loginErrorCode)
	if err != nil {
		logger.Log4.Debug("unmarshal LoginErrorCode error: %s", err)
	}
	logger.Log4.Debug("loginErrorCode: %+v", loginErrorCode)
	r.mCurTaskStepReuslt = TaskResultLogin_Loginsvr_LoginResponseFail
	r.FsmSendEvent(RobotEventTaskAnalysis, nil)
}

//登陆状态：处理网络异常
func (r *Robot) RobotStateLoginEventSocketAbnormal(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mCurTaskStepReuslt == TaskResultNone {
		r.mCurTaskStepReuslt = TaskResultSocketErr
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	}
	if r.mNetClient != nil {
		r.mNetClient.Close()
		r.mNetClient = nil
	}

}

//登陆状态：处理定时事件
func (r *Robot) RobotStateLoginEventTimer(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	if len(e.Args) == 0 {
		logger.Log4.Error("UserId-%d: no Remote Msg", r.mRobotData.MUId)
		return
	}
	timerData := e.Args[0].(*TimertData)
	switch timerData.MTimerType {
	case TimerLoginLoginSvr:
		r.mCurTaskStepReuslt = TaskResultLogin_Loginsvr_SendLoginRequestTimeOut
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		break
	case TimerLoginRegister:
		r.mCurTaskStepReuslt = TaskResultLogin_Loginsvr_SendRegisterRequestTimeOut
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		break
	case TimerLoginLobbySvr:
		r.mCurTaskStepReuslt = TaskResultLogin_Lobbysvr_SendLoginRequestTimeOut
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		break

	default:
		break
	}
}

//登陆状态：处理任务解析
func (r *Robot) RobotStateLoginEventTaskAnalysis(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	if r.mCurTaskStep != TaskStepNone {
		r.mTaskMng.ReportTaskStepCompleteResult(r.mCurTaskId, r.mCurTaskType, r.mCurTaskStep, r.mCurTaskStepReuslt)
	}
	var taskStep TaskStep = TaskStepNone
	taskStep, _ = r.mTaskMng.DispatchTaskStep()
	if taskStep == TaskStepNone {
		//当前任务步骤已作完，跳到任务管理状态继续分配任务
		r.FsmTransferState(RobotStateTaskMng)
		return
	}
	r.mCurTaskStep = taskStep
	r.mCurTaskStepReuslt = TaskResultNone

	switch r.mCurTaskStep {
	case TaskStepRegister:
		r.FsmSendEvent(RobotEventRegister, nil)
		break
	case TaskStepLoginLoginSvr:
		r.FsmSendEvent(RobotEventLoginLoginSvrd, nil)
		break
	case TaskStepLoginLobbySvr:
		r.FsmSendEvent(RobotEventLoginLobbySvrd, nil)
		break
	default:
		break
	}
	return
}

func (r *Robot) RobotStateLoginEventRegister(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	err := r.ConnectLoginSvr()
	if err != nil {
		//网络连接失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultSocketErr
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	}
	err = r.RequestRegister()
	if err != nil {
		//网络连接失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultLogin_Loginsvr_SendRegisterRequestFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	} else {
		//设置请求定时器
		r.SetTimer(TimerLoginRegister, RequestTimeOut)

	}
	return
}

func (r *Robot) RequestRegister() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}
	md5Ctx := md5.New()
	md5Ctx.Write([]byte(r.mRobotData.MPassWord))
	cipherStr := md5Ctx.Sum(nil)
	secret := hex.EncodeToString(cipherStr)

	m := &LoginSubCmd.CS_Request_Mobile_Account{
		PhoneNumber:  proto.String(r.mRobotData.MUserName),
		Passwd:       proto.String(secret),
		Token:        proto.String(""),
		UiSpread:     proto.Uint32(0),
		Devicestring: proto.String(r.mRobotData.MDevice),
		Packageflag:  proto.String(r.mRobotData.MPackageflag),
		IDevice:      proto.Int32(2),
		ClientIp:     proto.String(r.mRobotData.MClientIp),
	}

	err := r.mNetClient.SenMsg(int16(Main.EnMainCmdID_LOGIN_MAIN_CMD), int16(LoginSubCmd.EnSubCmdID_REQUEST_CREATE_MOBILE_ACCOUNT_SUB_CMD), m)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
		return errors.New("ERR_NET_SEND_FAIL")
	}
	return nil
}

func (r *Robot) RequestXzmjLeaveRoom() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}

	m := &GameSubUserCmd.CSUserLeaveRoomRqst{
		RoomId: proto.Int32(int32(r.mRobotData.MXzmjRoomId)),
	}

	logger.Log4.Debug("CSUserLeaveRoomRqst: %+v", m)

	err := r.mNetClient.SenGameMsg(int16(Main.EnMainCmdID_GAME_MAIN_USER),
		int16(GameSubUserCmd.EnSubCmdID_GAME_SUB_CS_USER_LEAVE_ROOM_RQST), int16(r.mRobotData.MXzmjRoomId), m)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
		return errors.New("ERR_NET_SEND_FAIL")
	}
	return nil
}

func (r *Robot) RequestDdzLeaveRoom() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}

	m := &GameSubUserCmd.CSUserLeaveRoomRqst{
		RoomId: proto.Int32(int32(r.mRobotData.MDdzRoomId)),
	}

	logger.Log4.Debug("CSUserLeaveRoomRqst: %+v", m)

	err := r.mNetClient.SenGameMsg(int16(Main.EnMainCmdID_GAME_MAIN_USER),
		int16(GameSubUserCmd.EnSubCmdID_GAME_SUB_CS_USER_LEAVE_ROOM_RQST), int16(r.mRobotData.MDdzRoomId), m)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
		return errors.New("ERR_NET_SEND_FAIL")
	}
	return nil
}

//登陆状态：处理登陆loginsvr
func (r *Robot) RobotStateLoginEventLoginLoginSvrd(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	err := r.ConnectLoginSvr()
	if err != nil {
		//网络连接失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultLogin_Loginsvr_ConnectFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	}
	err = r.RequestLoginSvr()
	if err != nil {
		//网络连接失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultLogin_Loginsvr_SendLoginRequestFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	} else {
		//设置请求定时器
		r.SetTimer(TimerLoginLoginSvr, RequestTimeOut)

	}
	return
}

func (r *Robot) RequestLoginSvr() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}
	md5Ctx := md5.New()
	md5Ctx.Write([]byte(r.mRobotData.MPassWord))
	cipherStr := md5Ctx.Sum(nil)
	secret := hex.EncodeToString(cipherStr)

	m := &LoginSubCmd.LoginRequestInfo{
		Account:      proto.String(r.mRobotData.MUserName),
		Passwd:       proto.String(secret),
		DeviceString: proto.String(r.mRobotData.MDevice),
		Packageflag:  proto.String(r.mRobotData.MPackageflag),
	}

	err := r.mNetClient.SenMsg(int16(Main.EnMainCmdID_LOGIN_MAIN_CMD), int16(LoginSubCmd.EnSubCmdID_REQUEST_LOGIN_SUB_CMD), m)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
		return errors.New("ERR_NET_SEND_FAIL")
	}
	return nil
}

func (r *Robot) ConnectLoginSvr() error {
	if r.mNetClient != nil {
		r.mNetClient.Close()
		r.mNetClient = nil
	}
	tmp0 := r.mSystemCfg.MLoginSvrAddr
	tmp1 := strconv.Itoa(r.mSystemCfg.MLoginSvrPort)
	tmp := tmp0 + ":" + tmp1
	lnetClient, err := net.NewNetClient(tmp, "")
	if err != nil {
		logger.Log4.Error("UserId-%d: connet login svr err:%s", r.mRobotData.MUId, err)
		return err
	}
	r.mNetClient = lnetClient
	go func() {
		msgHead, err := r.mNetClient.ReadMsg()
		if err != nil {
			logger.Log4.Error("UserId-%d: ReadMsg err:%s", r.mRobotData.MUId, err)
			r.FsmSendEvent(RobotEventSocketAbnormal, nil)
			return
		} else {
			logger.Log4.Debug("UserId-%d: msgHead :%v", r.mRobotData.MUId, msgHead)
			r.FsmSendEvent(RobotEventRemoteMsg, msgHead)
		}

	}()
	return nil

}

func (r *Robot) ConnectGateSvr() error {
	if r.mNetClient != nil {
		r.mNetClient.Close()
		r.mNetClient = nil
	}
	tmp0 := r.mSystemCfg.MGateSvrAddr
	tmp1 := strconv.Itoa(r.mSystemCfg.MGateSvrPort)
	tmp := tmp0 + ":" + tmp1
	lnetClient, err := net.NewNetClient(tmp, "")
	if err != nil {
		logger.Log4.Error("UserId-%d: connet Gate svr err:%s", r.mRobotData.MUId, err)
		return err
	}
	r.mNetClient = lnetClient
	go func() {
		r.mWg.Add(1)
		for {
			if r.mNetClient == nil {
				break
			}
			msgHead, err := r.mNetClient.ReadMsg()
			if err != nil {
				logger.Log4.Error("UserId-%d: Gate ReadMsg err:%s", r.mRobotData.MUId, err)
				r.FsmSendEvent(RobotEventSocketAbnormal, nil)
				break
			} else {
				logger.Log4.Debug("UserId-%d: Gate msgHead :%v", r.mRobotData.MUId, msgHead)
				r.FsmSendEvent(RobotEventRemoteMsg, msgHead)
			}
		}
		r.mWg.Done()

	}()

	go func() {

		for {
			r.mWg.Add(1)
			if r.mNetClient == nil {
				break
			}
			time.Sleep(time.Duration(2) * time.Second)
			if r.mNetClient == nil {
				logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
				break
			}
			cur := time.Now()
			timestamp := cur.UnixNano() / 1000000
			m := &GateSubCmd.UserTimeSendServer{
				GameTime: proto.Int64(timestamp),
			}

			err := r.mNetClient.SenMsg(int16(Main.EnMainCmdID_GATE_MAIN_CMD), int16(GateSubCmd.EnSubCmdID_USER_TIME_TOSERVER_SUB_CMD), m)
			if err != nil {
				logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
				break
			}

		}
		r.mWg.Done()

	}()
	return nil

}

//登陆状态：处理登陆lobbysvr
func (r *Robot) RobotStateLoginEventLoginLobbySvrd(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	err := r.ConnectGateSvr()
	if err != nil {
		//网络连接失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultLogin_Gatesvr_ConnectFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	}
	err = r.RequestLoginGateSvr()
	if err != nil {
		//网络连接失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultLogin_Lobbysvr_SendLoginRequestFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	} else {
		//设置请求定时器
		r.SetTimer(TimerLoginLobbySvr, RequestTimeOut)

	}
	return

}

/*
// LOGIN_GATE_SUB_CMD
message LoginGateInfo
{
	required uint32 userid   = 1;			// 角色id
	required string token   = 2;
	required int32 iDevice   = 3			[default = 1];					//device id: 1 IOS, 2 Android, 3 Windo//ws
	required int32 iPlatform = 4			[default = 1];					//platform id: 1 Apple Store, 2 360, 3 ?
	required string iVersion = 5			[default = "1.0.0.0"];			//具体版本号
	optional uint32  uiSpread = 6;                  //推广号
	optional string packageflag  = 7;                  //标示包名
	optional string devicestring  = 8;                  //设备串号
	optional string userName  = 9;                  //登录用的用户名
	optional string packmark = 10;					//包标识
	optional string ip_address = 11;				//IP地址
}

*/

func (r *Robot) RequestLoginGateSvr() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}

	m := &GateSubCmd.LoginGateInfo{
		Userid:       proto.Uint32(uint32(r.mRobotData.MUserId)),
		Token:        proto.String(r.mRobotData.MToken),
		IDevice:      proto.Int32(int32(r.mRobotData.MUserType)),
		IPlatform:    proto.Int32(1),
		IVersion:     proto.String("1.0.0.0"),
		Packageflag:  proto.String(r.mRobotData.MPackageflag),
		Devicestring: proto.String(r.mRobotData.MDevice),
		UserName:     proto.String(r.mRobotData.MUserName),
		IpAddress:    proto.String(r.mRobotData.MClientIp),
	}

	err := r.mNetClient.SenMsg(int16(Main.EnMainCmdID_GATE_MAIN_CMD), int16(GateSubCmd.EnSubCmdID_LOGIN_GATE_SUB_CMD), m)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
		return errors.New("ERR_NET_SEND_FAIL")
	}
	return nil
}

func (r *Robot) SetTimer(timerType TimerType, timeValue time.Duration) error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	var lRetErr error
	if _, ok := r.mTimers[timerType]; ok {
		lRetErr = errors.New("ERR_EXIST")
		return lRetErr
	}
	t := time.AfterFunc(timeValue, func() {
		var timerData TimertData
		timerData.MTimerType = timerType
		timerData.MTimeValue = timeValue
		r.mFsm.Event(string(RobotEventTimer), &timerData)

	})
	r.mTimers[timerType] = t
	return nil
}
func (r *Robot) ReSetTimer(timerType TimerType, timeValue time.Duration) error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	var lRetErr error
	var t *time.Timer
	var ok bool
	if t, ok = r.mTimers[timerType]; !ok {
		lRetErr = errors.New("ERR_NOT_EXIST")
		return lRetErr
	}
	t.Reset(timeValue)
	return nil
}

func (r *Robot) CancelTimer(timerType TimerType) error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	var lRetErr error
	var t *time.Timer
	var ok bool
	if t, ok = r.mTimers[timerType]; !ok {
		lRetErr = errors.New("ERR_NOT_EXIST")
		return lRetErr
	}
	t.Stop()
	delete(r.mTimers, timerType)
	return nil
}

/*
	//微游戏
	r.mFsm.AddStateEvent(RobotStateClub, RobotEventInit, r.RobotStateClubEventInit)
	r.mFsm.AddStateEvent(RobotStateClub, RobotEventQuit, r.RobotStateClubEventQuit)
	r.mFsm.AddStateEvent(RobotStateClub, RobotEventRemoteMsg, r.RobotStateClubEventRemoteMsg)
	r.mFsm.AddStateEvent(RobotStateClub, RobotEventSocketAbnormal, r.RobotStateClubEventSocketAbnormal)
	r.mFsm.AddStateEvent(RobotStateClub, RobotEventTimer, r.RobotStateClubEventTimer)
	r.mFsm.AddStateEvent(RobotStateClub, RobotEventTaskAnalysis, r.RobotStateClubEventTaskAnalysis)
	r.mFsm.AddStateEvent(RobotStateClub, RobotEventClubEnter, r.RobotStateClubEventClubEnter)
*/

//微游戏状态
func (r *Robot) RobotStateClubEventInit(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	if r.mIsLoginOk == false {
		r.mCurTaskStepReuslt = TaskResultNotLogin
	}
	r.FsmSendEvent(RobotEventTaskAnalysis, nil)
}

func (r *Robot) RobotStateClubEventQuit(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

}

//登陆状态：处理远程消息
func (r *Robot) RobotStateClubEventRemoteMsg(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if len(e.Args) == 0 {
		logger.Log4.Error("UserId-%d: no Remote Msg", r.mRobotData.MUId)
		return
	}
	msgHead := e.Args[0].(*net.MsgHead)
	switch msgHead.MMainCmd {
	case int16(Main.EnMainCmdID_LOGIN_MAIN_CMD):
		r.HandelLoginMainMsg(msgHead)
		break
	case int16(Main.EnMainCmdID_GATE_MAIN_CMD):
		r.HandelGateMainMsg(msgHead)
		break
	case int16(Main.EnMainCmdID_LOBBY_MAIN_CMD):
		r.HandelLobbyMainMsg(msgHead)
		break
	case int16(Main.EnMainCmdID_CLUB_MAIN_CMD):
		r.HandelClubMainMsg(msgHead)
		break
	default:
		logger.Log4.Debug("UserId-%d: not process Cmd:%d, SubCmd:%d", r.mRobotData.MUId, msgHead.MMainCmd, msgHead.MSubCmd)
		break
	}

}

//登陆状态：处理网络异常
func (r *Robot) RobotStateClubEventSocketAbnormal(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mCurTaskStepReuslt == TaskResultNone {
		r.mCurTaskStepReuslt = TaskResultSocketErr
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	}
	if r.mNetClient != nil {
		r.mNetClient.Close()
		r.mNetClient = nil
	}

}

//登陆状态：处理定时事件
func (r *Robot) RobotStateClubEventTimer(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	if len(e.Args) == 0 {
		logger.Log4.Error("UserId-%d: no Remote Msg", r.mRobotData.MUId)
		return
	}
	timerData := e.Args[0].(*TimertData)
	switch timerData.MTimerType {
	case TimerClubCreateClub:
		r.mCurTaskStepReuslt = TaskResultClub_SendRequestCreateClubTimeOut
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		break
	case TimerClubJoinClub:
		r.mCurTaskStepReuslt = TaskResultClub_SendRequestJoinClubTimeOut
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		break
	case TimerClubAddGold:
		r.mCurTaskStepReuslt = TaskResultClub_SendRequestAddGoldTimeOut
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		break

	default:
		break
	}
}

//登陆状态：处理任务解析
func (r *Robot) RobotStateClubEventTaskAnalysis(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	if r.mCurTaskStep != TaskStepNone {
		r.mTaskMng.ReportTaskStepCompleteResult(r.mCurTaskId, r.mCurTaskType, r.mCurTaskStep, r.mCurTaskStepReuslt)
	}
	var taskStep TaskStep = TaskStepNone
	taskStep, _ = r.mTaskMng.DispatchTaskStep()
	if taskStep == TaskStepNone {
		//当前任务步骤已作完，跳到任务管理状态继续分配任务
		r.FsmTransferState(RobotStateTaskMng)
		return
	}
	r.mCurTaskStep = taskStep
	r.mCurTaskStepReuslt = TaskResultNone

	switch r.mCurTaskStep {
	case TaskStepClubEnter:
		r.FsmSendEvent(RobotEventClubEnter, nil)
		break
	case TaskStepClubAddGold:
		r.FsmSendEvent(RobotEventClubAddGold, nil)
		break

	default:
		break
	}
	return
}

//微游戏状态： 进入微游戏事件
func (r *Robot) RobotStateClubEventClubEnter(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	prarmNum := r.mTaskMng.GetTaskPrarmNum()
	if prarmNum != 1 {
		r.mCurTaskStepReuslt = TaskResultParamErr
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Error("<ENTER> :UserId-%d:CurState：%s pram error", r.mRobotData.MUId, e.FSM.CurState())
		return
	}
	if r.mRobotData.MWantClubId == r.mRobotData.MClubData.MClubId {
		//已经在俱乐部中了，改任务直接置为成功
		r.mCurTaskStepReuslt = TaskResultSuccess
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Info("<ENTER> :UserId-%d:CurState：%s 已经在俱乐部-%d中了",
			r.mRobotData.MUId, e.FSM.CurState(), r.mRobotData.MWantClubId)
		return
	}
	if r.mRobotData.MClubData.MClubId != 0 {
		//已经在其他俱乐部中了，不能再加入其他俱乐部
		r.mCurTaskStepReuslt = TaskResultClub_CfgErr
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Error("<ENTER> :UserId-%d:CurState：%s 已经在俱乐部-%d中了，不能在创建其他俱乐部了",
			r.mRobotData.MUId, e.FSM.CurState(), r.mRobotData.MClubData.MClubId)
		return
	}

	prarmValue := r.mTaskMng.GetTaskPrarm(0)
	logger.Log4.Debug("UserId-%d: get prarmValue:%s", r.mRobotData.MUId, prarmValue)
	if prarmValue == "1" {
		//创建微游戏
		err := r.RequestCreateClub()
		if err != nil {
			//网络连接失败，结束当前任务
			r.mCurTaskStepReuslt = TaskResultClub_SendRequestCreateClubFail
			r.FsmSendEvent(RobotEventTaskAnalysis, nil)
			return
		} else {
			//设置请求定时器
			r.SetTimer(TimerClubCreateClub, RequestTimeOut)

		}
	} else {
		//加入微游戏
		err := r.RequestJoinClub()
		if err != nil {
			//网络连接失败，结束当前任务
			r.mCurTaskStepReuslt = TaskResultClub_SendRequestJoinClubFail
			r.FsmSendEvent(RobotEventTaskAnalysis, nil)
			return
		} else {
			//设置请求定时器
			r.SetTimer(TimerClubJoinClub, RequestTimeOut)
		}
	}

	return

}

func (r *Robot) RequestCreateClub() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}
	clubName := "Club" + strconv.Itoa(r.mRobotData.MWantClubId)
	m := &ClubSubCmd.CS_CreateClub{
		StrClubName: proto.String(clubName),
		IUserID:     proto.Int64(int64(r.mRobotData.MUserId)),
		IWantCluId:  proto.Int32(int32(r.mRobotData.MWantClubId)),
	}

	err := r.mNetClient.SenMsg(int16(Main.EnMainCmdID_CLUB_MAIN_CMD), int16(ClubSubCmd.EnClientSubCmdID_CS_CREATE_CLUB), m)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
		return errors.New("ERR_NET_SEND_FAIL")
	}
	return nil
}

func (r *Robot) RequestJoinClub() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}

	m := &ClubSubCmd.CS_JoinClub{
		IUserID: proto.Int64(int64(r.mRobotData.MUserId)),
		IClubID: proto.Int64(int64(r.mRobotData.MWantClubId)),
	}

	err := r.mNetClient.SenMsg(int16(Main.EnMainCmdID_CLUB_MAIN_CMD), int16(ClubSubCmd.EnClientSubCmdID_CS_JOIN_CLUB), m)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
		return errors.New("ERR_NET_SEND_FAIL")
	}
	return nil
}

//微游戏状态： 请求加金币
func (r *Robot) RobotStateClubEventAddGold(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mRobotData.MUserGold >= 100 {
		r.mCurTaskStepReuslt = TaskResultSuccess
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Debug("UserId-%d: gold:%d", r.mRobotData.MUId, r.mRobotData.MUserGold)
		return
	}
	if r.mRobotData.MUserDiamond < 5 {
		r.mCurTaskStepReuslt = TaskResultClub_SendRequestAddDiamondNotEnough
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Debug("UserId-%d: gold:%d, But  can not add ,because:MUserDiamond:%d",
			r.mRobotData.MUId, r.mRobotData.MUserGold, r.mRobotData.MUserDiamond)
		return
	}
	//
	err := r.RequestDiamondToGold()
	if err != nil {
		//网络连接失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultClub_SendRequestAddGoldFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	} else {
		//设置请求定时器
		r.SetTimer(TimerClubAddGold, RequestTimeOut)

	}

	return

}

func (r *Robot) RequestDiamondToGold() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}
	m := &LobbySubCmd.CS_RequestDiamondToGold{
		Id: proto.Int32(2),
	}

	err := r.mNetClient.SenMsg(int16(Main.EnMainCmdID_LOBBY_MAIN_CMD), int16(LobbySubCmd.EnSubCmdID_REQUEST_DIAMOND_TO_GOLD_SUB_CMD), m)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
		return errors.New("ERR_NET_SEND_FAIL")
	}
	return nil
}

/*
	//血战麻将
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventInit, r.RobotStateXzmjEventInit)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventQuit, r.RobotStateXzmjEventQuit)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventRemoteMsg, r.RobotStateXzmjEventRemoteMsg)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventSocketAbnormal, r.RobotStateXzmjEventSocketAbnormal)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventTimer, r.RobotStateXzmjEventTimer)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventTaskAnalysis, r.RobotStateXzmjEventTaskAnalysis)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventXzmjCreateRoom, r.RobotStateXzmjEventCreateRoom)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventXzmjEnterRoom, r.RobotStateXzmjEventEnterRoom)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventXzmjStartGame, r.RobotStateXzmjEventStartRoom)
	r.mFsm.AddStateEvent(RobotStateXzmj, RobotEventXzmjOperate, r.RobotStateXzmjEventOperate)
*/

//血战麻将状态
func (r *Robot) RobotStateXzmjEventInit(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	if r.mIsLoginOk == false {
		r.mCurTaskStepReuslt = TaskResultNotLogin
	}
	r.FsmSendEvent(RobotEventTaskAnalysis, nil)
}

func (r *Robot) RobotStateXzmjEventQuit(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	r.mIsEnterRoom = false

}

//血战麻将状态：处理远程消息
func (r *Robot) RobotStateXzmjEventRemoteMsg(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if len(e.Args) == 0 {
		logger.Log4.Error("UserId-%d: no Remote Msg", r.mRobotData.MUId)
		return
	}
	msgHead := e.Args[0].(*net.MsgHead)
	switch msgHead.MMainCmd {
	case int16(Main.EnMainCmdID_LOGIN_MAIN_CMD):
		r.HandelLoginMainMsg(msgHead)
		break
	case int16(Main.EnMainCmdID_GATE_MAIN_CMD):
		r.HandelGateMainMsg(msgHead)
		break
	case int16(Main.EnMainCmdID_LOBBY_MAIN_CMD):
		r.HandelLobbyMainMsg(msgHead)
		break
	case int16(Main.EnMainCmdID_CLUB_MAIN_CMD):
		r.HandelClubMainMsg(msgHead)
		break
	case int16(Main.EnMainCmdID_GAME_MAIN_USER):
		r.HandelGameMainUserMsg(msgHead)
		break
	case int16(Main.EnMainCmdID_GAME_MAIN_GAME):
		r.mXzmjTable.HandelGameMainMsg(msgHead)
		break
	default:
		logger.Log4.Debug("UserId-%d: not process Cmd:%d, SubCmd:%d", r.mRobotData.MUId, msgHead.MMainCmd, msgHead.MSubCmd)
		break
	}

}

//血战麻将状态：处理网络异常
func (r *Robot) RobotStateXzmjEventSocketAbnormal(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mCurTaskStepReuslt == TaskResultNone {
		r.mCurTaskStepReuslt = TaskResultSocketErr
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	}
	if r.mNetClient != nil {
		r.mNetClient.Close()
		r.mNetClient = nil
	}

}

//血战麻将状态：处理定时事件
func (r *Robot) RobotStateXzmjEventTimer(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	if len(e.Args) == 0 {
		logger.Log4.Error("UserId-%d: no Remote Msg", r.mRobotData.MUId)
		return
	}
	timerData := e.Args[0].(*TimertData)
	switch timerData.MTimerType {
	case TimerClubCreateRoom:
		r.mCurTaskStepReuslt = TaskResultClub_SendRequestCreateRoomTimeOut
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		break
	case TimerClubEnterRoom:
		r.mCurTaskStepReuslt = TaskResultClub_SendRequestEnterRoomTimeOut
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		break

	default:
		break
	}
}

//血战麻将状态：处理任务解析
func (r *Robot) RobotStateXzmjEventTaskAnalysis(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	if r.mCurTaskStep != TaskStepNone {
		r.mTaskMng.ReportTaskStepCompleteResult(r.mCurTaskId, r.mCurTaskType, r.mCurTaskStep, r.mCurTaskStepReuslt)
	}
	var taskStep TaskStep = TaskStepNone
	taskStep, _ = r.mTaskMng.DispatchTaskStep()
	if taskStep == TaskStepNone {
		//当前任务步骤已作完，跳到任务管理状态继续分配任务
		r.FsmTransferState(RobotStateTaskMng)
		return
	}
	r.mCurTaskStep = taskStep
	r.mCurTaskStepReuslt = TaskResultNone

	switch r.mCurTaskStep {
	case TaskStepXzmjCreateRoom:
		r.FsmSendEvent(RobotEventXzmjCreateRoom, nil)
		break
	case TaskStepXzmjEnterRoom:
		r.FsmSendEvent(RobotEventXzmjEnterRoom, nil)
		break
	case TaskStepXzmjStartGame:
		r.FsmSendEvent(RobotEventXzmjStartGame, nil)
		break
	default:
		break
	}
	return
}

//血战麻将状态： 创建房间
func (r *Robot) RobotStateXzmjEventCreateRoom(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	prarmNum := r.mTaskMng.GetTaskPrarmNum()
	if prarmNum != 2 {
		r.mCurTaskStepReuslt = TaskResultParamErr
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Error("<ENTER> :UserId-%d:CurState：%s pram error", r.mRobotData.MUId, e.FSM.CurState())

		return
	}
	var isNeedCreateRoom bool = false
	prarmValue := r.mTaskMng.GetTaskPrarm(0)
	if prarmValue == "1" {
		isNeedCreateRoom = true
	}

	if isNeedCreateRoom == false {
		//不需要创建房间
		r.mCurTaskStepReuslt = TaskResultSuccess
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Info("<ENTER> :UserId-%d:CurState：%s 不需要创建房间",
			r.mRobotData.MUId, e.FSM.CurState())
		return
	}

	//创建微游戏
	err := r.RequestCreateXzmjRoom()
	if err != nil {
		//网络连接失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultClub_SendRequestCreateRoomFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	} else {
		//设置请求定时器
		r.SetTimer(TimerClubCreateRoom, RequestTimeOut)

	}

	return

}

/*
//CS_CLUB_CREATE_TABLE		 =302;	 //创建桌子
message CS_ClubCreateTable
{
	required int64             ClubID         = 1;         //俱乐部ID
	required int64 			   RoomId		  = 2;		   //房间id

	required int32 			   TableType 	  =  3;			//0：成员房, 1 好友房
	required stTableAttribute	TableAttribute = 4; //桌子高级参数
};
message stTableAttribute
{
	required int32 			   GameCurrencyType = 1;		//游戏货币类型（1:钻石，2：金币，3积分）
	required int32 			   PayRoomRateType 	= 2;			//房费支付方式 （0：房主支付，1：AA支付）
	required int32 			   PlanGameCount	=3; 		//开房局数
	required int32			   DiZhu			=4;		//底注
	required int64			   EnterScore       =5;		//进入分数限制
	required int64			   LeaveScore       =6;		//离开分数限制
	required int64			   BeiShuLimit      =7;		//最大倍数限制
	required int32			   ChairCount       =8;		//房间椅子数量
	required int32			   IsAllowEnterAfterStart   =9;		//是否允许开始后加入房间
	required int64			   TableType		  = 10;			//桌子类型（0.成员房 1.好友房）
	required int64			   RoomRate       =11;		//房费
	required int64			   ServerRate       =12;		//服务费
	required string			   TableAdvanceParam =13; 			//桌子高级参数
	required string			   TableName =14; 			//桌子名字
	required int32			   IsIpWarn =15; 			//是否相同ip警告
	required int32			   IsGpsWarn =16; 			//是否相同地点警告
	optional int32			   TableId =17; 			//压力测试用，指定想要创建的桌子id

};
*/

type XzmjTableAdvanceParam struct {
	StartGameMinPlayerCount int32
	ZiMoJiaFan              int32
	DianGangHuaZiMo         int32
	HuanSanZhangType        int32
	JinGouDiao              int32
	HaiDiLaoYue             int32
	DaXiaoYu                int32
	YaoJiu                  int32
	Jiang                   int32
}

func (r *Robot) RequestCreateXzmjRoom() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}

	var advanceParam XzmjTableAdvanceParam
	advanceParam.StartGameMinPlayerCount = 2
	advanceParam.ZiMoJiaFan = 1
	advanceParam.DianGangHuaZiMo = 1
	advanceParam.HuanSanZhangType = 1
	advanceParam.JinGouDiao = 1
	advanceParam.HaiDiLaoYue = 1
	advanceParam.DaXiaoYu = 1
	advanceParam.YaoJiu = 1
	advanceParam.Jiang = 1
	var advanceParamData []byte
	var err error
	if advanceParamData, err = json.Marshal(advanceParam); err != nil {
		logger.Log4.Error("UserId-%d: XzmjTableAdvanceParam Marshal err:%s", r.mRobotData.MUId, err)
		return errors.New("ERR_PARAM_MARSHAL")
	}
	logger.Log4.Debug("UserId-%d: XzmjTableAdvanceParam %s", r.mRobotData.MUId, advanceParamData)

	tableName := "TableName" + strconv.Itoa(r.mRobotData.MXzmjTableId)
	m := &ClubSubCmd.CS_ClubCreateTable{
		ClubID:    proto.Int64(int64(r.mRobotData.MClubData.MClubId)),
		RoomId:    proto.Int64(int64(r.mRobotData.MXzmjRoomId)),
		TableType: proto.Int32(int32(1)),
		TableAttribute: &ClubSubCmd.StTableAttribute{
			GameCurrencyType:       proto.Int32(2),
			PayRoomRateType:        proto.Int32(3),
			PlanGameCount:          proto.Int32(0),
			DiZhu:                  proto.Int32(1),
			EnterScore:             proto.Int64(10),
			LeaveScore:             proto.Int64(2),
			BeiShuLimit:            proto.Int64(32),
			ChairCount:             proto.Int32(4),
			IsAllowEnterAfterStart: proto.Int32(1),
			TableType:              proto.Int64(0),
			RoomRate:               proto.Int64(0),
			ServerRate:             proto.Int64(100),
			IsGpsWarn:              proto.Int32(1),
			IsIpWarn:               proto.Int32(1),
			TableName:              proto.String(tableName),
			TableAdvanceParam:      proto.String(string(advanceParamData)),
			TableId:                proto.Int32(int32(r.mRobotData.MXzmjTableId)),
		},
	}

	err = r.mNetClient.SenMsg(int16(Main.EnMainCmdID_CLUB_MAIN_CMD), int16(ClubSubCmd.EnClientSubCmdID_CS_CLUB_CREATE_TABLE), m)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
		return errors.New("ERR_NET_SEND_FAIL")
	}
	return nil
}

//血战状态： 进入房间
func (r *Robot) RobotStateXzmjEventEnterRoom(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	prarmNum := r.mTaskMng.GetTaskPrarmNum()
	if prarmNum != 2 {
		r.mCurTaskStepReuslt = TaskResultParamErr
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Error("<ENTER> :UserId-%d:CurState：%s pram error", r.mRobotData.MUId, e.FSM.CurState())

		return
	}
	var isNeedEnterRoom bool = false
	prarmValue := r.mTaskMng.GetTaskPrarm(1)
	if prarmValue == "1" {
		isNeedEnterRoom = true
	}

	if isNeedEnterRoom == false {
		//不需要进入
		r.mCurTaskStepReuslt = TaskResultSuccess
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Error("<ENTER> :UserId-%d:CurState：%s 不需要进入房间",
			r.mRobotData.MUId, e.FSM.CurState())
	}

	//创建微游戏
	err := r.RequestEnterXzmjRoom()
	if err != nil {
		//网络连接失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultClub_SendRequestEnterRoomFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	} else {
		//设置请求定时器
		r.SetTimer(TimerClubEnterRoom, RequestTimeOut)

	}

}

/*
//CS_CLUB_ENTER_ROOM		 =304;	 //进入房间
message CS_ClubEnterRoom
{
	required int64             ClubID         = 1;         //俱乐部ID
	required int64 			   RoomId		  = 2;		   //房间id
	required int64 			   TableId		  = 3;		   //Tableid
	required int64 			   UserLng		  = 4;		   //经度
	required int64 			   UserLat		  = 5;		   //纬度
};

*/
func (r *Robot) RequestEnterXzmjRoom() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}

	m := &ClubSubCmd.CS_ClubEnterRoom{
		ClubID:  proto.Int64(int64(r.mRobotData.MClubData.MClubId)),
		RoomId:  proto.Int64(int64(r.mRobotData.MXzmjRoomId)),
		TableId: proto.Int64(int64(r.mRobotData.MXzmjTableId)),
		UserLng: proto.Int64(int64(0)),
		UserLat: proto.Int64(int64(0)),
	}

	err := r.mNetClient.SenMsg(int16(Main.EnMainCmdID_CLUB_MAIN_CMD), int16(ClubSubCmd.EnClientSubCmdID_CS_CLUB_ENTER_ROOM), m)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
		return errors.New("ERR_NET_SEND_FAIL")
	}
	return nil
}

//血战状态： 开始游戏
func (r *Robot) RobotStateXzmjEventStartRoom(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	//r.mCurTaskStepReuslt = TaskResultSuccess
	//r.FsmSendEvent(RobotEventTaskAnalysis, nil)

	return

}

//血战状态： 游戏操作
func (r *Robot) RobotStateXzmjEventOperate(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	return

}

func (r *Robot) ReportXzmjGameEnd() {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mCurTaskType == TaskTypeXzmj && r.mCurTaskStep == TaskStepXzmjStartGame {
		r.mRobotData.MXzmjGameCount += 1
		if r.mRobotData.MXzmjGameCount >= 400 {
			r.RequestXzmjLeaveRoom()
		}

	}

}

/*
	//斗地主
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventInit, r.RobotStateDdzEventInit)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventQuit, r.RobotStateDdzEventQuit)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventRemoteMsg, r.RobotStateDdzEventRemoteMsg)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventSocketAbnormal, r.RobotStateDdzEventSocketAbnormal)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventTimer, r.RobotStateDdzEventTimer)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventTaskAnalysis, r.RobotStateDdzEventTaskAnalysis)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventDdzCreateRoom, r.RobotStateDdzEventCreateRoom)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventDdzEnterRoom, r.RobotStateDdzEventEnterRoom)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventDdzStartGame, r.RobotStateDdzEventStartRoom)
	r.mFsm.AddStateEvent(RobotStateDdz, RobotEventDdzOperate, r.RobotStateDdzEventOperate)

*/

//斗地主状态
func (r *Robot) RobotStateDdzEventInit(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	if r.mIsLoginOk == false {
		r.mCurTaskStepReuslt = TaskResultNotLogin
	}
	r.FsmSendEvent(RobotEventTaskAnalysis, nil)
}

func (r *Robot) RobotStateDdzEventQuit(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	r.mIsEnterRoom = false

}

//斗地主状态：处理远程消息
func (r *Robot) RobotStateDdzEventRemoteMsg(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if len(e.Args) == 0 {
		logger.Log4.Error("UserId-%d: no Remote Msg", r.mRobotData.MUId)
		return
	}
	msgHead := e.Args[0].(*net.MsgHead)
	switch msgHead.MMainCmd {
	case int16(Main.EnMainCmdID_LOGIN_MAIN_CMD):
		r.HandelLoginMainMsg(msgHead)
		break
	case int16(Main.EnMainCmdID_GATE_MAIN_CMD):
		r.HandelGateMainMsg(msgHead)
		break
	case int16(Main.EnMainCmdID_LOBBY_MAIN_CMD):
		r.HandelLobbyMainMsg(msgHead)
		break
	case int16(Main.EnMainCmdID_CLUB_MAIN_CMD):
		r.HandelClubMainMsg(msgHead)
		break
	case int16(Main.EnMainCmdID_GAME_MAIN_USER):
		r.HandelGameMainUserMsg(msgHead)
		break
	case int16(Main.EnMainCmdID_GAME_MAIN_GAME):
		r.mDdzTable.HandelGameMainMsg(msgHead)
		break
	default:
		logger.Log4.Debug("UserId-%d: not process Cmd:%d, SubCmd:%d", r.mRobotData.MUId, msgHead.MMainCmd, msgHead.MSubCmd)
		break
	}

}

//斗地主状态：处理网络异常
func (r *Robot) RobotStateDdzEventSocketAbnormal(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mCurTaskStepReuslt == TaskResultNone {
		r.mCurTaskStepReuslt = TaskResultSocketErr
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	}
	if r.mNetClient != nil {
		r.mNetClient.Close()
		r.mNetClient = nil
	}

}

//斗地主状态：处理定时事件
func (r *Robot) RobotStateDdzEventTimer(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	if len(e.Args) == 0 {
		logger.Log4.Error("UserId-%d: no Remote Msg", r.mRobotData.MUId)
		return
	}
	timerData := e.Args[0].(*TimertData)
	switch timerData.MTimerType {
	case TimerClubCreateRoom:
		r.mCurTaskStepReuslt = TaskResultClub_SendRequestCreateRoomTimeOut
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		break
	case TimerClubEnterRoom:
		r.mCurTaskStepReuslt = TaskResultClub_SendRequestEnterRoomTimeOut
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		break

	default:
		break
	}
}

//斗地主状态：处理任务解析
func (r *Robot) RobotStateDdzEventTaskAnalysis(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	if r.mCurTaskStep != TaskStepNone {
		r.mTaskMng.ReportTaskStepCompleteResult(r.mCurTaskId, r.mCurTaskType, r.mCurTaskStep, r.mCurTaskStepReuslt)
	}
	var taskStep TaskStep = TaskStepNone
	taskStep, _ = r.mTaskMng.DispatchTaskStep()
	if taskStep == TaskStepNone {
		//当前任务步骤已作完，跳到任务管理状态继续分配任务
		r.FsmTransferState(RobotStateTaskMng)
		return
	}
	r.mCurTaskStep = taskStep
	r.mCurTaskStepReuslt = TaskResultNone

	switch r.mCurTaskStep {
	case TaskStepDdzCreateRoom:
		r.FsmSendEvent(RobotEventDdzCreateRoom, nil)
		break
	case TaskStepDdzEnterRoom:
		r.FsmSendEvent(RobotEventDdzEnterRoom, nil)
		break
	case TaskStepDdzStartGame:
		r.FsmSendEvent(RobotEventDdzStartGame, nil)
		break
	default:
		break
	}
	return
}

//斗地主状态： 创建房间 --修改为进入金币场
func (r *Robot) RobotStateDdzEventCreateRoom(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	prarmNum := r.mTaskMng.GetTaskPrarmNum()
	if prarmNum != 2 {
		r.mCurTaskStepReuslt = TaskResultParamErr
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Error("<ENTER> :UserId-%d:CurState：%s pram error", r.mRobotData.MUId, e.FSM.CurState())

		return
	}
	var isNeedCreateRoom bool = false
	prarmValue := r.mTaskMng.GetTaskPrarm(0)
	if prarmValue == "1" {
		isNeedCreateRoom = true
	}

	if isNeedCreateRoom == false {
		//不需要创建房间
		r.mCurTaskStepReuslt = TaskResultSuccess
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Error("<ENTER> :UserId-%d:CurState：%s 不需要创建房间",
			r.mRobotData.MUId, e.FSM.CurState())
		return
	}

	//创建微游戏
	err := r.RequestCreateDdzRoom()
	if err != nil {
		//网络连接失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultClub_SendRequestCreateRoomFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	} else {
		//设置请求定时器
		r.SetTimer(TimerClubCreateRoom, RequestTimeOut)

	}

	return

}

/*
//CS_CLUB_CREATE_TABLE		 =302;	 //创建桌子
message CS_ClubCreateTable
{
	required int64             ClubID         = 1;         //俱乐部ID
	required int64 			   RoomId		  = 2;		   //房间id

	required int32 			   TableType 	  =  3;			//0：成员房, 1 好友房
	required stTableAttribute	TableAttribute = 4; //桌子高级参数
};
message stTableAttribute
{
	required int32 			   GameCurrencyType = 1;		//游戏货币类型（1:钻石，2：金币，3积分）
	required int32 			   PayRoomRateType 	= 2;			//房费支付方式 （0：房主支付，1：AA支付）
	required int32 			   PlanGameCount	=3; 		//开房局数
	required int32			   DiZhu			=4;		//底注
	required int64			   EnterScore       =5;		//进入分数限制
	required int64			   LeaveScore       =6;		//离开分数限制
	required int64			   BeiShuLimit      =7;		//最大倍数限制
	required int32			   ChairCount       =8;		//房间椅子数量
	required int32			   IsAllowEnterAfterStart   =9;		//是否允许开始后加入房间
	required int64			   TableType		  = 10;			//桌子类型（0.成员房 1.好友房）
	required int64			   RoomRate       =11;		//房费
	required int64			   ServerRate       =12;		//服务费
	required string			   TableAdvanceParam =13; 			//桌子高级参数
	required string			   TableName =14; 			//桌子名字
	required int32			   IsIpWarn =15; 			//是否相同ip警告
	required int32			   IsGpsWarn =16; 			//是否相同地点警告
	optional int32			   TableId =17; 			//压力测试用，指定想要创建的桌子id

};
*/

type DdzTableAdvanceParam struct {
	GameType         uint32
	ConfirmLandowner uint32 `json:"confirmLandowner"`
	Needcardholder   bool   `json:"needcardholder"`
	Shuffletype      uint32 `json:"shuffletype"`
}

func (r *Robot) RequestCreateDdzRoom() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}

	var advanceParam DdzTableAdvanceParam
	advanceParam.GameType = 0
	advanceParam.ConfirmLandowner = 0
	advanceParam.Needcardholder = false
	advanceParam.Shuffletype = 1

	var advanceParamData []byte
	var err error
	if advanceParamData, err = json.Marshal(advanceParam); err != nil {
		logger.Log4.Error("UserId-%d: DdzTableAdvanceParam Marshal err:%s", r.mRobotData.MUId, err)
		return errors.New("ERR_PARAM_MARSHAL")
	}
	logger.Log4.Debug("UserId-%d: DdzTableAdvanceParam %s", r.mRobotData.MUId, advanceParamData)

	tableName := "TableName" + strconv.Itoa(r.mRobotData.MXzmjTableId)
	m := &ClubSubCmd.CS_ClubCreateTable{
		ClubID:    proto.Int64(int64(r.mRobotData.MClubData.MClubId)),
		RoomId:    proto.Int64(int64(r.mRobotData.MDdzRoomId)),
		TableType: proto.Int32(int32(1)),
		TableAttribute: &ClubSubCmd.StTableAttribute{
			GameCurrencyType:       proto.Int32(2),
			PayRoomRateType:        proto.Int32(3),
			PlanGameCount:          proto.Int32(0),
			DiZhu:                  proto.Int32(1),
			EnterScore:             proto.Int64(10),
			LeaveScore:             proto.Int64(2),
			BeiShuLimit:            proto.Int64(32),
			ChairCount:             proto.Int32(3),
			IsAllowEnterAfterStart: proto.Int32(0),
			TableType:              proto.Int64(0),
			RoomRate:               proto.Int64(0),
			ServerRate:             proto.Int64(100),
			IsGpsWarn:              proto.Int32(1),
			IsIpWarn:               proto.Int32(1),
			TableName:              proto.String(tableName),
			TableAdvanceParam:      proto.String(string(advanceParamData)),
			TableId:                proto.Int32(int32(r.mRobotData.MDdzTableId)),
		},
	}

	err = r.mNetClient.SenMsg(int16(Main.EnMainCmdID_CLUB_MAIN_CMD), int16(ClubSubCmd.EnClientSubCmdID_CS_CLUB_CREATE_TABLE), m)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
		return errors.New("ERR_NET_SEND_FAIL")
	}
	return nil
}

//斗地主状态： 进入房间
func (r *Robot) RobotStateDdzEventEnterRoom(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	prarmNum := r.mTaskMng.GetTaskPrarmNum()
	if prarmNum != 2 {
		r.mCurTaskStepReuslt = TaskResultParamErr
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Error("<ENTER> :UserId-%d:CurState：%s pram error", r.mRobotData.MUId, e.FSM.CurState())

		return
	}
	var isNeedEnterRoom bool = false
	prarmValue := r.mTaskMng.GetTaskPrarm(1)
	if prarmValue == "1" {
		isNeedEnterRoom = true
	}

	if isNeedEnterRoom == false {
		//不需要进入
		r.mCurTaskStepReuslt = TaskResultSuccess
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Error("<ENTER> :UserId-%d:CurState：%s 不需要进入房间",
			r.mRobotData.MUId, e.FSM.CurState())
	}

	//创建微游戏
	err := r.RequestEnterDdzRoom()
	if err != nil {
		//网络连接失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultClub_SendRequestEnterRoomFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	} else {
		//设置请求定时器
		r.SetTimer(TimerClubEnterRoom, RequestTimeOut)

	}

}

/*
//CS_CLUB_ENTER_ROOM		 =304;	 //进入房间
message CS_ClubEnterRoom
{
	required int64             ClubID         = 1;         //俱乐部ID
	required int64 			   RoomId		  = 2;		   //房间id
	required int64 			   TableId		  = 3;		   //Tableid
	required int64 			   UserLng		  = 4;		   //经度
	required int64 			   UserLat		  = 5;		   //纬度
};

*/
func (r *Robot) RequestEnterDdzRoom() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}

	m := &ClubSubCmd.CS_ClubEnterRoom{
		ClubID:  proto.Int64(int64(r.mRobotData.MClubData.MClubId)),
		RoomId:  proto.Int64(int64(r.mRobotData.MDdzRoomId)),
		TableId: proto.Int64(int64(r.mRobotData.MDdzTableId)),
		UserLng: proto.Int64(int64(0)),
		UserLat: proto.Int64(int64(0)),
	}

	err := r.mNetClient.SenMsg(int16(Main.EnMainCmdID_CLUB_MAIN_CMD), int16(ClubSubCmd.EnClientSubCmdID_CS_CLUB_ENTER_ROOM), m)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
		return errors.New("ERR_NET_SEND_FAIL")
	}
	return nil
}

//斗地主状态： 开始游戏
func (r *Robot) RobotStateDdzEventStartRoom(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	//r.mCurTaskStepReuslt = TaskResultSuccess
	//r.FsmSendEvent(RobotEventTaskAnalysis, nil)

	return

}

//斗地主状态： 游戏操作
func (r *Robot) RobotStateDdzEventOperate(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	return

}
func (r *Robot) ReportDdzGameEnd() {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mCurTaskType == TaskTypeDdz && r.mCurTaskStep == TaskStepDdzStartGame {
		r.mRobotData.MDdzGameCount += 1
		if r.mRobotData.MDdzGameCount >= 400 {
			r.RequestDdzLeaveRoom()
		}

	}

}
