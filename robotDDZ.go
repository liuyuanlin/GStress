package main

import (
	"GStress/fsm"
	"GStress/logger"
	"GStress/msg/ClubSubCmd"
	"GStress/msg/GameSubUserCmd"
	"GStress/msg/LobbySubCmd"
	"GStress/msg/LobbySubMatch"
	"GStress/msg/Main"
	"GStress/net"
	"errors"

	"github.com/golang/protobuf/proto"
)

//RobotStateDdzEventInitNew 斗地主状态-新
func (r *Robot) RobotStateDdzEventInitNew(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	if r.mIsLoginOk == false {
		r.mCurTaskStepReuslt = TaskResultNotLogin
	}
	r.FsmSendEvent(RobotEventTaskAnalysis, nil)
}

//RobotStateDdzEventQuitNew ...
func (r *Robot) RobotStateDdzEventQuitNew(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)
	r.mIsEnterRoom = false
}

//RobotStateDdzEventRemoteMsgNew 斗地主状态：处理远程消息
func (r *Robot) RobotStateDdzEventRemoteMsgNew(e *fsm.Event) {
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

//HandelUserLeaveRoom 机器人主动离开房间
func (r *Robot) HandelUserLeaveRoom(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if msgHead == nil || r.mRobotData.MDdzRoomType != 1 { //暂时只有金币场处理离开房间
		return
	}

	m := &GameSubUserCmd.CSUserLeaveRoomRqst{
		RoomId: proto.Int32(int32(r.mRobotData.MDdzRoomId)),
	}

	//err := r.mNetClient.SenMsg(int16(Main.EnMainCmdID_GAME_MAIN_USER), int16(GameSubUserCmd.EnSubCmdID_GAME_SUB_SC_USER_FRAME_ENTER), m)
	err := r.mNetClient.SenGameMsg(int16(Main.EnMainCmdID_GAME_MAIN_USER),
		int16(GameSubUserCmd.EnSubCmdID_GAME_SUB_CS_USER_LEAVE_ROOM_RQST), int16(r.mRobotData.MDdzRoomId), m)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
	}
}

//RobotStateDdzEventSocketAbnormalNew 斗地主状态：处理网络异常
func (r *Robot) RobotStateDdzEventSocketAbnormalNew(e *fsm.Event) {
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

//RobotStateDdzEventTimerNew 斗地主状态：处理定时事件
func (r *Robot) RobotStateDdzEventTimerNew(e *fsm.Event) {
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

//RobotStateDdzEventTaskAnalysisNew 斗地主状态：处理任务解析
func (r *Robot) RobotStateDdzEventTaskAnalysisNew(e *fsm.Event) {
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
	case TaskStepDdzEnterRoomNew:
		r.FsmSendEvent(RobotEventDDZEnterRoomNew, nil)
		break
	case TaskStepDdzStartGameNew:
		r.FsmSendEvent(RobotEventDdzStartGameNew, nil)
		break
	default:
		break
	}
	return
}

//RobotStateDdzEventEnterRoomNew 斗地主状态： 进入房间
func (r *Robot) RobotStateDdzEventEnterRoomNew(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	//进入房间
	var err error
	if r.mRobotData.MDdzRoomType == 1 { //进入金币场
		err = r.RequestEnterDdzGoldRoom()
	} else if r.mRobotData.MDdzRoomType == 3 { //进入比赛场
		err = r.RequestEnterDdzMatchRoom()
	} else {
		//不需要进入
		r.mCurTaskStepReuslt = TaskResultSuccess
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		logger.Log4.Error("<ENTER> :UserId-%d:CurState：%s 不需要进入房间",
			r.mRobotData.MUId, e.FSM.CurState())
		return
	}

	if err != nil {
		//网络连接失败，结束当前任务
		r.mCurTaskStepReuslt = TaskResultClub_SendRequestEnterRoomFail
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
		return
	} else {
		//设置请求定时器
		//r.SetTimer(TimerClubEnterRoom, RequestTimeOut)
		r.mCurTaskStepReuslt = TaskResultSuccess
		r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	}

}

//RobotStateDdzEventStartGameNew 斗地主状态： 开始游戏
func (r *Robot) RobotStateDdzEventStartGameNew(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mRobotData.MDdzRoomType == 1 {
		r.RequestStartDdzGoldGame()
	} else {
		r.RequestStartDdzMatchGame()
	}

	r.mCurTaskStepReuslt = TaskResultSuccess
	r.FsmSendEvent(RobotEventTaskAnalysis, nil)
	//r.mCurTaskStepReuslt = TaskResultSuccess
	//r.FsmSendEvent(RobotEventTaskAnalysis, nil)
}

//RobotStateDdzEventOperateNew 斗地主状态： 游戏操作
func (r *Robot) RobotStateDdzEventOperateNew(e *fsm.Event) {
	logger.Log4.Debug("<ENTER> :UserId-%d:CurState：%s", r.mRobotData.MUId, e.FSM.CurState())
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	return

}

//RequestEnterDdzGoldRoom ...
func (r *Robot) RequestEnterDdzGoldRoom() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}

	m := &ClubSubCmd.CS_ClubEnterRoom{
		ClubID:       proto.Int64(int64(0)),
		RoomId:       proto.Int64(int64(r.mRobotData.MDdzRoomId)),
		TableId:      proto.Int64(int64(0)),
		UserLng:      proto.Int64(int64(0)),
		UserLat:      proto.Int64(int64(0)),
		RoomType:     proto.Int64(int64(r.mRobotData.MDdzRoomType)),
		MMatchTypeId: proto.Int32(int32(0)),
		MatchDayTime: proto.Int64(int64(0)),
	}

	//fmt.Printf("%v", m)
	err := r.mNetClient.SenMsg(int16(Main.EnMainCmdID_CLUB_MAIN_CMD), int16(ClubSubCmd.EnClientSubCmdID_CS_CLUB_ENTER_ROOM), m)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
		return errors.New("ERR_NET_SEND_FAIL")
	}
	return nil
}

