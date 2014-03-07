package lua

import (
	"math"
)

type table struct {
	array     []value
	hash      map[value]value
	metaTable *table
	flags     byte
}

func newTable() *table                     { return &table{hash: make(map[value]value)} }
func (t *table) invalidateTagMethodCache() { t.flags = 0 }
func (t *table) atString(k string) value   { return t.hash[k] }

func newTableWithSize(arraySize, hashSize int) *table {
	t := new(table)
	if arraySize > 0 {
		t.array = make([]value, arraySize)
	}
	if hashSize > 0 {
		t.hash = make(map[value]value, hashSize)
	} else {
		t.hash = make(map[value]value)
	}
	return t
}

func (l *State) fastTagMethod(table *table, event tm) value {
	if table == nil || table.flags&1<<event != 0 {
		return nil
	}
	return table.tagMethod(event, l.global.tagMethodNames[event])
}

func (t *table) extendArray(last int) { t.array = append(t.array, make([]value, last-len(t.array))...) }

func (t *table) atInt(k int) value {
	if 0 < k && k <= len(t.array) {
		return t.array[k-1]
	}
	return t.hash[float64(k)]
}

func (t *table) putAtInt(k int, v value) {
	if 0 < k && k <= len(t.array) {
		t.array[k-1] = v
	} else {
		t.hash[float64(k)] = v
	}
}

func (t *table) at(k value) value {
	switch k := k.(type) {
	case nil:
		return nil
	case float64:
		if i := int(k); float64(i) == k {
			return t.atInt(i)
		}
	case string:
		return t.atString(k)
	}
	return t.hash[k]
}

func (t *table) put(l *State, k, v value) {
	switch k := k.(type) {
	case nil:
		l.runtimeError("table index is nil")
	case float64:
		if i := int(k); float64(i) == k {
			t.putAtInt(i, v)
		} else if math.IsNaN(k) {
			l.runtimeError("table index is NaN")
		} else {
			t.hash[k] = v
		}
	case string:
		t.hash[k] = v
	default:
		t.hash[k] = v
	}
}

func (t *table) unboundSearch(j int) int {
	i := j
	for j++; nil != t.atInt(j); {
		i = j
		if j *= 2; j < 0 {
			for i = 1; nil != t.atInt(i); i++ {
			}
			return i - 1
		}
	}
	for j-i > 1 {
		m := (i + j) / 2
		if nil == t.atInt(m) {
			j = m
		} else {
			i = m
		}
	}
	return i
}

func (t *table) length() int {
	j := len(t.array)
	if j > 0 && t.array[j-1] == nil {
		i := 0
		for j-i > 1 {
			m := (i + j) / 2
			if t.array[m-1] == nil {
				j = m
			} else {
				i = m
			}
		}
		return i
	} else if t.hash == nil {
		return j
	}
	return t.unboundSearch(j)
}

func arrayIndex(k value) int {
	if n, ok := k.(float64); ok {
		if i := int(n); float64(i) == n {
			return i
		}
	}
	return -1
}

func (l *State) next(t *table, key int) bool {
	i, k := 0, l.stack[key]
	if k == nil { // first iteration
	} else if i = arrayIndex(k); 0 < i && i <= len(t.array) {
	} else if _, ok := t.hash[k]; !ok {
		l.runtimeError("invalid key to 'next'") // key not found
	} else {
		i = len(t.array)
	}
	for ; i < len(t.array); i++ {
		if t.array[i] != nil {
			l.stack[key] = float64(i + 1)
			l.stack[key+1] = t.array[i]
			return true
		}
	}
	found := k == nil
	for hk, v := range t.hash {
		if found {
			if v != nil {
				l.stack[key] = hk
				l.stack[key+1] = v
				return true
			}
		} else if l.equalObjects(hk, k) {
			found = true
		}
	}
	return false // no more elements
}
