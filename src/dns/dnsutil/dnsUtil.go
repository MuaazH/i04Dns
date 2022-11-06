package dnsutil

import (
	"bytes"
	"i04Dns/dns/dnsmessage"
)

const MaxDnsMessageSize = 512
const DefaultPort = 53

func IsNameEqual(n0 *dnsmessage.Name, n1 *dnsmessage.Name) bool {
	// some code coupling
	return n0.Length == n1.Length && bytes.Equal(n0.Data[:n0.Length], n1.Data[:n1.Length])
}

func IsQuestionEqual(q0 *dnsmessage.Question, q1 *dnsmessage.Question) bool {
	if q0 == nil {
		return q1 == nil
	}
	if q1 == nil {
		return false
	}
	return q0.Type == q1.Type && IsNameEqual(&q0.Name, &q1.Name)
}

func UnpackMessage(buf []byte) *dnsmessage.Message {
	msg := dnsmessage.Message{}
	err := msg.Unpack(buf)
	if err == nil {
		return &msg
	}
	return nil
}

func NewResponseHeader(id uint16, auth bool, rCode dnsmessage.RCode) *dnsmessage.Header {
	return &dnsmessage.Header{
		ID:                 id,
		Response:           true,
		OpCode:             0,
		Authoritative:      auth,
		Truncated:          false,
		RecursionDesired:   true,
		RecursionAvailable: true, // almost, useless in practice
		RCode:              rCode,
	}
}

func NewQuestionMessage(id uint16, question *dnsmessage.Question) *dnsmessage.Message {
	return &dnsmessage.Message{
		Header: dnsmessage.Header{
			ID:                 id,
			Response:           false,
			OpCode:             0,
			Authoritative:      true,
			Truncated:          false,
			RecursionDesired:   true,
			RecursionAvailable: true,
			RCode:              dnsmessage.RCodeSuccess,
		},
		Questions:   []dnsmessage.Question{*question},
		Answers:     nil,
		Authorities: nil,
		Additionals: nil,
	}
}
