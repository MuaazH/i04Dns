package cache

import (
	"i04Dns/dns/dnsmessage"
	"i04Dns/util"
	"i04Dns/util/bst"
	"strings"
	"sync"
)

//const ModuleName = "Cache"

type CachedResource struct {
	resource   *dnsmessage.Resource
	expiryDate int64
}

func (r *CachedResource) isExpired() bool {
	return r.expiryDate < util.TimeNowSeconds()
}

func (r *CachedResource) toResource() *dnsmessage.Resource {
	ttl := r.expiryDate - util.TimeNowSeconds()
	if ttl < 5 {
		ttl = 5
	}
	r.resource.Header.TTL = uint32(ttl)
	return r.resource
}

// $$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$

type Cache struct {
	tree        *bst.Tree
	nextCleanUp int64
	mutex       sync.Mutex
}

const cacheCheckInterval = 10 // seconds

func NewCache() *Cache {
	return &Cache{
		tree: bst.NewWithStringComparator(),
	}
}

func (cache *Cache) IsEmpty() bool {
	cache.mutex.Lock()
	result := cache.tree == nil || cache.tree.Empty()
	cache.mutex.Unlock()
	return result
}

func (cache *Cache) RemoveExpired() {
	cache.mutex.Lock()
	// filter
	if cache.tree == nil {
		return
	}
	expiredKeys := make([]string, cache.tree.Size())[0:0]
	nodeFilterExpired(cache.tree.Root, &expiredKeys)
	L := len(expiredKeys)
	for i := 0; i < L; i++ {
		cache.tree.Remove(expiredKeys[i])
	}
	cache.nextCleanUp = util.TimeNowSeconds() + cacheCheckInterval
	// end filter
	cache.mutex.Unlock()
}

func (cache *Cache) ReadyForCleanUp() bool {
	cache.mutex.Lock()
	result := cache.nextCleanUp <= util.TimeNowSeconds()
	cache.mutex.Unlock()
	return result
}

func (cache *Cache) AddAll(records *[]dnsmessage.Resource) {
	cache.mutex.Lock()
	l := len(*records)
	for i := 0; i < l; i++ {
		cache.doAdd(&(*records)[i])
	}
	cache.mutex.Unlock()
}

func (cache *Cache) Add(rec *dnsmessage.Resource) {
	cache.mutex.Lock()
	cache.doAdd(rec)
	cache.mutex.Unlock()
}

func (cache *Cache) doAdd(rec *dnsmessage.Resource) {
	if !isTypeCacheable(rec.Header.Type) {
		return
	}
	// add
	if rec.Header.TTL == 0 {
		return
	}
	cRec := CachedResource{
		resource:   rec,
		expiryDate: util.TimeNowSeconds() + int64(rec.Header.TTL),
	}
	nameStr := strings.ToLower(rec.Header.Name.String())
	list, nameFound := cache.tree.Get(nameStr)
	if nameFound {
		recordList := list.(*[]CachedResource)
		listLen := len(*recordList)
		cloneFound := false
		for i := 0; i < listLen; i++ {
			old := (*recordList)[i].resource
			if dnsmessage.IsBodyEqual(old, rec) {
				cloneFound = true
				(*recordList)[i].expiryDate = int64(rec.Header.TTL) + util.TimeNowSeconds()
				break
			}
		}
		if !cloneFound {
			*recordList = append(*recordList, cRec)
		}
	} else {
		newList := []CachedResource{cRec}
		cache.tree.Put(nameStr, &newList)
	}
	// end add
}

func (cache *Cache) Get(q *dnsmessage.Question) *[]dnsmessage.Resource {
	cache.mutex.Lock()
	list, typeFound := getRecord(cache.tree, q.Class, q.Type, strings.ToLower(q.Name.String()), 0)
	cache.mutex.Unlock()
	if typeFound {
		return list
	}
	return nil
}

func (cache *Cache) Clear() {
	cache.mutex.Lock()
	cache.tree.Clear()
	cache.mutex.Unlock()
}

func getRecord(tree *bst.Tree, class dnsmessage.Class, typ dnsmessage.Type, name string, depth int) (*[]dnsmessage.Resource, bool) {
	// don't go too deep
	if depth >= 3 {
		return nil, false
	}
	list, found := tree.Get(name)
	typeFound := false
	if found {
		recordList := *list.(*[]CachedResource)
		const maxCount = 32
		result := [maxCount]dnsmessage.Resource{}
		rCount := 0
		count := len(recordList)
		for i := 0; i < count; i++ {
			rec := recordList[i]
			// if a cname is found, search recursively
			if typ != dnsmessage.TypeCNAME && rec.resource.Header.Type == dnsmessage.TypeCNAME {
				cname := rec.resource.Body.(*dnsmessage.CNAMEResource)
				eq, tf := getRecord(tree, class, typ, cname.CNAME.String(), depth+1)
				if tf {
					typeFound = true
				}
				if eq != nil {
					eqLen := len(*eq)
					for j := 0; j < eqLen; j++ {
						result[rCount] = (*eq)[j]
						rCount++
						if rCount > maxCount {
							break
						}
					}
				}
			}
			// copy matching records
			if rec.resource.Header.Type == typ && rec.resource.Header.Class == class && !rec.isExpired() {
				typeFound = true
				result[rCount] = *rec.toResource()
				rCount++
				if rCount > maxCount {
					break
				}
			}
		}
		if rCount > 0 && typeFound {
			resultSlice := result[0:rCount]
			return &resultSlice, true
		}
	}
	return nil, false
}

func listFilterExpired(list *[]CachedResource) {
	length := len(*list)
	now := util.TimeNowSeconds()
	for i := 0; i < length; i++ {
		rec := (*list)[i]
		if rec.expiryDate <= now {
			//expiredHeader := (*list)[i].resource.Header
			//util.LogDebug(ModuleName, fmt.Sprintf("Expired record %d %d %s", int(expiredHeader.Class), expiredHeader.Type, expiredHeader.Name.String()))
			length--
			(*list)[i] = (*list)[length]
			*list = (*list)[:length]
			i--
		}
	}
}

func nodeFilterExpired(node *bst.Node, expiredKeys *[]string) {
	if node == nil {
		return
	}
	nodeFilterExpired(node.Left, expiredKeys)
	// filter node
	records := node.Value.(*[]CachedResource)
	if records != nil { // should never happen...
		listFilterExpired(records)
	}
	if records == nil || len(*records) == 0 {
		idx := len(*expiredKeys)
		*expiredKeys = (*expiredKeys)[0 : idx+1]
		(*expiredKeys)[idx] = node.Key.(string)
	}
	// end filter node
	nodeFilterExpired(node.Right, expiredKeys)
}

func isTypeCacheable(typ dnsmessage.Type) bool {
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
