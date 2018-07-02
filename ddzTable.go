package main

import (
	"GStress/logger"
	"GStress/msg/DouDiZhu"
	"GStress/net"
	"errors"

	"github.com/golang/protobuf/proto"
)

//桌子状态
const (
	Ddz_Table_State_Init            = 1
	Ddz_Table_State_SendCard        = 2 //发牌
	Ddz_Table_State_EnsureLandOwner = 3 //确定地主
	Ddz_Table_State_OutCard         = 4 //出牌
	Ddz_Table_State_Game_End        = 5 //游戏结束
	Ddz_Table_State_Dissolve        = 6
	Ddz_Table_State_Wait_Dissolve   = 7 //等待解散投票
)

//玩家状态
const (
	Ddz_Mj_User_State_Init           = 1 //初始化
	Ddz_Mj_User_State_Sit            = 2 //坐下
	Ddz_Mj_User_State_Ready          = 3 //准备
	Ddz_Mj_User_State_ShowCard       = 4 //玩家在发牌中
	Ddz_Mj_User_State_QiangLandOwner = 5 //玩家在抢地主中
	Ddz_Mj_User_State_OutCard        = 6 //玩家在出牌中
	Ddz_Mj_User_State_End            = 7 // 结束
)

//扑克操作码
const (
	Ddz_enSeat_Operate_NONE            = 0 //不操作
	Ddz_enSeat_Operate_Ask_LandOwner   = 1 //叫地主
	Ddz_enSeat_Operate_Qiang_LandOwner = 2 //抢地主
	Ddz_enSeat_Operate_Discard         = 3 //出牌
	Ddz_enSeate_Operate_ASK_Score      = 4 //叫分

)

//牌类型
const (
	Ddz_enCardType_CT_ERROR               = 0  //错误类型
	Ddz_enCardType_CT_SINGLE              = 1  //单牌类型
	Ddz_enCardType_CT_DOUBLE              = 2  //对牌类型
	Ddz_enCardType_CT_THREE               = 3  //三条类型
	Ddz_enCardType_CT_SINGLE_LINE         = 4  //单连类型
	Ddz_enCardType_CT_DOUBLE_LINE         = 5  //对连类型
	Ddz_enCardType_CT_THREE_LINE          = 6  //三连类型
	Ddz_enCardType_CT_THREE_TAKE_ONE      = 7  //三带一单
	Ddz_enCardType_CT_THREE_TAKE_ONE_LINE = 8  //三带一连牌
	Ddz_enCardType_CT_THREE_TAKE_TWO      = 9  //三带一对
	Ddz_enCardType_CT_THREE_TAKE_TWO_LINE = 10 //三带一对连牌
	Ddz_enCardType_CT_FOUR_TAKE_ONE       = 11 //四带两单
	Ddz_enCardType_CT_FOUR_TAKE_ONE_LINE  = 12 //四带两张连牌
	Ddz_enCardType_CT_FOUR_TAKE_TWO       = 13 //四带两对
	Ddz_enCardType_CT_FOUR_TAKE_TWO_LINE  = 14 //四带两对连牌
	Ddz_enCardType_CT_RUAN_BOMB           = 15 //软炸弹
	Ddz_enCardType_CT_BOMB_CARD           = 16 //炸弹类型
	Ddz_enCardType_CT_LAIZI_BOMB          = 17 //赖子炸弹
	Ddz_enCardType_CT_MISSILE_CARD        = 18 //火箭类型

)

type DdzTableAttribute struct {
	MGameCurrencyType       int
	MPayRoomRateType        int
	MPlanGameCount          int
	MDiZhu                  int
	MEnterScore             int
	MLeaveScore             int
	MBeiShuLimit            int
	MChairCount             int
	MIsAllowEnterAfterStart int
	MTableType              int
	MRoomRate               int
	MServerRate             int
	// 高级参数部分
	MGameType             int
	IConfirmLandownerType int

	MOwnerUserId       int
	MTableAdvanceParam string
	MTableName         string
	MRoomId            int
	MTableId           int
	MKindId            int
	MClubId            int
	MIsIpWarn          int
	MIsGpsWarn         int
}

