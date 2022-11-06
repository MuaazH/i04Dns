package config

import (
	"i04Dns/dns/dnsmessage"
	"i04Dns/util/bst"
	"strings"
)

type Database struct {
	tree *bst.Tree
}

func NewDatabase() *Database {
	return &Database{bst.NewWithStringComparator()}
}

func (db *Database) IsEmpty() bool {
	return db.tree == nil || db.tree.Empty()
}

// Add adds shit. Use carefully, Can add duplicates
func (db *Database) Add(rec *dnsmessage.Resource) {
	if rec.Header.TTL == 0 {
		return
	}
	nameStr := strings.ToLower(rec.Header.Name.String())
	list, found := db.tree.Get(nameStr)
	if found {
		recordList := list.(*[]dnsmessage.Resource)
		*recordList = append(*recordList, *rec)
		// todo: prevent adding duplicate records
	} else {
		newList := []dnsmessage.Resource{*rec}
		db.tree.Put(nameStr, &newList)
	}
}

func (db *Database) LookupAnswer(q *dnsmessage.Question) (*[]dnsmessage.Resource, *[]dnsmessage.Resource) {
	list, typeFound := getRecord(db.tree, q.Class, q.Type, strings.ToLower(q.Name.String()), 0)
	type2Found := false
	var list2 *[]dnsmessage.Resource = nil
	if q.Type == dnsmessage.TypeA {
		list2, type2Found = getRecord(db.tree, q.Class, dnsmessage.TypeAAAA, strings.ToLower(q.Name.String()), 0)
	} else if q.Type == dnsmessage.TypeAAAA {
		list2, type2Found = getRecord(db.tree, q.Class, dnsmessage.TypeA, strings.ToLower(q.Name.String()), 0)
	}
	if typeFound {
		return list, list2
	}
	if type2Found {
		return &[]dnsmessage.Resource{}, list2
	}
	return nil, list2
}

func getRecord(tree *bst.Tree, class dnsmessage.Class, typ dnsmessage.Type, name string, depth int) (*[]dnsmessage.Resource, bool) {
	// todo: convert 127.0.0.1 to the right interface here, but also copy the record so the db is unchanged
	// don't go too deep
	if depth >= 3 {
		return nil, false
	}
	list, found := tree.Get(name)
	typeFound := false
	if found {
		recordList := *list.(*[]dnsmessage.Resource)
		const maxCount = 32
		result := [maxCount]dnsmessage.Resource{}
		rCount := 0
		count := len(recordList)
		for i := 0; i < count; i++ {
			rec := recordList[i]
			// if a cname is found, search recursively
			if typ != dnsmessage.TypeCNAME && rec.Header.Type == dnsmessage.TypeCNAME {
				cname := rec.Body.(*dnsmessage.CNAMEResource)
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
			if rec.Header.Type == typ && rec.Header.Class == class {
				typeFound = true
				result[rCount] = rec
				// todo: add ttl this to config
				rec.Header.TTL = 5
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
