package cache

import (
	"i04Dns/dns/dnsmessage"
	"i04Dns/txt"
	"i04Dns/util"
	"i04Dns/util/bst"
	"strconv"
	"strings"
)

type CachedResource struct {
	resource   *dnsmessage.Resource
	expiryDate int64
}

func (r *CachedResource) isExpired() bool {
	return r.expiryDate >= util.TimeNow()
}

func (r *CachedResource) toResource() *dnsmessage.Resource {
	ttl := r.expiryDate - util.TimeNow()
	if ttl < 0 {
		ttl = 0
	}
	r.resource.Header.TTL = uint32(ttl)
	return r.resource
}

// $$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$

type Cache struct {
	tree        *bst.Tree
	nextCleanUp int64
}

const cacheCheckInterval = 10 // seconds

func NewCache() *Cache {
	return &Cache{
		tree: bst.NewWithStringComparator(),
	}
}

func (cache *Cache) IsEmpty() bool {
	return cache.tree == nil || cache.tree.Empty()
}

func (cache *Cache) FilterExpired() {
	if cache.tree == nil {
		return
	}
	expiredKeys := make([]string, cache.tree.Size())[0:0]
	nodeFilterExpired(cache.tree.Root, &expiredKeys)
	L := len(expiredKeys)
	for i := 0; i < L; i++ {
		cache.tree.Remove(expiredKeys[i])
	}
	cache.nextCleanUp = util.TimeNow() + cacheCheckInterval
}

func (cache *Cache) WaitingCleanUp() bool {
	return cache.nextCleanUp <= util.TimeNow()
}

// Add Caches shit. Use carefully, Can add duplicates
func (cache *Cache) Add(rec *dnsmessage.Resource) {
	if rec.Header.TTL == 0 {
		return
	}
	cRec := CachedResource{
		resource:   rec,
		expiryDate: util.TimeNow() + int64(rec.Header.TTL),
	}
	nameStr := strings.ToLower(rec.Header.Name.String())
	list, found := cache.tree.Get(nameStr)
	if found {
		recordList := list.(*[]CachedResource)
		*recordList = append(*recordList, cRec)
		// todo: prevent adding duplicate records
	} else {
		newList := []CachedResource{cRec}
		cache.tree.Put(nameStr, &newList)
	}
}

func (cache *Cache) Get(q *dnsmessage.Question) *[]dnsmessage.Resource {
	list, typeFound := getRecord(cache.tree, q.Class, q.Type, strings.ToLower(q.Name.String()), 0)
	if typeFound {
		return list
	}
	return nil
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
	now := util.TimeNow()
	for i := 0; i < length; i++ {
		rec := (*list)[i]
		if rec.expiryDate <= now {
			expiredHeader := (*list)[i].resource.Header
			util.LogDebug(util.DnsModule, txt.DnsSrvRecordExpired, []string{txt.DnsSrvClass, txt.DnsSrvType, txt.DnsSrvName}, []string{strconv.Itoa(int(expiredHeader.Class)), strconv.Itoa(int(expiredHeader.Type)), expiredHeader.Name.String()})
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
