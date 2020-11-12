package main

import (
	"GStress/fsm"
	"GStress/logger"
	"GStress/msg/ClientCommon"
	"GStress/msg/redenvelopegame"
	"errors"

	"github.com/golang/protobuf/proto"
)

/*
	//砸金蛋游戏状态
	r.mFsm.AddStateEvent(RobotStateEgg, RobotEventInit, r.RobotStateEggEventInit)
	r.mFsm.AddStateEvent(RobotStateEgg, RobotEventQuit, r.RobotStateEggEventQuit)
	r.mFsm.AddStateEvent(RobotStateEgg, RobotEventRemoteMsg, r.RobotStateEggEventRemoteMsg)
	r.mFsm.AddStateEvent(RobotStateEgg, RobotEventSocketAbnormal, r.RobotStateEggEventSocketAbnormal)
	r.mFsm.AddStateEvent(RobotStateEgg, RobotEventTimer, r.RobotStateEggEventTimer)
	r.mFsm.AddStateEvent(RobotStateEgg, RobotEventTaskAnalysis, r.RobotStateEggEventTaskAnalysis)
	r.mFsm.AddStateEvent(RobotStateEgg, RobotEventEggGameWait, r.RobotStateEggGameWait)
	r.mFsm.AddStateEvent(RobotStateEgg, RobotEventEggGamePlay, r.RobotStateEggGamePlay)
	r.mFsm.AddStateEvent(RobotStateEgg, RobotEventEggGameEnd, r.RobotStateEggGameEnd)
*/
func (r *Robot) RobotStateEggEventInit(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	r.FsmSendEvent(RobotEventTaskAnalysis, nil)
}

