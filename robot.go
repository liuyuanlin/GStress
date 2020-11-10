package main

import (
	"GStress/fsm"
	"GStress/logger"
	"GStress/msg/ClientCommon"

	"GStress/net"
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
	TimerRegisterGame
)

type FsmState string

const (
	RobotStateNone    = "RobotStateNone"
	RobotStateInit    = "RobotStateInit"
	RobotStateTaskMng = "RobotStateTaskMng"
	RobotStateLogin   = "RobotStateLogin"
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
	RobotEventLoginLoginSvrd = "RobotEventLoginLoginSvrd"
	RobotEventRegisterGame   = "RobotEventRegisterGame"
)

type SystemConfig struct {
	MLoginSvrAddr string
	MLoginSvrPort int
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
	mWg                sync.WaitGroup
}

func (r *Robot) Init(taskMap TaskMap, systemCfg ExcelCfg, startState string) error {
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
	logger.Log4.Debug("UserName-%s: mSystemCfg:%v", r.mRobotData.MUserName, r.mSystemCfg)

	//初始化相关数据
	r.mCurTaskId = 0
	r.mCurTaskType = TaskTypeLogin
	r.mCurTaskStep = TaskStepNone
	r.mCurTaskStepReuslt = TaskResultNone
	r.mIsWorkEnd = false
	r.mIsLoginOk = false

	//初始化事件队列
	r.mEventQueue = sq.NewQueue(512)

	//初始任务管理器
	lRetErr = r.mTaskMng.Init(taskMap, r.mRobotData)
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
	r.mFsm.AddStateEvent(RobotStateLogin, RobotEventLoginLoginSvrd, r.RobotStateLoginEventLoginLoginSvrd)
	r.mFsm.AddStateEvent(RobotStateLogin, RobotEventRegisterGame, r.RobotStateLoginEventRegisterGame)

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
	msg := e.Args[0].(*ClientCommon.PushData)
	switch msg.CmdId {
	case int64(ClientCommon.Cmd_LOGIN):
		r.HandelReturnLoginInfo(msg)
		break
	case int64(ClientCommon.Cmd_REGISTER):
		r.HandelRspRegisterGame(msg)
		break
	default:
		break
	}

}

func (r *Robot) HandelReturnLoginInfo(msg *ClientCommon.PushData) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msg == nil {
		return
	}
	//收到响应取消定时器
	r.CancelTimer(TimerLoginLoginSvr)

	loginReturnInfo := &ClientCommon.LoginRsp{}
	err := proto.Unmarshal(msg.Data, loginReturnInfo)
	if err != nil {
		logger.Log4.Debug("unmarshal LoginReturnInfo error: %s", err)
	}
	logger.Log4.Debug("msg: %+v", msg)
	logger.Log4.Debug("loginReturnInfo: %+v", loginReturnInfo)
	if msg.Data == nil {
		r.mCurTaskStepReuslt = TaskResultLogin_Loginsvr_LoginResponseFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		r.mIsLoginOk = false
		return
	}

	logger.Log4.Debug("loginReturnInfo: %+v", loginReturnInfo)
	r.mIsLoginOk = true
	r.mRobotData.UserIdentifier = loginReturnInfo.UserIdentifier

	r.mCurTaskStepReuslt = TaskResultSuccess
	r.FsmSendEvent(RobotEventTaskAnalysis, nil)
}

func (r *Robot) HandelRspRegisterGame(msg *ClientCommon.PushData) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msg == nil {
		return
	}
	//收到响应取消定时器
	r.CancelTimer(TimerRegisterGame)

	RegisterRspInfo := &ClientCommon.RegisterRsp{}
	err := proto.Unmarshal(msg.Data, RegisterRspInfo)
	if err != nil {
		logger.Log4.Debug("unmarshal RegisterRsp error: %s", err)
	}
	logger.Log4.Debug("msg: %+v", msg)
	if msg.Data == nil {
		r.mCurTaskStepReuslt = TaskResultLogin_Loginsvr_RegisterGameResponseFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	}

	logger.Log4.Debug("loginReturnInfo: %+v", RegisterRspInfo)

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
	/*
		//loginErrorCode := &LoginSubCmd.LoginErrorCode{}
		err := proto.Unmarshal(msgHead.MData, loginErrorCode)
		if err != nil {
			logger.Log4.Debug("unmarshal LoginErrorCode error: %s", err)
		}
		logger.Log4.Debug("loginErrorCode: %+v", loginErrorCode)
		r.mCurTaskStepReuslt = TaskResultLogin_Loginsvr_LoginResponseFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	*/
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
		r.mCurTaskStepReuslt = TaskResultLogin_Loginsvr_SendRegisterGameRequestTimeOut
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		break
		/*
			case TimerLoginLobbySvr:
				r.mCurTaskStepReuslt = TaskResultLogin_Lobbysvr_SendLoginRequestTimeOut
				r.FsmSendEvent(RobotEventTaskAnalysis, nil)
				break
			case TimerLoginAddGold:
				r.mCurTaskStepReuslt = TaskResultLogin_SendRequestAddGoldTimeOut
				r.FsmSendEvent(RobotEventTaskAnalysis, nil)
				break
		*/
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
	case TaskStepLoginLoginSvr:
		r.FsmSendEvent(RobotEventLoginLoginSvrd, nil)
		break
	case TaskStepRegisterGame:
		r.FsmSendEvent(RobotEventRegisterGame, nil)
		break
	default:
		break
	}
	return
}

//登陆状态： 向游戏服务注册
func (r *Robot) RobotStateLoginEventRegisterGame(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	if r.mIsLoginOk != true {
		//网络连接失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultNotLogin
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	}
	err := r.RequestRegisterGame()
	if err != nil {
		//请求失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultLogin_Loginsvr_SendRegisterGameRequestFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	} else {
		//设置请求定时器
		r.SetTimer(TimerRegisterGame, RequestTimeOut)

	}
	return
}

//请求向游戏服务器注册
func (r *Robot) RequestRegisterGame() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}

	/*
			//注删长连接 --- 哪个用户 --- 哪个游戏
		message RegisterReq {
		    string UserIdentifier = 1;
		    int64 ActivityId = 2;
		    int32 GameId = 3;
		    SocketType SocketType = 4;
		    int32 PreviewFlag = 5;
		    string RandomCode = 6;
		}
	*/
	m := &ClientCommon.RegisterReq{
		UserIdentifier: r.mRobotData.UserIdentifier,
		ActivityId:     r.mRobotData.ActivityId,
		GameId:         r.mRobotData.GameId,
		SocketType:     ClientCommon.SocketType_PHONE,
		PreviewFlag:    0,
		RandomCode:     "",
	}

	data, err := proto.Marshal(m)
	if err != nil {
		logger.Log4.Error("marshaling error: ", err)
		return err
	}

	send := &ClientCommon.SendData{
		//GameId: r.mRobotData.GameId,
		GameId: 0,
		CmdId:  int64(ClientCommon.Cmd_REGISTER),
		Data:   data,
	}
	err = r.mNetClient.SenMsg(send)
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

	m := &ClientCommon.LoginReq{
		Code: r.mRobotData.MUserName,
	}

	data, err := proto.Marshal(m)
	if err != nil {
		logger.Log4.Error("marshaling error: ", err)
		return err
	}

	send := &ClientCommon.SendData{
		GameId: 0,
		CmdId:  int64(ClientCommon.Cmd_LOGIN),
		Data:   data,
	}
	err = r.mNetClient.SenMsg(send)
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
