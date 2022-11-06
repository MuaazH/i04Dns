package config

import (
	"strings"
	"sync"
)

const (
	asteriskS     = "*"
	digitAsterisk = '%'
	charAsterisk  = '$'
)

type CompiledNameFilter struct {
	words      *[]string
	name       string
	isShadowed bool
}

type FilterList struct {
	names      *[]*CompiledNameFilter
	usefulSize int
	isCompiled bool
}

func (f *CompiledNameFilter) String() string {
	if f == nil {
		return "null"
	}
	str := f.name
	if f.isShadowed {
		str = str + " [S]"
	}
	return str
}

func (list FilterList) String() string {
	var sb strings.Builder
	if list.isCompiled {
		sb.WriteString("Compiled")
	} else {
		sb.WriteString("Raw")
	}
	sb.WriteString(" [")
	L := list.Size()
	for i := 0; i < L; i++ {
		sb.WriteString("\n    ")
		sb.WriteString((*list.names)[i].String())
	}
	sb.WriteString("\n]")
	return sb.String()
}

func NewFilterList(size int) *FilterList {
	e := make([]*CompiledNameFilter, size)[0:0]
	var list = FilterList{&e, 0, true}
	return &list
}

func newNameFilter(filterString string, name string) *CompiledNameFilter {
	low := strings.ToLower(filterString)
	words := strings.Split(low, asteriskS)
	obj := CompiledNameFilter{&words, name, false}
	return &obj
}

func (list *FilterList) Add(filter string, name string) *FilterList {
	l := len(*list.names)
	item := newNameFilter(filter, name)
	if l == cap(*list.names) {
		sz := l * 2
		if sz < 2 {
			sz = 2
		}
		arr := make([]*CompiledNameFilter, sz)
		slc := arr[0 : l+1]
		copy(slc, *list.names)
		slc[l] = item
		list.names = &slc
	} else {
		out := (*list.names)[0 : l+1]
		out[l] = item
		list.names = &out
	}
	list.isCompiled = false
	return list
}

func (list *FilterList) Remove(index int) *FilterList {
	size := list.Size()
	if size > 0 && index >= 0 && index < size {
		last := size - 1
		// check the size
		quarter := list.Capacity() >> 2
		if last < quarter && quarter > 2 {
			arr := make([]*CompiledNameFilter, list.Capacity()>>1)
			for i := 0; i < index; i++ {
				arr[i] = (*list.names)[i]
			}
			j := index + 1
			for i := index; i < last; i++ {
				arr[i] = (*list.names)[j]
				j++
			}
			out := arr[0:last]
			list.names = &out
		} else {
			(*list.names)[index] = (*list.names)[last]
			out := (*list.names)[0:last]
			list.names = &out
		}
	}
	list.usefulSize = 0
	list.isCompiled = false
	return list
}

func (list *FilterList) Capacity() int {
	return cap(*list.names)
}

func (list *FilterList) Size() int {
	return len(*list.names)
}

func (list *FilterList) IsCompiled() bool {
	return list.isCompiled
}

func (list *FilterList) Compile() *FilterList {
	s := list.Size()
	// todo: shadowed names where moved to the end of list. Improve this code
	// reset
	for i := 0; i < s; i++ {
		(*list.names)[i].isShadowed = false
	}
	// sort without shadowed shit
	mergeSort(list.names)
	// start
	var wg sync.WaitGroup
	for i := 1; i < s; i++ {
		for j := 0; j < i; j++ {
			wg.Add(1)
			a := (*list.names)[j]
			b := (*list.names)[i]
			go verifyFilter(a, b, &wg)
		}
	}
	wg.Wait()
	list.isCompiled = true
	// sort again to move shadowed names to the end
	mergeSort(list.names)
	//
	for u := s - 1; u > -1; u-- {
		if !(*list.names)[u].isShadowed {
			list.usefulSize = u + 1
			break
		}
	}
	return list
}

func (list *FilterList) FindFilter(txt *string) *string {
	if list == nil || !list.isCompiled {
		return nil
	}
	S := list.usefulSize
	for i := 0; i < S; i++ {
		n := (*list.names)[i]
		if matches(txt, n) {
			return &n.name
		}
	}
	return nil
}

// ###########################
// #  Pattern Matching Sort  #
// ###########################

func cmpChar(filter uint8, subject uint8) bool {
	if filter == charAsterisk || filter == subject {
		return true
	}
	if filter == digitAsterisk {
		return '0' <= subject && subject <= '9'
	}
	return false
}

func isPrefixMoreGeneral(a string, b string) bool {
	aLength := len(a)
	bLength := len(b)
	// 'b' starts with '*' while 'a' does not => 'a' is not more general than 'b'
	if aLength > 0 && bLength == 0 {
		return false
	}
	// 'a' is longer than 'b' => 'a' is not more general than 'b'
	if aLength > bLength {
		return false
	}
	// make sure every char of 'a' matches the corresponding char in 'b'
	for i := 0; i < aLength; i++ {
		if !cmpChar(a[i], b[i]) {
			return false
		}
	}
	return true
}

