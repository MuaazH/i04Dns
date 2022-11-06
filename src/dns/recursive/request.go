package recursive

import (
	"i04Dns/dns/dnsmessage"
	"i04Dns/dns/history"
	"i04Dns/util"
	"i04Dns/util/bst"
	"net"
	"sync"
)

const (
	// For some reason, i don't want to use small ids. This is not protocol. I'm just playing around.
	minId = 0x0100
	maxId = 0xFFFF

	MaxForwardDelay = 8 * 1000 // Milliseconds
)

type ForwardedRequest struct {
	OriginalId uint16
	Question   *dnsmessage.Question
	Client     *net.UDPAddr
	Server     *net.UDPAddr
	History    *history.Entry
	StartDate  int64
	ExpireDate int64
}

type ForwardedRequests struct {
	nextId uint16
	tree   *bst.Tree
	mutex  sync.Mutex
}

func NewForwardedRequests() *ForwardedRequests {
	return &ForwardedRequests{
		nextId: minId,
		tree:   bst.NewWithUInt16Comparator(),
	}
}

func (forwardedRequests *ForwardedRequests) Add(req *ForwardedRequest) uint16 {
	if req == nil {
		panic("Illegal argument")
	}
	forwardedRequests.mutex.Lock()
	//
	if forwardedRequests.nextId >= maxId {
		forwardedRequests.nextId = minId
	}
	forwardedRequests.nextId++
	id := forwardedRequests.nextId

	req.StartDate = util.TimeNowMilliseconds()
	req.ExpireDate = util.TimeNowMilliseconds() + MaxForwardDelay
	forwardedRequests.tree.Put(id, req)
	//
	forwardedRequests.mutex.Unlock()
	return id
}

func (forwardedRequests *ForwardedRequests) Get(forwardId uint16) *ForwardedRequest {
	forwardedRequests.mutex.Lock()
	request, found := forwardedRequests.tree.Get(forwardId)
	forwardedRequests.mutex.Unlock()
	if found {
		return request.(*ForwardedRequest)
	}
	return nil
}

func (forwardedRequests *ForwardedRequests) Remove(forwardId uint16) {
	forwardedRequests.mutex.Lock()
	forwardedRequests.tree.Remove(forwardId)
	forwardedRequests.mutex.Unlock()
}

func (forwardedRequests *ForwardedRequests) RemoveExpired() *[]*ForwardedRequest {
	forwardedRequests.mutex.Lock()
	// collect expired
	expiredIds := make([]uint16, forwardedRequests.tree.Size())[0:0]
	expired := make([]*ForwardedRequest, forwardedRequests.tree.Size())[0:0]
	filterTree(forwardedRequests.tree.Root, &expiredIds, &expired)
	// remove expired
	L := len(expiredIds)
	for i := 0; i < L; i++ {
		forwardedRequests.tree.Remove(expiredIds[i])
	}
	// unlock
	forwardedRequests.mutex.Unlock()
	return &expired
}

func (forwardedRequests *ForwardedRequests) Clear() {
	forwardedRequests.mutex.Lock()
	forwardedRequests.tree.Clear()
	forwardedRequests.mutex.Unlock()
}

func filterTree(node *bst.Node, expiredIds *[]uint16, expired *[]*ForwardedRequest) {
	if node == nil {
		return
	}
	filterTree(node.Left, expiredIds, expired)
	req := node.Value.(*ForwardedRequest)
	eTime := util.TimeNowMilliseconds()
	if req.ExpireDate < eTime {
		length := len(*expiredIds)
		*expiredIds = (*expiredIds)[0 : length+1]
		*expired = (*expired)[0 : length+1]
		(*expiredIds)[length] = node.Key.(uint16)
		(*expired)[length] = req
	}
	filterTree(node.Right, expiredIds, expired)
}