//RequestEnterDdzMatchRoom ...
func (r *Robot) RequestEnterDdzMatchRoom() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}

	m := &LobbySubMatch.CMD_MATCH_ACTOR_ENTER_MATCH_CS_MSG{
		IRoomId:      proto.Int32(int32(r.mRobotData.MDdzRoomId)),
		IMatchTypeId: proto.Int32(int32(r.mRobotData.MDdzMatchTypeId)),
	}

	err := r.mNetClient.SenMsg(int16(Main.EnMainCmdID_LOBBY_MAIN_MATCH), int16(LobbySubMatch.EnSubCmdID_CMD_MATCH_ACTOR_ENTER_MATCH_CS), m)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
		return errors.New("ERR_NET_SEND_FAIL")
	}
	return nil
}

//RequestStartDdzGoldGame ...
func (r *Robot) RequestStartDdzGoldGame() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}

	m := &LobbySubCmd.REQUEST_START_DDZ_CMD_CS_MSG{
		IRoomId: proto.Uint32(uint32(r.mRobotData.MDdzRoomId)),
	}

	err := r.mNetClient.SenMsg(int16(Main.EnMainCmdID_LOBBY_MAIN_CMD), int16(LobbySubCmd.EnSubCmdID_REQUEST_START_DDZ_CMD_CS), m)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
		return errors.New("ERR_NET_SEND_FAIL")
	}
	return nil
}

//RequestStartDdzMatchGame ...
func (r *Robot) RequestStartDdzMatchGame() error {
	logger.Log4.Debug("<ENTER> :UserId-%d", r.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", r.mRobotData.MUId)

	if r.mNetClient == nil {
		logger.Log4.Error("UserId-%d: no connect", r.mRobotData.MUId)
		return errors.New("ERR_NO_NET_CONNECT")
	}

	m := &GameSubUserCmd.CSUserFrameEnter{
		RoomId: proto.Int32(int32(r.mRobotData.MDdzRoomId)),
	}

	//err := r.mNetClient.SenMsg(int16(Main.EnMainCmdID_GAME_MAIN_USER), int16(GameSubUserCmd.EnSubCmdID_GAME_SUB_SC_USER_FRAME_ENTER), m)
	err := r.mNetClient.SenGameMsg(int16(Main.EnMainCmdID_GAME_MAIN_USER),
		int16(GameSubUserCmd.EnSubCmdID_GAME_SUB_SC_USER_FRAME_ENTER), int16(r.mRobotData.MDdzRoomId), m)
	if err != nil {
		logger.Log4.Error("UserId-%d: send fail", r.mRobotData.MUId)
		return errors.New("ERR_NET_SEND_FAIL")
	}
	return nil
}
