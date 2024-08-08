package lcache

// Cache 结构体使用 map 存储问题键和相关答案的列表
type Cache struct {
	cache map[string][]AnswerVector
}

// NewCache 初始化 Cache
func NewCache() *Cache {
	return &Cache{
		cache: make(map[string][]AnswerVector),
	}
}

// AnswerVector 结构体存储答案的文本、向量表示以及问题键
type AnswerVector struct {
	DocId          string
	AnswerText     string
	AnswerVector   []float64
	QuestionVector []float64
	PreTwoVector   []float64
	QuestionKey    string
	PreviousKey    string // 存储前一个问题的键
}

// AddToCache 向缓存中添加问题和答案
func (c *Cache) AddToCache(docID, questionKey, previousKey, answerText string, answerVector, questionVector []float64) {
	av := AnswerVector{
		DocId:          docID,
		AnswerText:     answerText,
		AnswerVector:   answerVector,
		QuestionVector: questionVector,
		QuestionKey:    questionKey,
		PreviousKey:    previousKey,
	}
	c.cache[questionKey] = append(c.cache[questionKey], av)
}
