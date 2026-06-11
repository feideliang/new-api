package middleware

import (
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// cladeCodeBuiltInTools lists Claude Code / LLM framework built-in tool
// names that should be excluded from logging. Only user-defined skills/tools
// are meant to be tracked.
var cladeCodeBuiltInTools = map[string]struct{}{
	// Claude Code tools
	"Agent":             {},
	"AskUserQuestion":   {},
	"Bash":              {},
	"CronCreate":        {},
	"CronDelete":        {},
	"CronList":          {},
	"Edit":              {},
	"EnterPlanMode":     {},
	"EnterWorktree":     {},
	"ExitPlanMode":      {},
	"ExitWorktree":      {},
	"Glob":              {},
	"Grep":              {},
	"LSP":               {},
	"Monitor":           {},
	"NotebookEdit":      {},
	"PushNotification":  {},
	"Read":              {},
	"ScheduleWakeup":    {},
	"SendMessage":       {},
	"Skill":             {},
	"TaskCreate":        {},
	"TaskGet":           {},
	"TaskList":          {},
	"TaskOutput":        {},
	"TaskStop":          {},
	"TaskUpdate":        {},
	"TeamCreate":        {},
	"TeamDelete":        {},
	"TeamUpdate":        {},
	"WebFetch":          {},
	"WebSearch":         {},
	"Workflow":          {},
	"Write":             {},
	// Common LLM framework tools
	"code_interpreter": {},
	"browser":          {},
	"file_search":      {},
	"python":           {},
}

// skillNameRegex matches /skill-name or /namespace:skill-name patterns in text.
// Excludes file paths (containing another /) and common non-skill prefixes.
var skillNameRegex = regexp.MustCompile(`(?:^|\s)/([a-zA-Z][a-zA-Z0-9_:.-]{1,80})(?:$|\s|[.,!?;])`)

// nonSkillPrefixes lists words that appear after / but are not skill names
// (file paths, common commands, etc.)
var nonSkillPrefixes = map[string]struct{}{
	"Users": {}, "workspace": {}, "dev": {}, "opt": {}, "home": {},
	"etc": {}, "usr": {}, "tmp": {}, "var": {}, "bin": {}, "lib": {},
	"sbin": {}, "System": {}, "Library": {}, "Applications": {},
	"dl": {}, "root": {}, "mnt": {}, "media": {}, "Volumes": {},
	"v1": {}, "v2": {}, "api": {}, "swagger": {},
	"model": {}, "help": {}, "clear": {},
}

// ToolExtractorMiddleware extracts tool/skill/function names from the raw
// request body and stores them on the Gin context for consumption log
// recording. Runs before Distribute() so the body is still readable.
//
// Extraction sources (merged, deduplicated):
//  1. tools[] / functions[] arrays (Claude Code tool definitions, filtered)
//  2. messages[].content — /skill-name slash command patterns
//  3. system field — skill metadata (name: skill-name) from skill content
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

// extractToolNames parses tool/skill/function names from the raw JSON body.
// Merges results from tools[] definitions, messages[] content, and system field.
func extractToolNames(body []byte) []string {
	seen := make(map[string]struct{})

	// Source 1: tools[] / functions[] arrays
	for _, name := range extractFromTools(body) {
		if name != "" && !isClaudeBuiltIn(name) {
			seen[name] = struct{}{}
		}
	}

	// Source 2: messages[].content — /skill-name slash commands
	for _, name := range extractFromMessages(body) {
		if name != "" && !isClaudeBuiltIn(name) {
			seen[name] = struct{}{}
		}
	}

	// Source 3: system field — skill metadata like "name: writing-plans"
	for _, name := range extractFromSystem(body) {
		if name != "" && !isClaudeBuiltIn(name) {
			seen[name] = struct{}{}
		}
	}

	if len(seen) == 0 {
		return nil
	}
	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	return result
}

// extractFromTools reads tool names from tools[] / functions[] arrays.
//   - Claude:  tools[].name
//   - OpenAI:  tools[].function.name, functions[].name
//   - generic: tools[].type
func extractFromTools(body []byte) []string {
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

// extractFromMessages reads skill names from user message content.
// Looks for /skill-name or /namespace:skill-name patterns in messages[].
// Supports both Claude format (content string/array) and OpenAI format.
func extractFromMessages(body []byte) []string {
	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return nil
	}

	seen := make(map[string]struct{})
	for _, msg := range messages.Array() {
		if !msg.IsObject() {
			continue
		}
		role := msg.Get("role").String()
		// Only check user messages for slash commands
		if role != "user" {
			continue
		}

		content := msg.Get("content")
		texts := extractTextFromContent(content)
		for _, text := range texts {
			for _, match := range skillNameRegex.FindAllStringSubmatch(text, -1) {
				if len(match) >= 2 {
					name := match[1]
					if !isPathPrefix(name) {
						seen[name] = struct{}{}
					}
				}
			}
		}
	}

	if len(seen) == 0 {
		return nil
	}
	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	return result
}

// extractFromSystem reads skill names from the system field.
// Skill content loaded by Claude Code contains metadata like:
//
//	---\nname: writing-plans\ndescription: ...\n---
//
// Also handles simple /skill-name in system text.
func extractFromSystem(body []byte) []string {
	system := gjson.GetBytes(body, "system")
	if !system.Exists() {
		return nil
	}

	texts := extractTextFromContent(system)
	seen := make(map[string]struct{})

	// Pattern 1: "name: SKILL-NAME" in skill metadata blocks
	namePattern := regexp.MustCompile(`(?m)^name:\s*([a-zA-Z][a-zA-Z0-9_:.-]+)$`)
	for _, text := range texts {
		for _, match := range namePattern.FindAllStringSubmatch(text, -1) {
			if len(match) >= 2 {
				name := match[1]
				if !isClaudeBuiltIn(name) && !isPathPrefix(name) {
					seen[name] = struct{}{}
				}
			}
		}
		// Pattern 2: /skill-name in system text
		for _, match := range skillNameRegex.FindAllStringSubmatch(text, -1) {
			if len(match) >= 2 {
				name := match[1]
				if !isPathPrefix(name) {
					seen[name] = struct{}{}
				}
			}
		}
	}

	if len(seen) == 0 {
		return nil
	}
	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	return result
}

// extractTextFromContent extracts plain text from a JSON content field.
// Handles:
//   - string (direct text)
//   - array of content blocks (Claude format)
//   - array of message content items (OpenAI format)
func extractTextFromContent(content gjson.Result) []string {
	if !content.Exists() {
		return nil
	}

	// Direct string
	if content.Type == gjson.String {
		return []string{content.String()}
	}

	// Array of content blocks
	if content.IsArray() {
		var texts []string
		for _, item := range content.Array() {
			if !item.IsObject() {
				continue
			}
			itemType := item.Get("type").String()
			switch itemType {
			case "text":
				texts = append(texts, item.Get("text").String())
			case "tool_result", "tool_use":
				// For tool_result, check nested content
				if inner := item.Get("content"); inner.Exists() {
					texts = append(texts, extractTextFromContent(inner)...)
				}
			default:
				// Unknown type — try text field directly
				if txt := item.Get("text").String(); txt != "" {
					texts = append(texts, txt)
				}
			}
		}
		return texts
	}

	return nil
}

// isClaudeBuiltIn returns true if the name is a known Claude Code / LLM
// framework built-in tool.
func isClaudeBuiltIn(name string) bool {
	_, ok := cladeCodeBuiltInTools[name]
	return ok
}

// isPathPrefix returns true if name looks like a file path component rather
// than a skill name (e.g., "Users", "workspace", "dl").
func isPathPrefix(name string) bool {
	_, ok := nonSkillPrefixes[name]
	if ok {
		return true
	}
	// If the name contains a dot and looks like a file extension, skip it
	if strings.Contains(name, ".") && !strings.Contains(name, ":") {
		return true
	}
	return false
}