package recursive

import (
	"errors"
	"fmt"
	"i04Dns/config"
	"i04Dns/util"
	"math/rand"
	"net"
	"sync"
)

const MaxUDPServers = 16

type UdpRecursiveServer struct {
	Queue        *ForwardedRequests
	Address      *net.UDPAddr
	responseTime int32
	queriesCount int32
	repliesCount int32
}

func (srv *UdpRecursiveServer) IncQueryCount() {
	srv.queriesCount++
}

func (srv *UdpRecursiveServer) IncReplyCount(delayMS int32) {
	if srv.repliesCount < 1 {
		srv.responseTime = delayMS
	} else {
		srv.responseTime = int32(float32(srv.responseTime)*0.85 + float32(delayMS)*0.15)
	}
	srv.repliesCount++
}

// SpeedScore returns int from 0 to 999
func (srv *UdpRecursiveServer) SpeedScore() int {
	// todo: factor in ping
	if srv.queriesCount == 0 {
		return 400 // untested server
	}
	score1 := 1000 - int(srv.responseTime)
	score2 := int(srv.repliesCount * 1000 / srv.queriesCount)
	score := (score1 * score2) >> 2
	if score > 999 {
		score = 999
	}
	return score
}

type UdpRecursiveServers struct {
	mutex       sync.Mutex
	list        [MaxUDPServers]*UdpRecursiveServer
	count       int
	nextCleanUp int64
}

func NewUdpRecursiveServers() *UdpRecursiveServers {
	return &UdpRecursiveServers{
		mutex: sync.Mutex{},
		list:  [MaxUDPServers]*UdpRecursiveServer{},
		count: 0,
	}
}

func (servers *UdpRecursiveServers) add(address *net.UDPAddr) error {
	var err error = nil
	if servers.count >= MaxUDPServers {
		err = errors.New(fmt.Sprintf("too many servers (max is %d)", MaxUDPServers))
	} else {
		id := servers.count
		servers.list[id] = &UdpRecursiveServer{
			Queue:   NewForwardedRequests(),
			Address: address,
		}
		servers.count++
	}
	return err
}

func (servers *UdpRecursiveServers) GetByAddress(a *net.UDPAddr) (int, *UdpRecursiveServer) {
	servers.mutex.Lock()
	idx := -1
	var srv *UdpRecursiveServer = nil
	for i := 0; i < servers.count; i++ {
		if servers.list[i].Address.IP.Equal(a.IP) && servers.list[i].Address.Port == a.Port {
			idx = i
			srv = servers.list[i]
			break
		}
	}
	servers.mutex.Unlock()
	return idx, srv
}

func (servers *UdpRecursiveServers) GetCount() int {
	servers.mutex.Lock()
	count := servers.count
	servers.mutex.Unlock()
	return count
}

var rnd *rand.Rand

func (servers *UdpRecursiveServers) SelectBest() *UdpRecursiveServer {
	var bestServer *UdpRecursiveServer = nil
	bestScore := -1

	if rnd == nil {
		rnd = rand.New(rand.NewSource(util.TimeNowMilliseconds()))
	}

	if servers.count > 0 {
		if rnd.Int31n(100) < 10 { // 10% chance
			return servers.list[rnd.Intn(servers.count)] // return a random server to give screw ups a chance
		}
	}

	servers.mutex.Lock()
	for i := 0; i < servers.count; i++ {
		score := servers.list[i].SpeedScore()
		if score > bestScore {
			bestServer = servers.list[i]
			bestScore = score
		}
	}
	servers.mutex.Unlock()
	return bestServer
}

func (servers *UdpRecursiveServers) Clear() {
	servers.mutex.Lock()
	for i := 0; i < MaxUDPServers; i++ {
		servers.list[i] = nil
	}
	servers.mutex.Unlock()
}

func (servers *UdpRecursiveServers) ReadyForCleanUp() bool {
	servers.mutex.Lock()
	result := servers.nextCleanUp <= util.TimeNowSeconds()
	servers.mutex.Unlock()
	return result
}

func (servers *UdpRecursiveServers) RemoveExpiredFromQueues(queue int) *[]*ForwardedRequest {
	servers.mutex.Lock()
	var result *[]*ForwardedRequest
	if queue < servers.count && servers.list[queue] != nil && servers.list[queue].Queue != nil {
		result = servers.list[queue].Queue.RemoveExpired()
		servers.nextCleanUp = util.TimeNowSeconds() + 2 // 2 seconds
	}
	servers.mutex.Unlock()
	return result
}

func (servers *UdpRecursiveServers) Setup(config *config.Configuration) error {
	servers.mutex.Lock()
	// clear
	for i := 0; i < MaxUDPServers; i++ {
		servers.list[i] = nil
	}
	dnsServers := config.DnsServers
	for i := 0; i < len(*dnsServers); i++ {
		err := servers.add((*dnsServers)[i])
		if err != nil {
			return err
		}
	}
	servers.mutex.Unlock()
	return nil
}
