package main

import (
	"GStress/fsm"
	"GStress/logger"
	"GStress/msg"
	"GStress/net"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"

	sq "github.com/yireyun/go-queue"
)

const (
	RequestTimeOut = time.Second * 3
)

type TimerType int

const (
	TimerLoginLoginSvr = iota
	TimerLoginLobbySvr
)

type FsmState string

const (
	RobotStateNone    = "RobotStateNone"
	RobotStateInit    = "RobotStateInit"
	RobotStateTaskMng = "RobotStateTaskMng"
	RobotStateLogin   = "RobotStateLogin"
	RobotStateClub    = "RobotStateClub"
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
	RobotEventLoginLobbySvrd = "RobotEventLoginLobbySvrd"
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
	r.mRobotData.Init(robotAttr)

	//初始化事件队列
	r.mEventQueue = sq.NewQueue(1024 * 1024)

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
			//处理资源,如：关闭socket
			if r.mNetClient != nil {
				r.mNetClient.Close()

			}
			break
		}
	}

	return
}
func (r *Robot) DispatchEvent() {
	lEventCount := 0
	for {
		event, ok, quantity := r.mEventQueue.Get()
		if !ok {
			logger.Log4.Info("UserId-%d:Get event Fail,the mEventQueue Size is %d", r.mRobotData.MUId, quantity)
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
		time.Sleep(1000 * time.Millisecond)
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
	r.mFsm.AddStateEvent(RobotStateLogin, RobotEventLoginLoginSvrd, r.RobotStateLoginEventLoginLoginSvrd)
	r.mFsm.AddStateEvent(RobotStateLogin, RobotEventLoginLobbySvrd, r.RobotStateLoginEventLoginLobbySvrd)

	//......
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
	case int16(msg.EnMainCmdID_LOGIN_MAIN_CMD):
		r.HandelLoginMainMsg(msgHead)
		break
	default:
		break
	}

}

func (r *Robot) HandelLoginMainMsg(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}
	switch msgHead.MSubCmd {
	case int16(msg.EnSubCmdID_RETURN_LOGININFO_SUB_CMD):
		r.HandelReturnLoginInfo(msgHead)
		break
	case int16(msg.EnSubCmdID_LOGIN_ERROR_CODE_SUB_CMD):
		r.HandelLoginErrorCode(msgHead)
		break
	default:
		break
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

	loginReturnInfo := &msg.LoginReturnInfo{}
	err := proto.Unmarshal(msgHead.MData, loginReturnInfo)
	if err != nil {
		logger.Log4.Debug("unmarshal LoginReturnInfo error: %s", err)
	}
	logger.Log4.Debug("loginReturnInfo: %+v", loginReturnInfo)

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

	loginErrorCode := &msg.LoginErrorCode{}
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

	if r.mCurTaskStepReuslt != TaskResultNone {
		r.mCurTaskStepReuslt = TaskResultSocketErr
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
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
	case TaskStepLoginSvr:
		r.FsmSendEvent(RobotEventLoginLoginSvrd, nil)
		break
	case TaskStepLobbySvr:
		r.FsmSendEvent(RobotEventLoginLobbySvrd, nil)
		break
	default:
		break
	}
	return
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

	m := &msg.LoginRequestInfo{
		Account:      proto.String(r.mRobotData.MUserName),
		Passwd:       proto.String(secret),
		DeviceString: proto.String("12345612133333333422222222222343434234322"),
		Packageflag:  proto.String("QPB_WEB_1"),
	}

	err := r.mNetClient.SenMsg(int16(msg.EnMainCmdID_LOGIN_MAIN_CMD), int16(msg.EnSubCmdID_REQUEST_LOGIN_SUB_CMD), m)
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
		logger.Log4.Error("UserId-%d: connet err:%s", r.mRobotData.MUId, err)
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
			logger.Log4.Error("UserId-%d: msgHead :%v", r.mRobotData.MUId, msgHead)
			r.FsmSendEvent(RobotEventRemoteMsg, msgHead)
		}

	}()
	return nil

}

//登陆状态：处理登陆lobbysvr
func (r *Robot) RobotStateLoginEventLoginLobbySvrd(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

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