func (r *Robot) RobotStateEggEventQuit(e *fsm.Event) {
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

//砸金蛋状态：处理远程消息
func (r *Robot) RobotStateEggEventRemoteMsg(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if len(e.Args) == 0 {
		logger.Log4.Error("UserId-%d: no Remote Msg", r.mRobotData.MUId)
		return
	}
	msg := e.Args[0].(*ClientCommon.PushData)
	logger.Log4.Debug("UserId-%d:msg.CmdId:%d, GameId:%d", r.mRobotData.MUId, msg.CmdId, msg.GameId)
	if msg.GameId != 1 {
		logger.Log4.Debug("UserId-%d: ignore msg.CmdId:%d,  GameId:%d  msg", r.mRobotData.MUId, msg.CmdId, msg.GameId)
		return
	}
	switch msg.CmdId {
	case int64(redenvelopegame.GoldenEggCMD_GOLDENEGG_LIVE_START):
		r.HandelEggGameStart(msg)
		break
	case int64(redenvelopegame.GoldenEggCMD_GOLDENEGG_OVER):
		r.HandelEggGameOver(msg)
		break
	case int64(redenvelopegame.GoldenEggCMD_GOLDENEGG_CLIENT_OPEN):
		r.HandelEggGameOpen(msg)

	default:
		break
	}

}

//砸金蛋状态：处理网络异常
func (r *Robot) RobotStateEggEventSocketAbnormal(e *fsm.Event) {
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

//砸金蛋状态：处理定时事件
func (r *Robot) RobotStateEggEventTimer(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	if len(e.Args) == 0 {
		logger.Log4.Error("UserId-%d: no Remote Msg", r.mRobotData.MUId)
		return
	}
	timerData := e.Args[0].(*TimertData)
	switch timerData.MTimerType {

	default:
		break
	}
}

//砸金蛋状态：处理任务解析
func (r *Robot) RobotStateEggEventTaskAnalysis(e *fsm.Event) {
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
	case TaskStepEggWait:
		r.FsmSendEvent(RobotEventEggGameWait, nil)
		break
	case TaskStepEggPlay:
		r.FsmSendEvent(RobotEventEggGamePlay, nil)
		break
	case TaskStepEggEnd:
		r.FsmSendEvent(RobotEventEggGameEnd, nil)
		break
	default:
		break
	}
	return
}

//砸金蛋状态：等待游戏开始
func (r *Robot) RobotStateEggGameWait(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mIsLoginOk != true {
		//网络连接失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultNotLogin
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	}

	/*
	     WAIT=1;//等待开始
	       PLAYING=2;//进行中
	       GAMEOVER=3;//已结束
	   }
	*/
	logger.Log4.Debug("UserId-%d: 游戏状态：%d", r.mRobotData.MUId, r.mRobotData.mEggGameInfo.State)
	if r.mRobotData.mEggGameInfo.State != 1 && r.mRobotData.mEggGameInfo.State != 0 {
		//游戏处于其他状态, 直接进入下一步骤
		r.mCurTaskStepReuslt = TaskResultSuccess
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	}

	return
}

//砸金蛋状态：游戏中
func (r *Robot) RobotStateEggGamePlay(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	if r.mIsLoginOk != true {
		//网络连接失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultNotLogin
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	}

	/*
	     WAIT=1;//等待开始
	       PLAYING=2;//进行中
	       GAMEOVER=3;//已结束
	   }
	*/
	logger.Log4.Debug("UserId-%d: 游戏状态：%d", r.mRobotData.MUId, r.mRobotData.mEggGameInfo.State)
	if r.mRobotData.mEggGameInfo.State != 2 {
		//游戏处于其他状态, 直接进入下一步骤
		r.mCurTaskStepReuslt = TaskResultSuccess
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	}

	err := r.RequestEggClientOpen()
	if err != nil {
		//网络连接失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultEgg_SendEggOpenRequestFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	} else {
		//游戏过程不用设置定时器
		//r.SetTimer(TimerLoginLoginSvr, RequestTimeOut)

	}
	return
}

//砸金蛋状态：游戏结束
func (r *Robot) RobotStateEggGameEnd(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%d", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	if r.mIsLoginOk != true {
		//网络连接失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultNotLogin
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	}

	/*
	     WAIT=1;//等待开始
	       PLAYING=2;//进行中
	       GAMEOVER=3;//已结束
	   }
	*/
	logger.Log4.Debug("UserId-%d: 游戏状态：%d", r.mRobotData.MUId, r.mRobotData.mEggGameInfo.State)

	//如果无数据要求直接结束
	r.mCurTaskStepReuslt = TaskResultSuccess
	r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	return

}

func (r *Robot) RequestEggClientOpen() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}

	m := &redenvelopegame.ClientGoldenEggOpenReq{
		BatchId: r.mRobotData.mEggGameInfo.BatchId,
	}

	data, err := proto.Marshal(m)
	if err != nil {
		logger.Log4.Error("UserId-%d: marshaling error: ", r.mRobotData.MUId, err)
		return err
	}
	send := &ClientCommon.SendData{
		GameId: r.mRobotData.GameId,
		CmdId:  int64(redenvelopegame.GoldenEggCMD_GOLDENEGG_CLIENT_OPEN),
		Data:   data,
	}
	err = r.mNetClient.SenMsg(send)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
		return errors.New("ERR_NET_SEND_FAIL")
	}
	return nil
}

