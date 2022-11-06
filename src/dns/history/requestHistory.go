package history

import (
	"i04Dns/dns/dnsmessage"
	"sync"
)

type RequestState int

const (
	StateOpen                  RequestState = 0x0000
	StateClosed                RequestState = 0x0001 // 1
	StateServerError           RequestState = 0x0002 // 2
	StateForwarded             RequestState = 0x0004 // 3
	StateForwardTimeout        RequestState = 0x0008 // 4
	StateAnsweredFromRecursion RequestState = 0x0010 // 5
	StateAnsweredFromDB        RequestState = 0x0020 // 6
	StateAnsweredFromCache     RequestState = 0x0040 // 7
	StateBlockedType           RequestState = 0x0080 // 8
	StateBlockedName           RequestState = 0x0100 // 9
	StateBlockedNameOverridden RequestState = 0x0200 // 10
)

type Entry struct {
	Id            uint16          `json:"id"`
	Client        string          `json:"client"`
	Date          string          `json:"date"`
	Type          dnsmessage.Type `json:"type"`
	Name          string          `json:"name"`
	App           *string         `json:"app"`
	AppliedFilter *string         `json:"appliedFilter"`
	State         RequestState    `json:"action"`
}

type Requests struct {
	first int
	list  []*Entry
	mutex sync.Mutex
}

func New(size int) *Requests {
	return &Requests{
		list:  make([]*Entry, size)[0:0],
		first: 0,
		mutex: sync.Mutex{},
	}
}

func (history *Requests) Add(entry *Entry) {
	history.mutex.Lock()
	limit := cap(history.list)
	size := len(history.list)
	if size < limit {
		history.list = history.list[0 : size+1]
		history.list[size] = entry
	} else {
		history.list[history.first] = entry
		history.first++
		if history.first >= limit {
			history.first = 0
		}
	}
	history.mutex.Unlock()
}

func (history *Requests) at(index int) *Entry {
	i := index + history.first
	capacity := cap(history.list)
	if i >= capacity {
		i -= capacity
	}
	return history.list[i]
}

func (history *Requests) Filter(startDate *string) *[]*Entry {
	history.mutex.Lock()
	size := len(history.list)
	out := make([]*Entry, size)
	count := 0
	for i := 0; i < size; i++ {
		entry := history.at(i)
		if entry.Date < *startDate {
			continue
		}
		out[count] = entry
		count++
	}
	out = out[0:count]
	history.mutex.Unlock()
	return &out
}
