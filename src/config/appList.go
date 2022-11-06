package config

import "i04Dns/util"

type CompiledAppRule struct {
	Name      *string
	Path      *string
	Whitelist *FilterList
	Blacklist *FilterList
}

type AppRuleList []*CompiledAppRule

func (list *AppRuleList) Sort() {
	mergeSortCar(list)
}

func (list *AppRuleList) FindName(path *string) *CompiledAppRule {
	if util.IsNullOrBlank(path) {
		if len(*list) > 0 && *(*list)[0].Path == "*" {
			return (*list)[0]
		}
		return nil
	}
	idx := binarySearch(*list, 0, len(*list)-1, path)
	if idx > -1 {
		return (*list)[idx]
	}
	if len(*list) > 0 && *(*list)[0].Path == "*" {
		return (*list)[0]
	}
	return nil
}

func binarySearch(a []*CompiledAppRule, lo int, hi int, search *string) int {
	for lo <= hi {
		mid := (lo + hi) >> 1
		cmp := compareCarName(a[mid].Path, search)
		if cmp == 0 {
			return mid
		}
		if cmp > 0 {
			hi = mid - 1
		} else {
			lo = mid + 1
		}
	}
	return -1
}

// ################
// #  Merge Sort  #
// ################

func compareCar(a *CompiledAppRule, b *CompiledAppRule) int {
	// ASCII Only!!
	if a == nil || a.Path == nil {
		if b == nil || b.Path == nil {
			return 0
		}
		return 1
	}
	if b == nil || b.Path == nil {
		return -1
	}
	return compareCarName(a.Path, b.Path)
}

func compareCarName(a *string, b *string) int {
	if *a == "*" {
		if *b == "*" {
			return 0
		}
		return -1
	}
	if *b == "*" {
		return 1
	}

	lenA := len(*a)
	lenB := len(*b)
	if lenA < lenB {
		return -1
	}
	if lenB < lenA {
		return 1
	}
	for i := 0; i < lenA; i++ {
		x := (*a)[i]
		y := (*b)[i]
		if x < y {
			return -1
		}
		if y < x {
			return 1
		}
	}
	return 0
}

func mergeSortCar(array *AppRuleList) {
	size := len(*array)
	temp := make(AppRuleList, size)
	mergeSortDoSortCar(array, &temp, 0, size-1)
}

func mergeSortDoSortCar(a *AppRuleList, aux *AppRuleList, lo int, hi int) {
	if hi <= lo {
		return
	}
	mid := (lo + hi) >> 1

	mergeSortDoSortCar(a, aux, lo, mid)
	mergeSortDoSortCar(a, aux, mid+1, hi)

	mergeSortDoMergeCar(a, aux, lo, mid, hi)
}

func mergeSortDoMergeCar(a *AppRuleList, aux *AppRuleList, lo int, mid int, hi int) {
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
		} else if compareCar((*aux)[j], (*aux)[i]) < 0 {
			(*a)[k] = (*aux)[j]
			j++
		} else {
			(*a)[k] = (*aux)[i]
			i++
		}
	}
}
