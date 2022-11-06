package dns

import (
	"errors"
	"fmt"
	"i04Dns/common"
	"i04Dns/config"
	"i04Dns/dns/cache"
	"i04Dns/dns/dnsmessage"
	"i04Dns/dns/dnsutil"
	"i04Dns/dns/history"
	"i04Dns/dns/recursive"
	"i04Dns/util"
	"net"
	"sync"
	"time"
)

const ModuleName = "Dns-Server"

type Server struct {
	mutex          sync.Mutex
	runningState   *util.RunningState
	socket         *net.UDPConn
	ctx            *common.AppContext
	Cache          *cache.Cache
	udpServers     *recursive.UdpRecursiveServers
	RequestHistory *history.Requests
}

type message struct {
	msg    *dnsmessage.Message
	sender *net.UDPAddr
}

func New() *Server {
	return &Server{
		runningState:   util.NewRunningState(),
		Cache:          cache.NewCache(),
		udpServers:     recursive.NewUdpRecursiveServers(),
		RequestHistory: history.New(1024 * 4),
	}
}

func (srv *Server) filterResources(records *[]dnsmessage.Resource, client *net.UDPAddr) *[]dnsmessage.Resource {
	if records == nil {
		return nil
	}
	count := len(*records)
	if count == 0 {
		return records
	}

	app := util.GetProcessName(uint32(client.Port), true, util.IsIp4(client.IP))
	appRule := srv.ctx.GetConfig().AppRules.FindName(app)

	result := make([]dnsmessage.Resource, len(*records))
	valid := 0
	for i := 0; i < count; i++ {
		resource := (*records)[i]
		if resource.Body == nil || !isTypeSupported(resource.Header.Type) {
			continue
		}
		name := resource.Header.Name.String()
		allowed, _ := srv.IsNameAllowed(&name, appRule)
		if !allowed {
			continue
		}
		result[valid] = resource
		valid++
	}
	result = result[:valid]
	return &result
}

func (srv *Server) processResponse(msg *dnsmessage.Message, sender *net.UDPAddr) {
	util.LogInfo(ModuleName, fmt.Sprintf("Response %d form %s", msg.ID, sender.String()))
	_, rSrv := srv.udpServers.GetByAddress(sender)
	if rSrv != nil {
		req := rSrv.Queue.Get(msg.ID)
		if req != nil && dnsutil.IsQuestionEqual(req.Question, &msg.Questions[0]) {
			rSrv.Queue.Remove(msg.ID)
			rSrv.IncReplyCount(int32(util.TimeNowMilliseconds() - req.StartDate))
			// cache result
			srv.Cache.AddAll(&msg.Answers)
			srv.Cache.AddAll(&msg.Authorities)
			srv.Cache.AddAll(&msg.Additionals)
			req.History.State |= history.StateClosed | history.StateAnsweredFromRecursion
			srv.sendResponse(dnsutil.NewResponseHeader(req.OriginalId, false, dnsmessage.RCodeSuccess), req.Question, &msg.Answers, &msg.Authorities, &msg.Additionals, req.Client)
		} else {
			// unknown message, could be a dumb attack
		}
	}
}

func (srv *Server) sendResponse(head *dnsmessage.Header, question *dnsmessage.Question, answer *[]dnsmessage.Resource, authorities *[]dnsmessage.Resource, additional *[]dnsmessage.Resource, client *net.UDPAddr) {
	//fmt.Printf("Sending response to %s\n", client.String())

	var rr []dnsmessage.Resource = nil
	var ar []dnsmessage.Resource = nil
	var ad []dnsmessage.Resource = nil
	if answer != nil {
		rr = *srv.filterResources(answer, client)
	}
	if authorities != nil {
		ar = *srv.filterResources(authorities, client)
	}
	if additional != nil {
		ad = *srv.filterResources(additional, client)
	}
	msg := dnsmessage.Message{
		Header:      *head,
		Questions:   []dnsmessage.Question{*question},
		Answers:     rr,
		Authorities: ar,
		Additionals: ad,
	}
	buf, err := msg.Pack()
	if err == nil {
		_, _ = srv.socket.WriteToUDP(buf, client)
	} else {
		// don't send shit back to the client because this error should be more rare than a trustworthy man
		util.LogError(ModuleName, err.Error())
		panic(err)
	}
}

func (srv *Server) forwardUdp(req *recursive.ForwardedRequest) (*recursive.UdpRecursiveServer, error) {
	recSrv := srv.udpServers.SelectBest()
	if recSrv == nil {
		return nil, errors.New("no udp servers to forward to")
	}
	recSrv.IncQueryCount()
	msgId := recSrv.Queue.Add(req)
	req.Server = recSrv.Address
	msg := dnsutil.NewQuestionMessage(msgId, req.Question)
	buf, err := msg.Pack()
	_, err = srv.socket.WriteToUDP(buf, recSrv.Address)
	return recSrv, err // when was the last time you saw a udp fail to send?
}

