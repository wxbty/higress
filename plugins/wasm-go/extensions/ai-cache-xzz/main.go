// File generated by hgctl. Modify as required.
// See: https://higress.io/zh-cn/docs/user/wasm-go#2-%E7%BC%96%E5%86%99-maingo-%E6%96%87%E4%BB%B6

package main

import (
	"encoding/json"
	"fmt"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache-xzz/dashscope"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache-xzz/dashvector"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache-xzz/embedding"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache-xzz/lcache"
	"github.com/tidwall/sjson"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/pkg/wrapper"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/tidwall/gjson"
)

const (
	CacheKeyContextKey       = "cacheKey"
	CacheEmbeddingKey        = "cacheEmbeddingKey"
	CacheContentContextKey   = "cacheContent"
	PartialMessageContextKey = "partialMessage"
	ToolCallsContextKey      = "toolCalls"
	StreamContextKey         = "stream"
)

var localCache *lcache.Cache

func main() {
	localCache = lcache.NewCache()
	wrapper.SetCtx(
		"ai-cache-xzz",
		wrapper.ParseConfigBy(parseConfig),
		wrapper.ProcessRequestHeadersBy(onHttpRequestHeaders),
		wrapper.ProcessRequestBodyBy(onHttpRequestBody),
		wrapper.ProcessResponseHeadersBy(onHttpResponseHeaders),
		wrapper.ProcessStreamingResponseBodyBy(onHttpResponseBody),
	)
}

type AIRagConfig struct {
	DashScopeClient      wrapper.HttpClient
	DashScopeAPIKey      string
	DashVectorClient     wrapper.HttpClient
	JieClient            wrapper.HttpClient
	DashVectorAPIKey     string
	DashVectorCollection string

	CacheKeyFrom KVExtractor `required:"true" yaml:"cacheKeyFrom" json:"cacheKeyFrom"`
	// @Title zh-CN 缓存 value 的来源
	// @Description zh-CN 往 redis 里存时，使用的 value 的提取方式
	CacheValueFrom KVExtractor `required:"true" yaml:"cacheValueFrom" json:"cacheValueFrom"`
	// @Title zh-CN 流式响应下，缓存 value 的来源
	// @Description zh-CN 往 redis 里存时，使用的 value 的提取方式
	CacheStreamValueFrom KVExtractor `required:"true" yaml:"cacheStreamValueFrom" json:"cacheStreamValueFrom"`
	// @Title zh-CN 返回 HTTP 响应的模版
	// @Description zh-CN 用 %s 标记需要被 cache value 替换的部分
	ReturnResponseTemplate string `required:"true" yaml:"returnResponseTemplate" json:"returnResponseTemplate"`
	// @Title zh-CN 返回流式 HTTP 响应的模版
	// @Description zh-CN 用 %s 标记需要被 cache value 替换的部分
	ReturnStreamResponseTemplate string `required:"true" yaml:"returnStreamResponseTemplate" json:"returnStreamResponseTemplate"`
}

type Doc struct {
	ID     string    `json:"id"`
	FIELDS Fields    `json:"fields"`
	Vector []float64 `json:"vector"`
}

type Docs struct {
	Docs []Doc `json:"docs"`
}

type Fields struct {
	DATA string `json:"data"`
}

type KVExtractor struct {
	// @Title zh-CN 从请求 Body 中基于 [GJSON PATH](https://github.com/tidwall/gjson/blob/master/SYNTAX.md) 语法提取字符串
	Prefix       string `required:"false" yaml:"prefix" json:"prefix"`
	RequestBody  string `required:"false" yaml:"requestBody" json:"requestBody"`
	RequestBodyT string `required:"false" yaml:"requestBodyT" json:"requestBodyT"`
	// @Title zh-CN 从响应 Body 中基于 [GJSON PATH](https://github.com/tidwall/gjson/blob/master/SYNTAX.md) 语法提取字符串
	ResponseBody string `required:"false" yaml:"responseBody" json:"responseBody"`
}

