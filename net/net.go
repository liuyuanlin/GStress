package net

import (
	"bytes"
	"encoding/binary"

	"net/url"

	"GStress/logger"
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

	u := url.URL{Scheme: "ws", Host: addr, Path: ""}
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
func (n *NetClient) SenMsg(mainCmd int16, paraCmd int16, pb proto.Message) error {

	if n.mConn == nil {

		return errors.New("no conn")
	}
	//消息体封装
	data, err := proto.Marshal(pb)
	if err != nil {
		logger.Log4.Error("marshaling error: ", err)
		return err
	}

	//消息头封装
	var lProtoCmd ProtoCmd
	lProtoCmd.size = int32(len(data))
	lProtoCmd.cmd = mainCmd
	lProtoCmd.para = paraCmd

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, lProtoCmd)
	b1 := buf.Bytes()

	var buffer bytes.Buffer //Buffer是一个实现了读写方法的可变大小的字节缓冲

	//整个消息长度封装
	var lAllLen int32 = 0
	lAllLen = int32(len(b1) + len(data))
	b0 := Int32ToBytes(lAllLen)

	//发送内容拼接
	buffer.Write(b0)
	buffer.Write(b1)
	buffer.Write(data)
	b3 := buffer.Bytes()

	//发送数据
	err = n.mConn.WriteMessage(websocket.BinaryMessage, b3)
	if err != nil {
		logger.Log4.Error("write:", err)
		return err
	}

	return nil
}

func (n *NetClient) SenGameMsg(mainCmd int16, paraCmd int16, roomId int16, pb proto.Message) error {

	if n.mConn == nil {

		return errors.New("no conn")
	}
	//消息体封装
	data, err := proto.Marshal(pb)
	if err != nil {
		logger.Log4.Error("marshaling error: ", err)
		return err
	}

	//消息头封装
	var lProtoCmd ProtoCmd
	lProtoCmd.size = int32(len(data))
	lProtoCmd.cmd = mainCmd
	lProtoCmd.para = paraCmd
	lProtoCmd.roomID = roomId

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, lProtoCmd)
	b1 := buf.Bytes()

	var buffer bytes.Buffer //Buffer是一个实现了读写方法的可变大小的字节缓冲

	//整个消息长度封装
	var lAllLen int32 = 0
	lAllLen = int32(len(b1) + len(data))
	b0 := Int32ToBytes(lAllLen)

	//发送内容拼接
	buffer.Write(b0)
	buffer.Write(b1)
	buffer.Write(data)
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
func (n *NetClient) ReadMsg() (*MsgHead, error) {
	logger.Log4.Debug("<ENTER>")
	defer logger.Log4.Debug("<LEAVE>")

	var lMsgHead MsgHead
	//检查连接
	if n.mConn == nil {

		return nil, errors.New("no conn")
	}

	_, buf, err := n.mConn.ReadMessage()
	if err != nil {
		logger.Log4.Error("read:", err)
		return nil, err

	}

	logger.Log4.Info("recv: %v", buf)
	lProtoCmd := new(ProtoCmd)
	lProtoCmd.cmd = int16(binary.LittleEndian.Uint16(buf[4:6]))
	lProtoCmd.para = int16(binary.LittleEndian.Uint16(buf[6:8]))
	lProtoCmd.dwTimeStamp = int32(binary.LittleEndian.Uint32(buf[8:12]))
	lProtoCmd.roomID = int16(binary.LittleEndian.Uint16(buf[12:14]))
	lProtoCmd.size = int32(binary.LittleEndian.Uint32(buf[14:18]))

	messagedata := buf[18:]

	lMsgHead.MMainCmd = lProtoCmd.cmd
	lMsgHead.MSubCmd = lProtoCmd.para
	lMsgHead.MData = messagedata
	logger.Log4.Info("recv-lMsgHead: %v", lMsgHead)

	return &lMsgHead, nil

}