func (r *Robot) HandelEggGameStart(msg *ClientCommon.PushData) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msg == nil {
		return
	}

	GoldenEggStateChangeRspInfo := &redenvelopegame.GoldenEggStateChangeRsp{}
	err := proto.Unmarshal(msg.Data, GoldenEggStateChangeRspInfo)
	if err != nil {
		logger.Log4.Debug("UserId-%d: unmarshal ClientGoldenEggLoginRsp error: %s", r.mRobotData.MUId, err)
	}
	logger.Log4.Debug("UserId-%d: msg: %+v", r.mRobotData.MUId, msg)
	if msg.Data == nil {
		logger.Log4.Debug("UserId-%d: ClientGoldenEggLoginRsp is  NULL:", r.mRobotData.MUId)
		return
	}

	logger.Log4.Debug("UserId-%d:GoldenEggStateChangeRspInfo: %+v", r.mRobotData.MUId, GoldenEggStateChangeRspInfo)

	if GoldenEggStateChangeRspInfo.Code != redenvelopegame.Code_SUCCESS {
		logger.Log4.Debug("UserId-%d: 消息错误 GoldenEggStateChangeRspInfo.Code:%v ", r.mRobotData.MUId, GoldenEggStateChangeRspInfo.Code)
		return
	}
	/*
		//cmdid = 4时游戏开始的通知
		//cmdid = 6时游戏结束的通知
		message GoldenEggStateChangeRsp{
			Code Code=1;
			int64 BatchId = 2; //批次Id
			RedEnvelopeBatchStatus State = 3; //该轮游戏状态
			int64 SystemTime = 4;//系统时间
			int64 StartTime = 5;//游戏开始时间
			int64 EndTime = 6;//游戏结束时间
		}

	*/
	tmpOldState := r.mRobotData.mEggGameInfo.State
	r.mRobotData.mEggGameInfo.BatchId = GoldenEggStateChangeRspInfo.BatchId
	r.mRobotData.mEggGameInfo.State = int64(GoldenEggStateChangeRspInfo.State)
	r.mRobotData.mEggGameInfo.SystemTime = GoldenEggStateChangeRspInfo.SystemTime
	r.mRobotData.mEggGameInfo.StartTime = GoldenEggStateChangeRspInfo.StartTime
	r.mRobotData.mEggGameInfo.EndTime = GoldenEggStateChangeRspInfo.EndTime

	logger.Log4.Debug("UserId-%d: OldState:%d  , newState:%d ", r.mRobotData.MUId, tmpOldState, r.mRobotData.mEggGameInfo.State)
	if tmpOldState != r.mRobotData.mEggGameInfo.State {
		//状态变化，触发下一步骤
		r.mCurTaskStepReuslt = TaskResultSuccess
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	}

}

func (r *Robot) HandelEggGameOver(msg *ClientCommon.PushData) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	r.HandelEggGameStart(msg)

}

func (r *Robot) HandelEggGameOpen(msg *ClientCommon.PushData) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msg == nil {
		return
	}

	ClientGoldenEggOpenRspInfo := &redenvelopegame.ClientGoldenEggOpenRsp{}
	err := proto.Unmarshal(msg.Data, ClientGoldenEggOpenRspInfo)
	if err != nil {
		logger.Log4.Debug("UserId-%d: unmarshal ClientGoldenEggOpenRsp error: %s", r.mRobotData.MUId, err)
	}
	logger.Log4.Debug("UserId-%d:msg: %+v", r.mRobotData.MUId, msg)
	if msg.Data == nil {
		logger.Log4.Debug("UserId-%d:ClientGoldenEggOpenRsp is  NULL:", r.mRobotData.MUId, r.mRobotData.MUId)
		return
	}

	logger.Log4.Debug("UserId-%d:ClientGoldenEggOpenRspInfo: %+v", r.mRobotData.MUId, ClientGoldenEggOpenRspInfo)

	/*
		//cmdid = 5手机端提交开奖结果
		message ClientGoldenEggOpenRsp{
			Code Code= 1;
			PBRedEnvelopeWinnerRecord Prize = 2; //奖品数据 为空没有中奖
		}
	*/

	if r.mRobotData.mEggGameInfo.State == 2 {
		//游戏处于其他状态, 继续请求
		err = r.RequestEggClientOpen()
		if err != nil {
			//网络连接失败，结束当前任务
			r.mCurTaskStepReuslt = TaskResultEgg_SendEggOpenRequestFail
			r.FsmSendEvent(RobotEventTaskAnalysis, nil)
			return
		}
		return
	}

}