type DdzTable struct {
	MRobot              *Robot
	MDiZhuSeatId        int
	MTableState         int
	MGameCount          int
	MWaitReadyTime      int
	MWaitEndTime        int
	MAllWaitOperateTime int
	MSelfSeatId         int
	MTableAttribute     DdzTableAttribute
	MTableUser          [3]DdzTableUser
}

func (t *DdzTable) Init(robot *Robot) error {
	if robot == nil {
		logger.Log4.Error("robot param error")
		return errors.New("ERR_PARAM")
	}
	t.MRobot = robot
	logger.Log4.Debug("<ENTER> :UserId-%d", t.MRobot.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", t.MRobot.mRobotData.MUId)

	for i, user := range t.MTableUser {
		user.MSeatId = i
	}

	t.MSelfSeatId = -1

	return nil
}

func (t *DdzTable) HandelGameMainMsg(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", t.MRobot.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", t.MRobot.mRobotData.MUId)

	if msgHead == nil {
		return
	}
	switch msgHead.MSubCmd {
	case int16(DouDiZhu.EnGameFrameID_GAME_SUB_SC_USER_TABLE_NOTIFY):
		t.HandleTableInfoNotify(msgHead)
		break
	default:
		break
	}

	return
}

func (t *DdzTable) HandleTableInfoNotify(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", t.MRobot.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", t.MRobot.mRobotData.MUId)

	if msgHead == nil {
		return
	}

	tableInfoNotify := &DouDiZhu.SC_TableInfoNotify{}
	err := proto.Unmarshal(msgHead.MData, tableInfoNotify)
	if err != nil {
		logger.Log4.Debug("unmarshal SC_TableInfoNotify error: %s", err)
	}
	logger.Log4.Debug("tableInfoNotify: %+v", tableInfoNotify)

	tableInfo := tableInfoNotify.GetMTableInfo()

	//更新桌子信息
	//1.桌子属性
	tableAttr := tableInfo.GetMTableAttribute()
	//更新桌子属性
	t.UpdateTableAttribute(tableAttr)
	t.MDiZhuSeatId = int(tableInfo.GetMLandOwnerSeatId())
	t.MGameCount = int(tableInfo.GetMGameCount())
	t.MAllWaitOperateTime = int(tableInfo.GetMAllWaitOperateTime())
	t.MTableState = int(tableInfo.GetMTableState())
	t.MWaitEndTime = int(tableInfo.GetMWaitEndTime())
	t.MWaitReadyTime = int(tableInfo.GetMWaitReadyTime())

	//更新桌子用户信息
	t.UpdateTableUserInfo(tableInfo.GetSzSeatInfos())

	//处理相关逻辑
	if t.MTableState == Ddz_Table_State_Game_End {
		t.MRobot.ReportDdzGameEnd()
	}

}