func (srv *Server) IsNameAllowed(name *string, appRule *config.CompiledAppRule) (bool, *string) {
	whiteFilters, blackFilters := srv.ctx.GetConfig().GetNameFilters()

	var white *string
	var black *string
	if appRule != nil {
		black = appRule.Blacklist.FindFilter(name)
		if black != nil {
			white = appRule.Whitelist.FindFilter(name)
			if white != nil {
				return true, white
			}
			return false, black
		}
	}

	black = blackFilters.FindFilter(name)
	if black != nil {
		if appRule != nil {
			white = appRule.Whitelist.FindFilter(name)
		}
		if white != nil {
			return true, white
		}
		white = whiteFilters.FindFilter(name)
		if white != nil {
			return true, white
		}
		return false, black
	}
	return true, nil
}

func (srv *Server) processRequest(msg *dnsmessage.Message, client *net.UDPAddr) {
	qCount := len(msg.Questions)
	if qCount == 0 {
		util.LogDebug(ModuleName, fmt.Sprintf("Empty request from %s, ", client.String()))
		return // a pointless message
	}
	question := msg.Questions[0] // all other queries are ignored
	questionName := question.Name.String()

	historyEntry := history.Entry{
		Id:     msg.Header.ID,
		Client: client.String(),
		Date:   util.TimeNowString(),
		Type:   question.Type,
		Name:   questionName,
		App:    nil,
		State:  history.StateOpen,
	}
	srv.RequestHistory.Add(&historyEntry)

	if !isTypeSupported(question.Type) {
		// send empty message to shut the client up
		historyEntry.State |= history.StateClosed | history.StateBlockedType
		srv.sendResponse(dnsutil.NewResponseHeader(msg.Header.ID, true, dnsmessage.RCodeSuccess), &question, nil, nil, nil, client)
		return
	}

	// check local database first
	db := srv.ctx.GetConfig().LocalDatabase
	if db != nil {
		answer, additional := db.LookupAnswer(&question)
		if answer != nil {
			// todo: if the address is loop-back ip, replace it with the ip of the server interface that received this query
			historyEntry.State |= history.StateClosed | history.StateAnsweredFromDB
			srv.sendResponse(dnsutil.NewResponseHeader(msg.Header.ID, true, dnsmessage.RCodeSuccess), &question, answer, answer, additional, client)
			return
		}
	}

	var appRule *config.CompiledAppRule

	if client.IP.IsLoopback() {
		notFound := "*"
		app := util.GetProcessName(uint32(client.Port), true, util.IsIp4(client.IP))
		if app == nil {
			app = &notFound
		}
		appRule = srv.ctx.GetConfig().AppRules.FindName(app)
	}

	// check if the host is whitelisted or blacklisted
	allowed, filter := srv.IsNameAllowed(&questionName, appRule)
	historyEntry.AppliedFilter = filter

	if allowed {
		if filter != nil {
			//util.LogInfo(ModuleName, fmt.Sprintf("Bypassing block for name %s with rule %s", questionName, *filter))
			historyEntry.State |= history.StateBlockedNameOverridden
		}
	} else {
		historyEntry.State |= history.StateClosed | history.StateBlockedName
		//if filter != nil {
		//util.LogInfo(ModuleName, fmt.Sprintf("Blocking name %s with rule '%s'", questionName, *filter))
		//}
		// send empty message to shut the client up
		srv.sendResponse(dnsutil.NewResponseHeader(msg.Header.ID, true, dnsmessage.RCodeSuccess), &question, nil, nil, nil, client)
		return
	}

	// check cache
	cachedResult := srv.Cache.Get(&question)
	if cachedResult != nil {
		historyEntry.State |= history.StateClosed | history.StateAnsweredFromCache
		util.LogInfo(ModuleName, fmt.Sprintf("Answering request %d from local cache", int(msg.Header.ID)))
		srv.sendResponse(dnsutil.NewResponseHeader(msg.Header.ID, true, dnsmessage.RCodeSuccess), &question, cachedResult, nil, nil, client)
		return
	}

	recursiveSrv, err := srv.forwardUdp(&recursive.ForwardedRequest{
		OriginalId: msg.Header.ID,
		Question:   &msg.Questions[0],
		Client:     client,
		History:    &historyEntry,
	})
	if err == nil {
		historyEntry.State |= history.StateForwarded
		util.LogInfo(ModuleName, fmt.Sprintf("Forwarding query %d to %s", msg.Header.ID, recursiveSrv.Address.String()))
	} else {
		historyEntry.State |= history.StateClosed | history.StateServerError
		util.LogError(ModuleName, err.Error())
	}
}

