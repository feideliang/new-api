package middleware

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// ToolExtractorMiddleware extracts tool/skill/function names from the raw
// request body and stores them on the Gin context for consumption log
// recording. Runs before Distribute() so the body is still readable.
//
// AOP-style hook — no handler files are modified. The extracted data is
// consumed by appendToolInfo() in service/log_info_generate.go.
//
func ToolExtractorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			c.Next()
			return
		}
		requestBody, err := storage.Bytes()
		if err != nil || !gjson.ValidBytes(requestBody) {
			c.Next()
			return
		}

		names := extractToolNames(requestBody)
		if len(names) > 0 {
			common.SetContextKey(c, constant.ContextKeyOriginalTools, names)
		}

		c.Next()
	}
}

// extractToolNames parses tool/function names from the raw JSON body.
// Supports multiple request formats:
//   - Claude:  tools[].name
//   - OpenAI:  tools[].function.name
//   - OpenAI:  functions[].name
//   - generic: tools[].type
func extractToolNames(body []byte) []string {
	result := gjson.GetBytes(body, "tools")
	if !result.Exists() || !result.IsArray() {
		if fnResult := gjson.GetBytes(body, "functions"); fnResult.Exists() && fnResult.IsArray() {
			names := make([]string, 0, len(fnResult.Array()))
			for _, fn := range fnResult.Array() {
				if name := fn.Get("name").String(); name != "" {
					names = append(names, name)
				}
			}
			return names
		}
		return nil
	}

	names := make([]string, 0, len(result.Array()))
	for _, tool := range result.Array() {
		if !tool.IsObject() {
			continue
		}
		// Claude format: tools[].name
		if name := tool.Get("name").String(); name != "" {
			names = append(names, name)
			continue
		}
		// OpenAI format: tools[].function.name
		if fn := tool.Get("function"); fn.Exists() {
			if name := fn.Get("name").String(); name != "" {
				names = append(names, name)
				continue
			}
		}
		// Generic fallback: tools[].type
		if toolType := tool.Get("type").String(); toolType != "" {
			names = append(names, toolType)
		}
	}
	return names
}