func isSuffixMoreGeneral(a string, b string) bool {
	aLength := len(a)
	bLength := len(b)
	// 'b' ends with '*' while 'a' does not => 'a' is not more general than 'b'
	if aLength > 0 && bLength == 0 {
		return false
	}
	// 'a' is longer than 'b' => 'a' is not more general than 'b'
	if aLength > bLength {
		return false
	}
	// make sure every char of 'a' matches the corresponding char in 'b'
	j := bLength - 1
	for i := aLength - 1; i > -1; i-- {
		if !cmpChar(a[i], b[j]) {
			return false
		}
		j--
	}
	return true
}

func verifyFilter(matcher *CompiledNameFilter, x *CompiledNameFilter, wg *sync.WaitGroup) {
	defer wg.Done()

	if !isPrefixMoreGeneral((*matcher.words)[0], (*x.words)[0]) {
		return
	}
	ac := len(*matcher.words)
	bc := len(*x.words)
	if !isSuffixMoreGeneral((*matcher.words)[ac-1], (*x.words)[bc-1]) {
		return
	}
	if !matches(&x.name, matcher) {
		return
	}
	x.isShadowed = true
}

func find(text string, index int, filter string) (bool, int) {
	tL := len(text)
	fL := len(filter)
	E := tL - (fL - 1)
	for i := index; i < E; i++ {
		matchFound := true
		for j := 0; j < fL; j++ {
			if !cmpChar(text[i+j], filter[j]) {
				matchFound = false
				break
			}
		}
		if matchFound {
			i += fL
			return true, i
		}
	}
	return false, -1
}

func startsWith(text *string, prefix *string) bool {
	L := len(*text)
	l := len(*prefix)
	if l > L {
		return false
	}
	for i := 0; i < l; i++ {
		if (*text)[i] != (*prefix)[i] {
			return false
		}
	}
	return true
}

func endsWith(text *string, suffix *string) bool {
	L := len(*text)
	l := len(*suffix)
	if l > L {
		return false
	}
	l--
	L--
	for i := 0; i < l; i++ {
		if (*text)[L-i] != (*suffix)[l-i] {
			return false
		}
	}
	return true
}

func matches(text *string, f *CompiledNameFilter) bool {
	L := len(*(f.words)) - 1
	idx := 0
	var success bool
	if !startsWith(text, &(*f.words)[0]) || !endsWith(text, &(*f.words)[L]) {
		return false
	}
	for i := 1; i < L; i++ {
		success, idx = find(*text, idx, (*f.words)[i])
		if !success {
			return false
		}
	}
	return true
}

// ################
// #  Merge Sort  #
// ################

func compare(a *CompiledNameFilter, b *CompiledNameFilter) int {
	// ASCII Only!!
	if a == nil {
		if b == nil {
			return 0
		}
		return 1
	}
	if b == nil {
		return -1
	}

	if a.isShadowed {
		if !b.isShadowed {
			return 1
		}
	} else {
		if b.isShadowed {
			return -1
		}
	}

	lenA := len(a.name)
	lenB := len(b.name)
	if lenA < lenB {
		return -1
	}
	if lenB < lenA {
		return 1
	}
	for i := 0; i < lenA; i++ {
		x := a.name[i]
		y := b.name[i]
		if x < y {
			return -1
		}
		if y < x {
			return 1
		}
	}
	return 0
}

func mergeSort(array *[]*CompiledNameFilter) {
	size := len(*array)
	temp := make([]*CompiledNameFilter, size)
	mergeSortDoSort(array, &temp, 0, size-1)
}

func mergeSortDoSort(a *[]*CompiledNameFilter, aux *[]*CompiledNameFilter, lo int, hi int) {
	if hi <= lo {
		return
	}
	mid := (lo + hi) >> 1

	mergeSortDoSort(a, aux, lo, mid)
	mergeSortDoSort(a, aux, mid+1, hi)

	mergeSortDoMerge(a, aux, lo, mid, hi)
}

func mergeSortDoMerge(a *[]*CompiledNameFilter, aux *[]*CompiledNameFilter, lo int, mid int, hi int) {
	// copy to aux[]
	for k := lo; k <= hi; k++ {
		(*aux)[k] = (*a)[k]
	}
	// merge back to a[]
	i := lo
	j := mid + 1
	for k := lo; k <= hi; k++ {
		if i > mid {
			(*a)[k] = (*aux)[j]
			j++
		} else if j > hi {
			(*a)[k] = (*aux)[i]
			i++
		} else if compare((*aux)[j], (*aux)[i]) < 0 {
			(*a)[k] = (*aux)[j]
			j++
		} else {
			(*a)[k] = (*aux)[i]
			i++
		}
	}
}
