package store

// InvertedIndex maps string values to lists of event indices.
type InvertedIndex struct {
	entries map[string][]int
}

func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{entries: make(map[string][]int)}
}

func (idx *InvertedIndex) Add(value string, eventIndex int) {
	idx.entries[value] = append(idx.entries[value], eventIndex)
}

func (idx *InvertedIndex) Get(value string) []int {
	return idx.entries[value]
}

// IntIndex maps int values to lists of event indices.
type IntIndex struct {
	entries map[int][]int
}

func NewIntIndex() *IntIndex {
	return &IntIndex{entries: make(map[int][]int)}
}

func (idx *IntIndex) Add(value int, eventIndex int) {
	idx.entries[value] = append(idx.entries[value], eventIndex)
}

func (idx *IntIndex) Get(value int) []int {
	return idx.entries[value]
}
