package labelpack

// Iter is a forward cursor over a packed Labels buffer. The zero value iterates
// an empty set. Iter must be used by pointer.
type Iter struct {
	data string
	i    int
}

// Iter returns a cursor over l, yielding labels in ascending name order.
func (l Labels) Iter() Iter {
	return Iter{data: string(l)}
}

// Next returns the next label and true, or zero values and false once the
// iterator is exhausted.
func (it *Iter) Next() (name, value string, ok bool) {
	if it.i >= len(it.data) {
		return "", "", false
	}
	name, it.i = decodeString(it.data, it.i)
	value, it.i = decodeString(it.data, it.i)
	return name, value, true
}

// MergeIter iterates the union of two packed label sets in ascending name
// order. A label present in own shadows the label of the same name in group.
// MergeIter must be used by pointer.
type MergeIter struct {
	group Iter
	own   Iter

	groupName  string
	groupValue string
	groupOK    bool

	ownName  string
	ownValue string
	ownOK    bool
}

// Merge returns an iterator over the union of group and own, in ascending name
// order, where own shadows group for labels present in both.
func Merge(group, own Labels) MergeIter {
	m := MergeIter{group: group.Iter(), own: own.Iter()}
	m.groupName, m.groupValue, m.groupOK = m.group.Next()
	m.ownName, m.ownValue, m.ownOK = m.own.Next()
	return m
}

// Next returns the next label of the merged set and true, or zero values and
// false once the iterator is exhausted.
func (m *MergeIter) Next() (name, value string, ok bool) {
	switch {
	case m.groupOK && m.ownOK:
		switch {
		case m.groupName < m.ownName:
			name, value = m.groupName, m.groupValue
			m.groupName, m.groupValue, m.groupOK = m.group.Next()
		case m.ownName < m.groupName:
			name, value = m.ownName, m.ownValue
			m.ownName, m.ownValue, m.ownOK = m.own.Next()
		default:
			// Same name in both: own wins, and the group entry is consumed too.
			name, value = m.ownName, m.ownValue
			m.groupName, m.groupValue, m.groupOK = m.group.Next()
			m.ownName, m.ownValue, m.ownOK = m.own.Next()
		}
		return name, value, true
	case m.groupOK:
		name, value = m.groupName, m.groupValue
		m.groupName, m.groupValue, m.groupOK = m.group.Next()
		return name, value, true
	case m.ownOK:
		name, value = m.ownName, m.ownValue
		m.ownName, m.ownValue, m.ownOK = m.own.Next()
		return name, value, true
	default:
		return "", "", false
	}
}

// RangeMerged calls f for each label of the union of group and own, in ascending
// name order, where own shadows group. If f returns false the iteration stops
// and RangeMerged returns false; otherwise it returns true once every label has
// been visited.
func RangeMerged(group, own Labels, f func(name, value string) bool) bool {
	// Fast paths avoid the merge bookkeeping when one side is empty, which is
	// common: targets with no group labels, or group labels with no overrides.
	switch {
	case group.IsEmpty():
		return own.Range(f)
	case own.IsEmpty():
		return group.Range(f)
	}

	it := Merge(group, own)
	for {
		name, value, ok := it.Next()
		if !ok {
			return true
		}
		if !f(name, value) {
			return false
		}
	}
}

// EqualMerged reports whether the union of aGroup and aOwn holds exactly the same
// labels as the union of bGroup and bOwn. It walks both merged views in lockstep,
// so it is linear in the total number of labels and allocates nothing.
func EqualMerged(aGroup, aOwn, bGroup, bOwn Labels) bool {
	// Identical buffers on both sides mean identical merged views, without
	// needing to decode anything.
	if aGroup == bGroup && aOwn == bOwn {
		return true
	}

	a := Merge(aGroup, aOwn)
	b := Merge(bGroup, bOwn)
	for {
		aName, aValue, aOK := a.Next()
		bName, bValue, bOK := b.Next()
		if !aOK || !bOK {
			// Equal only if both ran out at the same point.
			return aOK == bOK
		}
		if aName != bName || aValue != bValue {
			return false
		}
	}
}

// Overlaps reports whether a and b share at least one label name. It scans both
// buffers once and allocates nothing.
func Overlaps(a, b Labels) bool {
	if a.IsEmpty() || b.IsEmpty() {
		return false
	}

	aIter, bIter := a.Iter(), b.Iter()
	aName, _, aOK := aIter.Next()
	bName, _, bOK := bIter.Next()
	for aOK && bOK {
		switch {
		case aName < bName:
			aName, _, aOK = aIter.Next()
		case bName < aName:
			bName, _, bOK = bIter.Next()
		default:
			return true
		}
	}
	return false
}

// MergedLen returns the number of labels in the union of group and own, counting
// a name present in both only once. It requires a full scan of both buffers.
func MergedLen(group, own Labels) int {
	switch {
	case group.IsEmpty():
		return own.Len()
	case own.IsEmpty():
		return group.Len()
	}

	count := 0
	it := Merge(group, own)
	for {
		if _, _, ok := it.Next(); !ok {
			return count
		}
		count++
	}
}