func parseConfig(json gjson.Result, config *AIRagConfig, log wrapper.Log) error {

	serviceName := json.Get("nlp.serviceName").String()
	servicePort := json.Get("nlp.servicePort").Int()
	if servicePort == 0 {
		if strings.HasSuffix(serviceName, ".static") {
			// 静态IP类型服务的逻辑端口是80
			servicePort = 80
		}
	}
	config.JieClient = wrapper.NewClusterClient(wrapper.FQDNCluster{
		FQDN: serviceName,
		Host: serviceName,
		Port: servicePort,
	})

	config.DashScopeAPIKey = json.Get("dashscope.apiKey").String()

	config.DashScopeClient = wrapper.NewClusterClient(wrapper.DnsCluster{
		ServiceName: json.Get("dashscope.serviceName").String(),
		Port:        json.Get("dashscope.servicePort").Int(),
		Domain:      json.Get("dashscope.domain").String(),
	})
	config.DashVectorAPIKey = json.Get("dashvector.apiKey").String()
	config.DashVectorCollection = json.Get("dashvector.collection").String()
	config.DashVectorClient = wrapper.NewClusterClient(wrapper.DnsCluster{
		ServiceName: json.Get("dashvector.serviceName").String(),
		Port:        json.Get("dashvector.servicePort").Int(),
		Domain:      json.Get("dashvector.domain").String(),
	})

	config.CacheKeyFrom.Prefix = json.Get("cacheKeyFrom.prefix").String()
	config.CacheKeyFrom.RequestBody = json.Get("cacheKeyFrom.requestBody").String()
	if config.CacheKeyFrom.RequestBody == "" {
		config.CacheKeyFrom.RequestBody = "messages.@reverse.0.content"
	}
	config.CacheKeyFrom.RequestBodyT = json.Get("cacheKeyFrom.requestBodyT").String()
	if config.CacheKeyFrom.RequestBodyT == "" {
		config.CacheKeyFrom.RequestBodyT = "messages.2.content"
	}
	config.CacheValueFrom.ResponseBody = json.Get("cacheValueFrom.responseBody").String()
	if config.CacheValueFrom.ResponseBody == "" {
		config.CacheValueFrom.ResponseBody = "choices.0.message.content"
	}
	config.CacheStreamValueFrom.ResponseBody = json.Get("cacheStreamValueFrom.responseBody").String()
	if config.CacheStreamValueFrom.ResponseBody == "" {
		config.CacheStreamValueFrom.ResponseBody = "choices.0.delta.content"
	}
	config.ReturnResponseTemplate = json.Get("returnResponseTemplate").String()
	if config.ReturnResponseTemplate == "" {
		config.ReturnResponseTemplate = `{"id":"from-cache","choices":[{"index":0,"message":{"role":"assistant","content":"%s"},"finish_reason":"stop"}],"model":"gpt-4o","object":"chat.completion","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
	}
	config.ReturnStreamResponseTemplate = json.Get("returnStreamResponseTemplate").String()
	if config.ReturnStreamResponseTemplate == "" {
		config.ReturnStreamResponseTemplate = `data:{"id":"from-cache","choices":[{"index":0,"delta":{"role":"assistant","content":"%s"},"finish_reason":"stop"}],"model":"gpt-4o","object":"chat.completion","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}` + "\n\ndata:[DONE]\n\n"
	}
	return nil
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, config AIRagConfig, log wrapper.Log) types.Action {
	contentType, _ := proxywasm.GetHttpRequestHeader("content-type")
	// The request does not have a body.
	if contentType == "" {
		return types.ActionContinue
	}
	if !strings.Contains(contentType, "application/json") {
		log.Warnf("content is not json, can't process:%s", contentType)
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	proxywasm.RemoveHttpRequestHeader("Accept-Encoding")
	// The request has a body and requires delaying the header transmission until a cache miss occurs,
	// at which point the header should be sent.
	return types.HeaderStopIteration
}

func TrimQuote(source string) string {
	return strings.Trim(source, `"`)
}

// 定义一个动态string全局集合
var preQs = ""
var pre2Qs = ""

func onHttpRequestBody(ctx wrapper.HttpContext, config AIRagConfig, body []byte, log wrapper.Log) types.Action {
	bodyJson := gjson.ParseBytes(body)

	log.Infof("request body message:%s", bodyJson)

	rawContent := TrimQuote(bodyJson.Get(config.CacheKeyFrom.RequestBody).Raw)
	if rawContent == "" {
		log.Debugf("parse key from request body failed,cached:%s", config.CacheKeyFrom.RequestBody)
		return types.ActionContinue
	}

	log.Infof("request body message key:%s", rawContent)
	ctx.SetContext(CacheKeyContextKey, rawContent)

	actualKey := rawContent
	// 判断是否是完整的句子

	err := config.JieClient.Post(
		"/segment",
		[][2]string{{"Content-Type", "application/json"}},
		[]byte(fmt.Sprintf(`{"text":"%s"}`, rawContent)),
		func(statusCode int, responseHeaders http.Header, responseBody []byte) {
			res := string(responseBody)
			log.Infof("jieba response:%s", res)
			//转成bool
			if !strings.Contains(res, "true") {
				if preQs == "" {
					log.Infof("rawContent is not SentenceComplete and preQs is empty, key:%s", rawContent)
					return
				} else if pre2Qs == "" {
					actualKey = preQs + rawContent
				} else {
					actualKey = pre2Qs + rawContent
				}
			}
		},
	)
	if err != nil {
		log.Errorf("failed to post: %s", err)
		return types.ActionContinue
	}

	embedding.GetEmbedding(config.DashScopeClient, config.DashScopeAPIKey, actualKey, func(statusCode int, responseHeaders http.Header, responseBody []byte) {
		log.Infof("text-keyEmbedding,key:%s,status:%d", rawContent, statusCode)

		ebd, keyEmbedding := getEbd(responseBody)

		localQsVal := localCache.RetrieveBestAnswer(actualKey, keyEmbedding)
		if localQsVal != "" {
			ctx.SetContext(ToolCallsContextKey, struct{}{})
			proxywasm.SendHttpResponse(200, [][2]string{{"content-type", "text/event-stream; charset=utf-8"}}, []byte(fmt.Sprintf(config.ReturnResponseTemplate, localQsVal)), -1)
			return
		}

		log.Infof("localQsVal not exist, begin query embedding service")
		ctx.SetContext(CacheEmbeddingKey, ebd)
		embedding.QueryValByEmbeddingKey(config.DashVectorClient, config.DashVectorAPIKey, config.DashVectorCollection, keyEmbedding, func(statusCode int, responseHeaders http.Header, responseBody []byte) {
			var response dashvector.Response
			_ = json.Unmarshal(responseBody, &response)
			objects := response.Output
			log.Infof("QueryValByEmbeddingKey response:%d", len(objects))

			if len(objects) == 0 {
				log.Infof("ebd cache miss, key:%s", rawContent)
				newContent := config.CacheKeyFrom.Prefix + rawContent
				log.Infof("new content:%s", newContent)
				newBody, err := sjson.SetBytes(body, config.CacheKeyFrom.RequestBodyT, []byte(newContent))
				if err != nil {
					log.Errorf("Failed to set new value in JSON: %v", err)
				}
				// 替换请求体
				if err := proxywasm.ReplaceHttpRequestBody(newBody); err != nil {
					log.Errorf("Failed to replace HTTP request body: %v", err)
				}
				proxywasm.ResumeHttpRequest()
				return
			}

			doc := objects[0].Fields.Data
			score := objects[0].Score
			log.Infof("QueryValByEmbeddingKey response:%s,score:%f", doc, score)
			if score < 0.27 {
				// 计算答案的相似度
				log.Infof("vector cache hit, score: %f", score)
				ctx.SetContext(ToolCallsContextKey, struct{}{})
				proxywasm.SendHttpResponse(200, [][2]string{{"content-type", "text/event-stream; charset=utf-8"}}, []byte(fmt.Sprintf(config.ReturnResponseTemplate, doc)), -1)

			} else {
				log.Infof("cache miss, score:%f", score)
				newContent := config.CacheKeyFrom.Prefix + rawContent
				log.Infof("new content:%s", newContent)
				newBody, err := sjson.SetBytes(body, config.CacheKeyFrom.RequestBodyT, []byte(newContent))
				if err != nil {
					log.Errorf("Failed to set new value in JSON: %v", err)
				}
				// 替换请求体
				if err := proxywasm.ReplaceHttpRequestBody(newBody); err != nil {
					log.Errorf("Failed to replace HTTP request body: %v", err)
				}
				proxywasm.ResumeHttpRequest()
			}
		})

	})

	return types.ActionPause
}

func getEbd(responseBody []byte) (dashscope.Embedding, []float64) {
	var responseEmbedding dashscope.Response
	_ = json.Unmarshal(responseBody, &responseEmbedding)
	ebd := responseEmbedding.Output.Embeddings[0]
	keyEmbedding := ebd.Embedding
	return ebd, keyEmbedding
}

func processSSEMessage(ctx wrapper.HttpContext, config AIRagConfig, sseMessage string, log wrapper.Log) string {
	subMessages := strings.Split(sseMessage, "\n")
	var message string
	for _, msg := range subMessages {
		if strings.HasPrefix(msg, "data:") {
			message = msg
			break
		}
	}
	if len(message) < 6 {
		log.Errorf("invalid message:%s", message)
		return ""
	}
	// skip the prefix "data:"
	bodyJson := message[5:]
	if gjson.Get(bodyJson, config.CacheStreamValueFrom.ResponseBody).Exists() {
		tempContentI := ctx.GetContext(CacheContentContextKey)
		if tempContentI == nil {
			content := TrimQuote(gjson.Get(bodyJson, config.CacheStreamValueFrom.ResponseBody).Raw)
			ctx.SetContext(CacheContentContextKey, content)
			return content
		}
		append := TrimQuote(gjson.Get(bodyJson, config.CacheStreamValueFrom.ResponseBody).Raw)
		content := tempContentI.(string) + append
		ctx.SetContext(CacheContentContextKey, content)
		return content
	} else if gjson.Get(bodyJson, "choices.0.delta.content.tool_calls").Exists() {
		// TODO: compatible with other providers
		ctx.SetContext(ToolCallsContextKey, struct{}{})
		return ""
	}
	log.Debugf("unknown message:%s", bodyJson)
	return ""
}

func onHttpResponseHeaders(ctx wrapper.HttpContext, config AIRagConfig, log wrapper.Log) types.Action {
	contentType, _ := proxywasm.GetHttpResponseHeader("content-type")
	if strings.Contains(contentType, "text/event-stream") {
		ctx.SetContext(StreamContextKey, struct{}{})
	}
	return types.ActionContinue
}

func onHttpResponseBody(ctx wrapper.HttpContext, config AIRagConfig, chunk []byte, isLastChunk bool, log wrapper.Log) []byte {

	log.Info("onHttpResponseBody body message")

	if ctx.GetContext(ToolCallsContextKey) != nil {
		return chunk
	}

	keyI := ctx.GetContext(CacheKeyContextKey)
	if keyI == nil {
		return chunk
	}

	keyE := ctx.GetContext(CacheEmbeddingKey)
	if keyE == nil {
		return chunk
	}

	if !isLastChunk {
		stream := ctx.GetContext(StreamContextKey)
		if stream == nil {
			tempContentI := ctx.GetContext(CacheContentContextKey)
			if tempContentI == nil {
				ctx.SetContext(CacheContentContextKey, chunk)
				return chunk
			}
			tempContent := tempContentI.([]byte)
			tempContent = append(tempContent, chunk...)
			ctx.SetContext(CacheContentContextKey, tempContent)
		} else {
			var partialMessage []byte
			partialMessageI := ctx.GetContext(PartialMessageContextKey)
			if partialMessageI != nil {
				partialMessage = append(partialMessageI.([]byte), chunk...)
			} else {
				partialMessage = chunk
			}
			messages := strings.Split(string(partialMessage), "\n\n")
			for i, msg := range messages {
				if i < len(messages)-1 {
					// process complete message
					processSSEMessage(ctx, config, msg, log)
				}
			}
			if !strings.HasSuffix(string(partialMessage), "\n\n") {
				ctx.SetContext(PartialMessageContextKey, []byte(messages[len(messages)-1]))
			} else {
				ctx.SetContext(PartialMessageContextKey, nil)
			}
		}
		return chunk
	}

	log.Info("onHttpResponseBody body message 2")
	// last chunk
	question := keyI.(string)
	var answer string
	stream := ctx.GetContext(StreamContextKey)

	log.Info("onHttpResponseBody body message 3")

	if stream == nil {
		var body []byte
		tempContentI := ctx.GetContext(CacheContentContextKey)
		if tempContentI != nil {
			body = append(tempContentI.([]byte), chunk...)
		} else {
			body = chunk
		}
		bodyJson := gjson.ParseBytes(body)

		answer = TrimQuote(bodyJson.Get(config.CacheValueFrom.ResponseBody).Raw)
		if answer == "" {
			log.Warnf("parse answer from response body failded, body:%s", body)
			return chunk
		}
	} else {
		if len(chunk) > 0 {
			var lastMessage []byte
			partialMessageI := ctx.GetContext(PartialMessageContextKey)
			if partialMessageI != nil {
				lastMessage = append(partialMessageI.([]byte), chunk...)
			} else {
				lastMessage = chunk
			}
			if !strings.HasSuffix(string(lastMessage), "\n\n") {
				log.Warnf("invalid lastMessage:%s", lastMessage)
				return chunk
			}
			// remove the last \n\n
			lastMessage = lastMessage[:len(lastMessage)-2]
			answer = processSSEMessage(ctx, config, string(lastMessage), log)
		} else {
			tempContentI := ctx.GetContext(CacheContentContextKey)
			if tempContentI == nil {
				return chunk
			}
			answer = tempContentI.(string)
		}
	}

	log.Infof("onHttpResponseBody body message answer:%s 5", answer)
	keyEbd := keyE.(dashscope.Embedding)
	// 提取向量并创建Doc数组
	docs := make([]Doc, 1)
	rand.Seed(time.Now().UnixNano())
	docID := fmt.Sprintf("doc_%d", rand.Intn(90000)+10000) // 假设ID根据text_index生成

	embedding.GetEmbedding(config.DashScopeClient, config.DashScopeAPIKey, answer, func(statusCode int, responseHeaders http.Header, responseBody []byte) {
		_, answerVector := getEbd(responseBody)
		localCache.AddToCache(docID, question, preQs, answer, answerVector, keyEbd.Embedding)
	})

	docs[0] = Doc{
		ID:     docID,
		Vector: keyEbd.Embedding,
		FIELDS: Fields{
			DATA: answer,
		},
	}

	// 使用中间结构体序列化 docs 数组
	docsObj := Docs{Docs: docs}
	// 序列化JSON
	requestDocSerialized, _ := json.Marshal(docsObj)

	err := config.DashVectorClient.Post(
		fmt.Sprintf("/v1/collections/%s/docs", config.DashVectorCollection),
		[][2]string{{"Content-Type", "application/json"}, {"dashvector-auth-token", config.DashVectorAPIKey}},
		requestDocSerialized,
		func(statusCode int, responseHeaders http.Header, responseBody []byte) {
			log.Warnf("save doc,question:%s, body:%s", question, responseBody)
		},
		50000,
	)

	if err != nil {
		fmt.Printf("failed to post: %s", err)
	}

	pre2Qs = preQs
	preQs = question

	return chunk
}