func (srv *Server) openPort() bool {
	// open port
	dnsConf := srv.ctx.GetConfig().Dns
	host := dnsConf.Host
	network := dnsConf.Network
	if host == nil || network == nil {
		util.LogInfo(ModuleName, "Config invalid")
		time.Sleep(1000 * time.Millisecond)
		return false
	}
	util.LogInfo(ModuleName, "Opening port")
	s, err := net.ListenUDP(*network, &net.UDPAddr{
		Port: dnsutil.DefaultPort,
		IP:   net.ParseIP(*host),
	})
	if err != nil {
		util.LogError(ModuleName, "Opening port... ERROR")
		time.Sleep(1000 * time.Millisecond)
		return false
	}
	srv.socket = s
	util.LogInfo(ModuleName, "Opening port... OK")
	return true
}

func (srv *Server) receiveUdpMessages(msgInput *chan *message) {
	buffer := make([]byte, dnsutil.MaxDnsMessageSize*2)
	socket := srv.socket
	for srv.runningState.IsOn() {
		err := socket.SetReadDeadline(time.Now().Add(time.Second))
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		bufLen, sender, err := socket.ReadFromUDP(buffer)
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		//
		if !srv.runningState.IsOn() {
			break
		}
		//
		msg := dnsutil.UnpackMessage(buffer[:bufLen])
		if msg != nil {
			*msgInput <- &message{msg, sender}
		}
		// ignore bad messages
	}
}

func (srv *Server) loop(channel *chan *message) {
	msgInput := *channel
	for srv.runningState.IsOn() {
		//
		if len(msgInput) > 0 {
			msg := <-msgInput
			if msg.msg.Header.Response {
				srv.processResponse(msg.msg, msg.sender)
			} else {
				srv.processRequest(msg.msg, msg.sender)
			}
		} else {
			if srv.Cache.ReadyForCleanUp() {
				util.LogDebug(ModuleName, "Cleaning up cache")
				srv.Cache.RemoveExpired()
			}
			if srv.udpServers.ReadyForCleanUp() {
				for i := 0; i < recursive.MaxUDPServers; i++ {
					expired := srv.udpServers.RemoveExpiredFromQueues(i)
					if expired == nil {
						continue
					}
					expiredLen := len(*expired)
					for j := 0; j < expiredLen; j++ {
						ex := (*expired)[j]
						ex.History.State |= history.StateClosed | history.StateForwardTimeout
						util.LogInfo(ModuleName, fmt.Sprintf("Forwarded request %d to %s timed-out", ex.OriginalId, ex.Server.String()))
					}
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (srv *Server) Run(ctx *common.AppContext) {
	if srv.runningState.IsOn() || !ctx.GetConfig().Dns.Enabled {
		return
	}

	// keep track of the context
	srv.ctx = ctx

	if !srv.runningState.SetOn() {
		// already running
		return
	}

	// Clean shit up on exit
	defer func() {
		srv.ctx = nil
		if srv.socket != nil {
			_ = srv.socket.Close()
			srv.socket = nil
		}
		srv.runningState.SignalShutdownComplete()
	}()

	// open port
	if !srv.openPort() {
		return
	}

	srv.OnConfigUpdate()

	// handle incoming messages
	msgInput := make(chan *message, 128)

	// simple DNS server on Port 53
	go srv.receiveUdpMessages(&msgInput)

	// main loop
	srv.loop(&msgInput)
}

func (srv *Server) reset() {
	srv.Cache.Clear()
	srv.udpServers.Clear()
}

func (srv *Server) Stop() {
	srv.runningState.SetOff()
	srv.reset()
}

func (srv *Server) OnConfigUpdate() {
	if !srv.runningState.IsOn() {
		return
	}
	srv.reset()
	// fixme: don't ignore this error
	_ = srv.udpServers.Setup(srv.ctx.GetConfig())
}

func isTypeSupported(typ dnsmessage.Type) bool {
	switch typ {
	case dnsmessage.TypeA:
		return true
	case dnsmessage.TypeAAAA:
		return true
	case dnsmessage.TypeCNAME:
		return true
	case dnsmessage.TypeNS:
		return true
	case dnsmessage.TypeMX:
		return true
	case dnsmessage.TypePTR:
		return true
	case dnsmessage.TypeSOA:
		return true
	case dnsmessage.TypeSRV:
		return true
	default:
		return false
	}
}
