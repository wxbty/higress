package embedding

import (
	"encoding/json"
	"fmt"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache-xzz/dashscope"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache-xzz/dashvector"
	"github.com/alibaba/higress/plugins/wasm-go/pkg/wrapper"
	"net/http"
)

type Response struct {
	Output struct {
		Embeddings []struct {
			TextIndex int       `json:"text_index"`
			Embedding []float64 `json:"embedding"`
		} `json:"embeddings"`
	} `json:"output"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	RequestID string `json:"request_id"`
}

type Doc struct {
	TOPK   int       `json:"topk"`
	Vector []float64 `json:"vector"`
	Filter string    `json:"filter"`
}

type Fields struct {
	DATA string `json:"data"`
}

type Docs struct {
	Docs []Doc `json:"docs"`
}

func GetEmbedding(httpClient wrapper.HttpClient, apiKey, key, preKey string, cb func(statusCode int, responseHeaders http.Header, responseBody []byte)) {
	// 定义请求体
	texts := []string{
		key,
	}
	if preKey != "" {
		texts = []string{
			preKey, key,
		}
	}
	requestEmbedding := dashscope.Request{
		Model: "text-embedding-v1",
		Input: dashscope.Input{
			Texts: texts,
		},
		Parameter: dashscope.Parameter{
			TextType: "query",
		},
	}
	headers := [][2]string{{"Content-Type", "application/json"}, {"Authorization", "Bearer " + apiKey}}
	reqEmbeddingSerialized, _ := json.Marshal(requestEmbedding)

	httpClient.Post(
		"/api/v1/services/embeddings/text-embedding/text-embedding",
		headers,
		reqEmbeddingSerialized,
		cb,
		50000,
	)

}

func QueryValByEmbeddingKey(httpClient wrapper.HttpClient, apiKey, collection string, keyEmbedding []float64, cb func(statusCode int, responseHeaders http.Header, responseBody []byte)) {
	// 定义请求体
	requestQuery := dashvector.Request{
		TopK:         1,
		OutputFileds: []string{"raw"},
		Vector:       keyEmbedding,
	}

	requestQuerySerialized, _ := json.Marshal(requestQuery)
	headers := [][2]string{{"Content-Type", "application/json"}, {"dashvector-auth-token", apiKey}}

	httpClient.Post(
		fmt.Sprintf("/v1/collections/%s/query", collection),
		headers,
		requestQuerySerialized,
		cb,
		50000,
	)

}
