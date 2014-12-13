package zlib

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

type ModbusEvent struct {
	Function FunctionCode
	Response []byte
}

var ModbusEventType = EventType{
	TypeName:         CONNECTION_EVENT_MODBUS,
	GetEmptyInstance: func() EventData { return new(ModbusEvent) },
}

func (m *ModbusEvent) GetType() EventType {
	return ModbusEventType
}

type encodedModbusEvent struct {
	Function FunctionCode `json:"function_code"`
	Response []byte       `json:"response"`
}

func (m *ModbusEvent) MarshalJSON() ([]byte, error) {
	e := encodedModbusEvent{
		Function: m.Function,
		Response: m.Response,
	}
	return json.Marshal(&e)
}

func (m *ModbusEvent) UnmarshalJSON(b []byte) error {
	e := new(encodedModbusEvent)
	if err := json.Unmarshal(b, e); err != nil {
		return err
	}
	m.Function = e.Function
	m.Response = e.Response
	return nil
}

type FunctionCode byte
type ExceptionFunctionCode byte
type ExceptionCode byte

type ModbusRequest struct {
	Function FunctionCode
	Data     []byte
}

func (r *ModbusRequest) MarshalBinary() (data []byte, err error) {
	data = make([]byte, 7+1+len(r.Data))
	copy(data[0:4], ModbusHeaderBytes)
	msglen := len(r.Data) + 2 // unit ID and function
	binary.BigEndian.PutUint16(data[4:6], uint16(msglen))

	data[7] = byte(r.Function)
	copy(data[8:], r.Data)

	return
}

type ModbusResponse struct {
	Function FunctionCode
	Data     []byte
}

func (c *Conn) ReadMin(res []byte, bytes int) (cnt int, err error) {
	for cnt < bytes {
		var n int
		n, err = c.getUnderlyingConn().Read(res[cnt:])
		cnt += n

		if err != nil && cnt >= len(res) {
			err = fmt.Errorf("modbus: response buffer too small")
		}

		if err != nil {
			return
		}
	}

	return
}

func (c *Conn) GetModbusResponse() (res ModbusResponse, err error) {
	var cnt int
	buf := make([]byte, 1024) // should be more memory than we need

	cnt, err = c.ReadMin(buf, 6)
	if err != nil {
		err = fmt.Errorf("modbus: could not get response: %e", err)
		return
	}

	// first 4 bytes should be known, verify them
	if !bytes.Equal(buf[0:4], ModbusHeaderBytes) {
		err = fmt.Errorf("modbus: not a modbus response")
		return
	}

	msglen := int(binary.BigEndian.Uint16(buf[4:6]))

	for cnt < msglen+6 {
		var n int
		n, err = c.getUnderlyingConn().Read(buf[cnt:])
		cnt += n

		if err != nil && cnt >= len(buf) {
			err = fmt.Errorf("modbus: resporse buffer too small")
		}

		if err != nil {
			return
		}
	}

	//TODO this really should be done by a more elegant unmarshaling function
	res = ModbusResponse{
		Function: FunctionCode(buf[7]),
		Data:     buf[8:],
	}

	return
}

type ModbusException struct {
	Function      ExceptionFunctionCode
	ExceptionType ExceptionCode
}

func (e ExceptionFunctionCode) FunctionCode() FunctionCode {
	code := byte(e) & byte(0x79)
	return FunctionCode(code)
}

func (c FunctionCode) ExceptionFunctionCode() ExceptionFunctionCode {
	code := byte(c) | byte(0x80)
	return ExceptionFunctionCode(code)
}

func (c FunctionCode) IsException() bool {
	return (byte(c) & 0x80) == 0x80
}

var ModbusHeaderBytes = []byte{
	0x13, 0x37, // do not matter, will just be verifying they are the same
	0x00, 0x00, // must be 0
}

var ModbusFunctionEncapsulatedInterface = FunctionCode(0x2B)
