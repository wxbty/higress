package lcache

import (
	"fmt"
	"math"
)

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

// CosineSimilarity 计算两个向量之间的余弦相似度
func CosineSimilarity(v1, v2 []float64) float64 {
	if len(v1) != len(v2) {
		// 如果两个向量长度不同，无法计算余弦相似度
		return 0
	}

	dotProduct := 0.0
	normV1 := 0.0
	normV2 := 0.0

	for i := range v1 {
		dotProduct += v1[i] * v2[i]
		normV1 += v1[i] * v1[i]
		normV2 += v2[i] * v2[i]
	}

	normV1 = math.Sqrt(normV1)
	normV2 = math.Sqrt(normV2)

	// 避免除以零的情况
	if normV1 == 0 || normV2 == 0 {
		return 0
	}

	return dotProduct / (normV1 * normV2)
}

var threshold = 0.65

// RetrieveBestAnswer 根据当前问题检索最佳答案，考虑直接答案和跨轮次的答案
func (c *Cache) RetrieveBestAnswer(currentQuestionKey string, currentQuestionVector []float64) string {
	// 直接根据当前问题键检索答案
	answers, exists := c.cache[currentQuestionKey]
	if exists {
		//循环对比每个answer向量和key向量的相似度
		var bestAnswerText string
		highestSimilarity := -1.0
		for _, av := range answers {
			similarity := CosineSimilarity(currentQuestionVector, av.AnswerVector)
			if similarity > threshold {
				if similarity > highestSimilarity {
					highestSimilarity = similarity
					bestAnswerText = av.AnswerText
				}
			}
		}
		if highestSimilarity > threshold {
			fmt.Println("response from local cache , similarity:", highestSimilarity)
			return bestAnswerText
		}
	}

	// 如果当前问题没有直接答案，检查跨轮次的答案
	var bestAnswerText string
	highestSimilarity := -1.0

	for _, answerList := range c.cache {
		for _, av := range answerList {
			// 检查前一个问题的键是否与当前问题键匹配
			preTwoVector := av.PreTwoVector
			similarity := CosineSimilarity(currentQuestionVector, preTwoVector)
			if similarity > threshold {
				//对比当前问题和候选回复的相似度
				similarity = CosineSimilarity(currentQuestionVector, av.AnswerVector)
				if similarity > highestSimilarity {
					highestSimilarity = similarity
					bestAnswerText = av.AnswerText
				}

			}
		}
	}

	if highestSimilarity > threshold {
		fmt.Println("response from local cache , similarity:", highestSimilarity)
		return bestAnswerText
	} else {
		fmt.Println("local cache miss , similarity:", highestSimilarity)
		return ""
	}
}
