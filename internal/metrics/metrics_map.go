package metrics

// Type to hold map of oldest queue elements
type MessagePoolMetricsMap struct {
	OldestElement map[string]int64
	TotalElements map[string]int64
}

func NewMessagePoolMetricsMap() *MessagePoolMetricsMap {
	return &MessagePoolMetricsMap{
		OldestElement: make(map[string]int64, 0),
		TotalElements: make(map[string]int64, 0),
	}
}

func (oldestMap *MessagePoolMetricsMap) UpdateOldestElementMap(data map[string]int64) {
	oldestMap.OldestElement = mergeData(oldestMap.OldestElement, data)
}

func (oldestMap *MessagePoolMetricsMap) UpdateTotalElementsMap(data map[string]int64) {
	oldestMap.TotalElements = mergeData(oldestMap.TotalElements, data)
}

func mergeData(currentData map[string]int64, newData map[string]int64) map[string]int64 {
	if newData == nil {
		return make(map[string]int64, 0)
	}

	for key := range currentData {
		if _, ok := newData[key]; !ok {
			newData[key] = int64(0)
		}
	}

	return newData
}
