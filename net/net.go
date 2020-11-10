package net

import (
	"bytes"
	"encoding/binary"

	"net/url"

	"GStress/logger"
	"GStress/msg/ClientCommon"
	"errors"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
)

func Int32ToBytes(i int32) []byte {
	var buf = make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(i))
	return buf
}

type MsgHead struct {
	MMainCmd int16
	MSubCmd  int16
	MData    []byte
}

type ProtoCmd struct {
	cmd         int16
	para        int16
	dwTimeStamp int32
	roomID      int16
	size        int32
}

//TODO-liuyuanlin:需要设计锁保护，考虑网络断开情况，战时为考虑
type NetClient struct {
	mAddr string
	mPath string
	mConn *websocket.Conn
}

func NewNetClient(addr string, path string) (*NetClient, error) {

	u := url.URL{Scheme: "ws", Host: addr, Path: "/ws"}
	logger.Log4.Debug("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		logger.Log4.Error("dial:", err)
		return nil, err
	}

	n := &NetClient{
		mAddr: addr,
		mPath: path,
		mConn: c,
	}

	return n, nil
}

//TODO-liuyuanlin:需要加锁
func (n *NetClient) Close() {

	if n.mConn != nil {
		n.mConn.Close()
		n.mConn = nil
	}

	return
}

//TODO-liuyuanlin:
func (n *NetClient) SenMsg(sendmsg *ClientCommon.SendData) error {

	if n.mConn == nil {

		return errors.New("no conn")
	}
	logger.Log4.Info("send-msg: %v", sendmsg)
	sendData, err := proto.Marshal(sendmsg)
	if err != nil {
		logger.Log4.Error("marshaling error: ", err)
		return err
	}

	var buffer bytes.Buffer //Buffer是一个实现了读写方法的可变大小的字节缓冲

	buffer.Write(sendData)
	b3 := buffer.Bytes()

	//发送数据
	err = n.mConn.WriteMessage(websocket.BinaryMessage, b3)
	if err != nil {
		logger.Log4.Error("write:", err)
		return err
	}

	return nil
}

//TODO-liuyuanlin:
func (n *NetClient) ReadMsg() (*(ClientCommon.PushData), error) {
	logger.Log4.Debug("<ENTER>")
	defer logger.Log4.Debug("<LEAVE>")

	//检查连接
	if n.mConn == nil {

		return nil, errors.New("no conn")
	}

	_, buf, err := n.mConn.ReadMessage()
	if err != nil {
		logger.Log4.Error("read:", err)
		return nil, err

	}

	pushdata := new(ClientCommon.PushData)
	proto.Unmarshal(buf, pushdata)

	logger.Log4.Info("recv: %v", buf)

	logger.Log4.Info("recv-msg: %v", pushdata)
	logger.Log4.Debug("pushdata.CmdId:%d", pushdata.CmdId)

	return pushdata, nil
}
