package main

import (
	"GStress/fsm"
	"GStress/logger"
	"GStress/msg/ClubSubCmd"
	"GStress/msg/GateSubCmd"
	"GStress/msg/LobbySubCmd"
	"GStress/msg/LoginSubCmd"
	"GStress/msg/Main"
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
	RequestTimeOut = time.Second * 20
)

type TimerType int

const (
	TimerLoginRegister = iota
	TimerLoginLoginSvr
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
	RobotEventRegister       = "RobotEventRegister"
	RobotEventLoginLoginSvrd = "RobotEventLoginLoginSvrd"
	RobotEventLoginLobbySvrd = "RobotEventLoginLobbySvrd"
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
				r.mNetClient = nil
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
		time.Sleep(1000 * time.Millisecond)
		logger.Log4.Info("UserId-%d:wait evet", r.mRobotData.MUId)
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
	default:
		break
	}

	return
}

func (r *Robot) HandelClubBaseInfo(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil {
		return
	}

	//微游戏基础信息收到后，则整个登陆完成，取消定时器
	r.CancelTimer(TimerLoginLobbySvr)

	clubBaseInfo := &ClubSubCmd.SC_ClubBaseInfoOnLogin{}
	err := proto.Unmarshal(msgHead.MData, clubBaseInfo)
	if err != nil {
		logger.Log4.Debug("unmarshal SC_ClubBaseInfoOnLogin error: %s", err)
	}
	logger.Log4.Debug("clubBaseInfo: %+v", clubBaseInfo)

	r.mCurTaskStepReuslt = TaskResultSuccess
	r.FsmSendEvent(RobotEventTaskAnalysis, nil)

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
		r.mCurTaskStepReuslt = TaskResultLogin_Loginsvr_RegisterResponseFail
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

	if r.mCurTaskStepReuslt != TaskResultNone {
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
			logger.Log4.Error("UserId-%d: msgHead :%v", r.mRobotData.MUId, msgHead)
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
		for {
			msgHead, err := r.mNetClient.ReadMsg()
			if err != nil {
				logger.Log4.Error("UserId-%d: Gate ReadMsg err:%s", r.mRobotData.MUId, err)
				r.FsmSendEvent(RobotEventSocketAbnormal, nil)
				return
			} else {
				logger.Log4.Error("UserId-%d: Gate msgHead :%v", r.mRobotData.MUId, msgHead)
				r.FsmSendEvent(RobotEventRemoteMsg, msgHead)
			}
		}

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