func (t *DdzTable) UpdateTableUserInfo(seatInfo []*DouDiZhu.SeatInfo) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", t.MRobot.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", t.MRobot.mRobotData.MUId)
	if seatInfo == nil {
		return
	}
	for _, user := range seatInfo {
		lSeatId := user.GetMSeatId()
		if lSeatId < 0 || lSeatId >= 3 {
			logger.Log4.Error("<LEAVE>:UserId-%d: seatd id %d is error", t.MRobot.mRobotData.MUId, lSeatId)
			continue
		}
		t.MTableUser[lSeatId].MoffLineFlag = bool(user.GetMOffLineFlag())
		t.MTableUser[lSeatId].MUserState = int(user.GetMUserState())

		//更新基本信息
		userBaseInfo := user.GetMBaseInfo()
		t.MTableUser[lSeatId].MBaseInfo.MUserID = int(userBaseInfo.GetMUserID())
		t.MTableUser[lSeatId].MBaseInfo.MAccid = int(userBaseInfo.GetMAccid())
		t.MTableUser[lSeatId].MBaseInfo.MNickName = userBaseInfo.GetMNickName()
		t.MTableUser[lSeatId].MBaseInfo.MFaceID = userBaseInfo.GetMFaceID()
		t.MTableUser[lSeatId].MBaseInfo.MSex = int(userBaseInfo.GetMSex())
		t.MTableUser[lSeatId].MBaseInfo.MUserGold = int(userBaseInfo.GetMUserGold())
		t.MTableUser[lSeatId].MBaseInfo.MUserDiamond = int(userBaseInfo.GetMUserDiamond())
		t.MTableUser[lSeatId].MBaseInfo.MDescription = userBaseInfo.GetMDescription()
		t.MTableUser[lSeatId].MBaseInfo.MVipLevel = int(userBaseInfo.GetMVipLevel())
		t.MTableUser[lSeatId].MBaseInfo.MUserPoint = int(userBaseInfo.GetMUserPoint())
		t.MTableUser[lSeatId].MBaseInfo.MUserIp = userBaseInfo.GetMUserIp()
		t.MTableUser[lSeatId].MBaseInfo.MUserLng = int(userBaseInfo.GetMUserLng())
		t.MTableUser[lSeatId].MBaseInfo.MUserLat = int(userBaseInfo.GetMUserLat())

		if t.MTableUser[lSeatId].MBaseInfo.MUserID == t.MRobot.mRobotData.MUserId {
			t.MSelfSeatId = int(lSeatId)
		}

	}
}

func (t *DdzTable) UpdateTableAttribute(tableAttr *DouDiZhu.StTableAttribute) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", t.MRobot.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", t.MRobot.mRobotData.MUId)

	if tableAttr == nil {
		return
	}
	t.MTableAttribute.MGameCurrencyType = int(tableAttr.GetGameCurrencyType())
	t.MTableAttribute.MPayRoomRateType = int(tableAttr.GetPayRoomRateType())
	t.MTableAttribute.MPlanGameCount = int(tableAttr.GetPlanGameCount())
	t.MTableAttribute.MDiZhu = int(tableAttr.GetDiZhu())
	t.MTableAttribute.MEnterScore = int(tableAttr.GetEnterScore())
	t.MTableAttribute.MLeaveScore = int(tableAttr.GetLeaveScore())
	t.MTableAttribute.MBeiShuLimit = int(tableAttr.GetBeiShuLimit())
	t.MTableAttribute.MChairCount = int(tableAttr.GetChairCount())
	t.MTableAttribute.MIsAllowEnterAfterStart = int(tableAttr.GetIsAllowEnterAfterStart())
	t.MTableAttribute.MTableType = int(tableAttr.GetTableType())
	t.MTableAttribute.MRoomRate = int(tableAttr.GetRoomRate())
	t.MTableAttribute.MServerRate = int(tableAttr.GetServerRate())
	// 高级参数部分
	t.MTableAttribute.MGameType = int(tableAttr.GetMGameType())
	t.MTableAttribute.IConfirmLandownerType = int(tableAttr.GetIconfirmLandownerType())

	t.MTableAttribute.MOwnerUserId = int(tableAttr.GetMOwnerUserId())
	t.MTableAttribute.MTableAdvanceParam = tableAttr.GetTableAdvanceParam()
	t.MTableAttribute.MTableName = tableAttr.GetMTableName()
	t.MTableAttribute.MRoomId = int(tableAttr.GetMRoomId())
	t.MTableAttribute.MTableId = int(tableAttr.GetMTableId())
	t.MTableAttribute.MKindId = int(tableAttr.GetMKindId())
	t.MTableAttribute.MClubId = int(tableAttr.GetMClubId())
	t.MTableAttribute.MIsIpWarn = int(tableAttr.GetMIsIpWarn())
	t.MTableAttribute.MIsGpsWarn = int(tableAttr.GetMIsGpsWarn())

	logger.Log4.Debug("UserId-%d: MTableAttribute:%v", t.MRobot.mRobotData.MUId, t.MTableAttribute)

	return

}
