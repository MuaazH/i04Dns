package dns

import (
	"fmt"
	"i04Dns/config"
	"i04Dns/dns/cache"
	"i04Dns/dns/dnsmessage"
	"i04Dns/txt"
	"i04Dns/util"
	"net"
	"strconv"
	"time"
)

const maxDnsMessageSize = 512
const defaultPort = 53

var defaultPortStr = "53"

type ServerContext struct {
	Cache  *cache.Cache
	socket *net.UDPConn
}

type message struct {
	msg    *dnsmessage.Message
	sender *net.UDPAddr
}

func unpack(buf []byte) *dnsmessage.Message {
	msg := dnsmessage.Message{}
	err := msg.Unpack(buf)
	if err == nil {
		return &msg
	}
	return nil
}

func processResponse(ctx *ServerContext, msg *dnsmessage.Message, sender *net.UDPAddr) {
	fmt.Printf("Response %d from %s\n", msg.Header.ID, sender.String())
}

func newResponseHeader(id uint16, auth bool, rCode dnsmessage.RCode) *dnsmessage.Header {
	return &dnsmessage.Header{
		ID:                 id,
		Response:           true,
		OpCode:             0,
		Authoritative:      auth,
		Truncated:          false,
		RecursionDesired:   true,
		RecursionAvailable: true,
		RCode:              rCode,
	}
}

func sendResponse(ctx *ServerContext, head *dnsmessage.Header, query *dnsmessage.Question, answer *[]dnsmessage.Resource, additional *[]dnsmessage.Resource, client *net.UDPAddr) {
	var rr []dnsmessage.Resource = nil
	var ar []dnsmessage.Resource = nil
	var ad []dnsmessage.Resource = nil
	if answer != nil {
		if head.Authoritative {
			ar = *answer
		} else {
			rr = *answer
		}
	}
	if additional != nil {
		ad = *additional
	}
	msg := dnsmessage.Message{
		Header:      *head,
		Questions:   []dnsmessage.Question{*query},
		Answers:     rr,
		Authorities: ar,
		Additionals: ad,
	}
	buf, err := msg.Pack()
	if err == nil {
		_, _ = ctx.socket.WriteToUDP(buf, client)
	} else {
		// todo: log error
		// don't send shit back to the client because this error should be more rare than a trustworthy man
		fmt.Println(err.Error())
	}
}

func processRequest(ctx *ServerContext, msg *dnsmessage.Message, sender *net.UDPAddr) {
	qCount := len(msg.Questions)
	if qCount == 0 {
		util.LogDebug(util.DnsModule, txt.DnsSrvEmptyRequest, []string{txt.DnsSrvClient}, []string{sender.String()})
		return // a pointless message
	}
	query := msg.Questions[0] // all other queries are ignored
	queryName := query.Name.String()
	util.LogInfo(util.DnsModule, txt.DnsSrvQuestion, []string{txt.DnsSrvId, txt.DnsSrvClient, txt.DnsSrvClass, txt.DnsSrvType, txt.DnsSrvName}, []string{strconv.Itoa(int(msg.Header.ID)), sender.String(), query.Class.String(), query.Type.String(), queryName})
	db := config.GetDB()
	if db != nil {
		answer, additional := db.LookupAnswer(&query)
		if answer != nil {
			// todo: log what the answer was in detail
			util.LogInfo(util.DnsModule, txt.DnsSrvAnswerFromDB, []string{txt.DnsSrvId, txt.DnsSrvClient}, []string{strconv.Itoa(int(msg.Header.ID)), sender.String()})
			sendResponse(ctx, newResponseHeader(msg.Header.ID, true, dnsmessage.RCodeSuccess), &query, answer, additional, sender)
			return
		}
	}
	white, black := config.GetNameFilters()
	// todo: check if the host is whitelisted or blacklisted
	blackFilter := black.FindFilter(queryName)
	if blackFilter != nil {
		whiteFilter := white.FindFilter(queryName)
		if whiteFilter == nil {
			util.LogInfo(util.DnsModule, txt.DnsSrvNameBlocked, []string{txt.DnsSrvName, txt.DnsSrvNameFilter}, []string{queryName, *blackFilter})
			sendResponse(ctx, newResponseHeader(msg.Header.ID, true, dnsmessage.RCodeSuccess), &query, nil, nil, sender)
			return // name is blacklisted, go to hell
		} else {
			util.LogInfo(util.DnsModule, txt.DnsSrvNameBlockOverride, []string{txt.DnsSrvName, txt.DnsSrvNameFilter}, []string{queryName, *whiteFilter})
		}
	}
	// todo: filter name
	// todo: check cache
	// todo: forward message & store request in cache
}

func Run() {
	context := ServerContext{
		Cache:  cache.NewCache(),
		socket: nil,
	}
	util.LogInfo(util.DnsModule, txt.OpenPort, []string{txt.Port}, []string{defaultPortStr})
	dnsConf := config.GetDnsConf()
	host := dnsConf.Host
	network := dnsConf.Network
	for {
		if host == nil || network == nil {
			util.LogInfo(util.DnsModule, txt.DnsSrvConfInvalid, []string{}, []string{})
			time.Sleep(5000 * time.Millisecond)
			continue
		}
		s, err := net.ListenUDP(*network, &net.UDPAddr{
			Port: defaultPort,
			IP:   net.ParseIP(*host),
		})
		if err != nil {
			util.LogInfo(util.DnsModule, txt.OpenPortFail, []string{txt.Port}, []string{defaultPortStr})
			time.Sleep(5000 * time.Millisecond)
			continue
		}
		context.socket = s
		break
	}
	defer func(socket *net.UDPConn) {
		_ = socket.Close()
	}(context.socket)
	util.LogInfo(util.DnsModule, txt.OpenPortOk, []string{txt.Port}, []string{defaultPortStr})
	msgInput := make(chan *message, 128)
	// simple DNS server on Port 53
	go func() {
		buffer := make([]byte, maxDnsMessageSize*2)
		for {
			bufLen, sender, err := context.socket.ReadFromUDP(buffer)
			if err != nil {
				time.Sleep(10 * time.Millisecond)
				// this is UDP, this error is very rare
				fmt.Println(err)
				continue
			}
			msg := unpack(buffer[:bufLen])
			if msg != nil {
				msgInput <- &message{msg, sender}
			}
			// ignore bad messages
		}
	}()

	// main loop
	for {
		if len(msgInput) > 0 {
			msg := <-msgInput
			if msg.msg.Header.Response {
				processResponse(&context, msg.msg, msg.sender)
			} else {
				processRequest(&context, msg.msg, msg.sender)
			}
		} else {
			if context.Cache.WaitingCleanUp() {
				util.LogDebug(util.DnsModule, txt.DnsSrvClearingCache, []string{}, []string{})
				context.Cache.FilterExpired()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
