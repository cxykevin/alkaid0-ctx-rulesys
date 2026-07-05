package rulesys

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// --- 单元测试 ---

func TestPurePrompt(t *testing.T) {
	input := "axios拦截器怎么取消"
	segs := SplitString(input)
	if len(segs) != 1 {
		t.Errorf("expected 1 segment, got %d", len(segs))
	}
	if len(segs) > 0 && segs[0].Type != SegmentPrompt {
		t.Errorf("expected prompt, got %s", segs[0].Type)
	}
}

func TestPromptCode(t *testing.T) {
	input := "fix this performance bottleneck\n\nfunction processData(data) {\n  let result = [];\n  return result;\n}"
	segs := SplitString(input)
	if len(segs) < 2 {
		t.Errorf("expected at least 2 segments, got %d: %+v", len(segs), segs)
	}
	if len(segs) >= 1 && segs[0].Type != SegmentPrompt {
		t.Errorf("segment 0 expected prompt, got %s: %q", segs[0].Type, segs[0].Content)
	}
	if len(segs) >= 2 && segs[1].Type != SegmentCode {
		t.Errorf("segment 1 expected code, got %s: %q", segs[1].Type, segs[1].Content)
	}
}

func TestPromptLog(t *testing.T) {
	input := "fix this port conflict\n\nError: listen EADDRINUSE: address already in use :::3000\n    at Server.setupListenHandle (net.js:1330:16)"
	segs := SplitString(input)
	if len(segs) < 2 {
		t.Errorf("expected at least 2 segments, got %d: %+v", len(segs), segs)
	}
	if len(segs) >= 1 && segs[0].Type != SegmentPrompt {
		t.Errorf("segment 0 expected prompt, got %s: %q", segs[0].Type, segs[0].Content)
	}
	if len(segs) >= 2 && segs[1].Type != SegmentLog {
		t.Errorf("segment 1 expected log, got %s: %q", segs[1].Type, segs[1].Content)
	}
}

func TestPromptCodeLog(t *testing.T) {
	input := "fix this\n\nfunction processData(data) {\n    const result = data.map(item => {\n        return item;\n    });\n    return result;\n}"
	segs := SplitString(input)
	if len(segs) < 2 {
		t.Errorf("expected at least 2 segments, got %d: %+v", len(segs), segs)
	}
	if len(segs) >= 1 && segs[0].Type != SegmentPrompt {
		t.Errorf("segment 0 expected prompt, got %s", segs[0].Type)
	}
	if len(segs) >= 2 && segs[1].Type != SegmentCode {
		t.Errorf("segment 1 expected code, got %s", segs[1].Type)
	}
}

func TestMultiCodeSegments(t *testing.T) {
	input := "fix this lint error\n\nfunction fetchData() {\n  const data = await fetch('/api/data');\n  return data.json();\n}\n\nconsole.log('done')"
	segs := SplitString(input)
	codeCount := 0
	for _, s := range segs {
		if s.Type == SegmentCode {
			codeCount++
		}
	}
	if codeCount < 1 {
		t.Errorf("expected at least 1 code segment, got %d: %+v", codeCount, segs)
	}
}

func TestFencedCodeBlock(t *testing.T) {
	input := "fix this\n\n```python\ndef hello():\n    print('hello')\n```\n\nthis is more text"
	segs := SplitString(input)
	if len(segs) < 3 {
		t.Errorf("expected at least 3 segments (prompt, code, prompt), got %d: %+v", len(segs), segs)
	} else {
		if segs[0].Type != SegmentPrompt {
			t.Errorf("segment 0 expected prompt, got %s", segs[0].Type)
		}
		if segs[1].Type != SegmentCode {
			t.Errorf("segment 1 expected code, got %s", segs[1].Type)
		}
		if !strings.Contains(segs[1].Content, "```") {
			t.Errorf("code segment should include fence markers: %q", segs[1].Content)
		}
	}
}

func TestErrFuncEdgeCase(t *testing.T) {
	// "errfunc main()" 不应被误判为 log
	input := "fix this\n\nerrfunc main() {}"
	segs := SplitString(input)
	if len(segs) >= 2 && segs[1].Type == SegmentLog {
		t.Errorf("'errfunc main()' should be code, not log: %+v", segs)
	}
}

func TestDockerBuildLog(t *testing.T) {
	input := "docker build fails\n\nStep 5/10 : RUN apt-get update\n ---> abc123\nE: Unable to locate package"
	segs := SplitString(input)
	logFound := false
	for _, s := range segs {
		if s.Type == SegmentLog {
			logFound = true
			break
		}
	}
	if !logFound {
		t.Errorf("expected a log segment for docker build output: %+v", segs)
	}
}

func TestCIError(t *testing.T) {
	input := "看下这个CI/CD报错\n\nERROR: Job failed: exit code 1\nStage: build\nFailed to compile."
	segs := SplitString(input)
	logFound := false
	for _, s := range segs {
		if s.Type == SegmentLog {
			logFound = true
			break
		}
	}
	if !logFound {
		t.Errorf("expected a log segment for CI error: %+v", segs)
	}
}

func TestStackTrace(t *testing.T) {
	input := "fix this error\n\nError: Cannot read properties of undefined (reading 'map')\n    at processData (/app/src/utils.js:15:23)\n    at handleRequest (/app/src/handler.js:42:5)"
	segs := SplitString(input)
	logFound := false
	for _, s := range segs {
		if s.Type == SegmentLog && strings.Contains(s.Content, "Error:") {
			logFound = true
			break
		}
	}
	if !logFound {
		t.Errorf("expected log segment with stack trace: %+v", segs)
	}
}

func TestJavaCode(t *testing.T) {
	input := "fix this java code\n\npublic class HelloWorld {\n    public static void main(String[] args) {\n        System.out.println(\"Hello\");\n    }\n}"
	segs := SplitString(input)
	codeFound := false
	for _, s := range segs {
		if s.Type == SegmentCode {
			codeFound = true
			break
		}
	}
	if !codeFound {
		t.Errorf("expected code segment for Java code: %+v", segs)
	}
}

func TestSQLCode(t *testing.T) {
	input := "optimize this query\n\nSELECT u.id, u.name, o.total\nFROM users u\nJOIN orders o ON u.id = o.user_id\nWHERE u.status = 'active'"
	segs := SplitString(input)
	codeFound := false
	for _, s := range segs {
		if s.Type == SegmentCode {
			codeFound = true
			break
		}
	}
	if !codeFound {
		t.Errorf("expected code segment for SQL: %+v", segs)
	}
}

func TestDockerfileCode(t *testing.T) {
	input := "fix this Dockerfile\n\nFROM node:14\nWORKDIR /app\nCOPY package*.json ./\nRUN npm install\nCOPY . .\nEXPOSE 3000"
	segs := SplitString(input)
	codeFound := false
	for _, s := range segs {
		if s.Type == SegmentCode {
			codeFound = true
			break
		}
	}
	if !codeFound {
		t.Errorf("expected code segment for Dockerfile: %+v", segs)
	}
}

func TestNginxConfig(t *testing.T) {
	input := "check this nginx config\n\nserver {\n    listen 80;\n    server_name example.com;\n}"
	segs := SplitString(input)
	codeFound := false
	for _, s := range segs {
		if s.Type == SegmentCode {
			codeFound = true
			break
		}
	}
	if !codeFound {
		t.Errorf("expected code segment for nginx config: %+v", segs)
	}
}

func TestCommentNotLog(t *testing.T) {
	// // 注释行不应该将代码段误切换为 log 段
	input := "fix this\n\nfunction processData(data) {\n    // process each item\n    const result = data.map(item => {\n        return item.value * 2;\n    });\n    return result;\n}"
	segs := SplitString(input)
	for _, s := range segs {
		if s.Type == SegmentLog && strings.Contains(s.Content, "process each item") {
			t.Errorf("code comment should not be classified as log: %+v", segs)
		}
	}
}

func TestStructFieldWithPointer(t *testing.T) {
	// Struct fields with pointer types (e.g. entry *LogEntry) should be code
	input := "修bug：\nfunc TestShouldDisplay(t *testing.T) {\n        tests := []struct {\n                name     string\n                entry    *LogEntry\n                minLevel string\n                expected bool\n        }{\n                {\n                        name: \"DEBUG with DEBUG min\",\n                        entry: &LogEntry{\n                                Level: \"DEBUG\",\n                        },\n                        minLevel: \"DEBUG\",\n                        expected: true,\n                },\n        }\n}"
	segs := SplitString(input)
	if len(segs) < 2 {
		t.Errorf("expected at least 2 segments, got %d: %+v", len(segs), segs)
	}
	if len(segs) >= 1 && segs[0].Type != SegmentPrompt {
		t.Errorf("segment 0 expected prompt, got %s: %q", segs[0].Type, segs[0].Content)
	}
	codeFound := false
	for _, s := range segs {
		if s.Type == SegmentCode {
			codeFound = true
			break
		}
	}
	if !codeFound {
		t.Errorf("expected a code segment: %+v", segs)
	}
	entryFoundAsPrompt := false
	for _, s := range segs {
		if s.Type == SegmentPrompt && strings.Contains(s.Content, "*LogEntry") {
			entryFoundAsPrompt = true
		}
	}
	if entryFoundAsPrompt {
		t.Errorf("'*LogEntry' should be in code segment, not prompt: %+v", segs)
	}
}

func TestJSONWithChineseTail(t *testing.T) {
	input := "{\n  \"files\": [],\n  \"references\": [\n    { \"path\": \"./tsconfig.app.json\" },\n    { \"path\": \"./tsconfig.node.json\" }\n  ]\n}\n解释一下"
	segs := SplitString(input)
	codeFound := false
	promptChinese := false
	for _, s := range segs {
		if s.Type == SegmentCode {
			codeFound = true
		}
		if s.Type == SegmentPrompt && strings.Contains(s.Content, "解释一下") {
			promptChinese = true
		}
	}
	if !codeFound {
		t.Errorf("expected a code segment for JSON: %+v", segs)
	}
	if !promptChinese {
		t.Errorf("expected '解释一下' in a prompt segment: %+v", segs)
	}
}

func TestCodeSegmentEndingWithChinese(t *testing.T) {
	input := "{\n                        name: \"INFO with DEBUG min\",\n                        entry: &LogEntry{\n                                Level: \"INFO\",\n                        },这是什么"
	segs := SplitString(input)
	chineseInPrompt := false
	for _, s := range segs {
		if s.Type == SegmentPrompt && strings.Contains(s.Content, "这是什么") {
			chineseInPrompt = true
		}
	}
	if !chineseInPrompt {
		t.Errorf("expected '这是什么' in a prompt segment: %+v", segs)
	}
}

func TestColonFollowedByQuote(t *testing.T) {
	input := "line:\"2025/12/07 14:04:35 [INFO][log] log inited\","
	segs := SplitString(input)
	if len(segs) != 1 {
		t.Errorf("expected single code segment, got %d: %+v", len(segs), segs)
	}
	if len(segs) == 1 && segs[0].Type != SegmentCode {
		t.Errorf("expected code, got %s: %q", segs[0].Type, segs[0].Content)
	}
}

// --- 批量评估 ---

type testRecord struct {
	Original string `json:"original"`
	Tagged   string `json:"tagged"`
}

func loadTestData(t *testing.T, path string) []testRecord {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("test data not found at %s: %v", path, err)
		return nil
	}
	var records []testRecord
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rec testRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Logf("failed to parse line: %v", err)
			continue
		}
		records = append(records, rec)
	}
	return records
}

// parseTagged 从 tagged 字符串中提取 ground truth 分段
type groundTruth struct {
	Type    SegmentType
	Content string
}

func parseTagged(tagged string) []groundTruth {
	var gts []groundTruth
	// 格式: <|tag_prompt_start|>content<|tag_prompt_end|>
	//       <|tag_code_start|>content<|tag_code_end|>
	//       <|tag_log_start|>content<|tag_log_end|>
	//       <|tag_info_start|>content<|tag_info_end|>
	//       <|tag_rule_start|>content<|tag_rule_end|>
	//       <|tag_link_start|>content<|tag_link_end|>
	pos := 0
	for pos < len(tagged) {
		// 查找下一个标签
		startTag := ""
		var segType SegmentType
		tagNames := []struct {
			name string
			st   SegmentType
		}{
			{"<|tag_prompt_start|>", SegmentPrompt},
			{"<|tag_code_start|>", SegmentCode},
			{"<|tag_log_start|>", SegmentLog},
			{"<|tag_info_start|>", SegmentPrompt},   // info → prompt
			{"<|tag_rule_start|>", SegmentPrompt},   // rule → prompt
			{"<|tag_link_start|>", SegmentPrompt},   // link → prompt
		}

		bestIdx := -1
		for _, tn := range tagNames {
			idx := strings.Index(tagged[pos:], tn.name)
			if idx >= 0 && (bestIdx < 0 || idx < bestIdx) {
				bestIdx = idx
				startTag = tn.name
				segType = tn.st
			}
		}

		if bestIdx < 0 {
			break
		}

		// 跳过起始标签
		contentStart := pos + bestIdx + len(startTag)

		// 查找对应的结束标签
		endTag := strings.Replace(startTag, "_start|>", "_end|>", 1)
		contentEnd := strings.Index(tagged[contentStart:], endTag)
		if contentEnd < 0 {
			break
		}

		content := tagged[contentStart : contentStart+contentEnd]
		gts = append(gts, groundTruth{Type: segType, Content: content})

		pos = contentStart + contentEnd + len(endTag)
	}
	return gts
}

// groundTruthType 将 groundTruth 的 Type 转为字符串
func (gt groundTruth) TypeStr() string {
	switch gt.Type {
	case SegmentPrompt:
		return "prompt"
	case SegmentCode:
		return "code"
	case SegmentLog:
		return "log"
	default:
		return "unknown"
	}
}

// String 实现 SegmentType 序列化
func (st SegmentType) MarshalText() ([]byte, error) {
	return []byte(st.String()), nil
}

func TestBatchEvaluate(t *testing.T) {
	records := loadTestData(t, "../testdata.jsonl")
	if records == nil {
		return
	}

	totalCorrect := 0
	totalChars := 0
	promptCorrect := 0
	codeCorrect := 0
	logCorrect := 0
	promptTotal := 0
	codeTotal := 0
	logTotal := 0
	errorCases := 0

	type typMatch struct {
		pred SegmentType
		gt   SegmentType
	}

	for i, rec := range records {
		// 跳过太短的输入
		if len(rec.Original) < 3 {
			continue
		}

		gts := parseTagged(rec.Tagged)
		if len(gts) == 0 {
			continue
		}

		// 运行引擎
		segs := SplitString(rec.Original)

		// 字符级匹配：对每个字符检查预测类型是否匹配 ground truth
		// 构建 ground truth 的类型映射
		gtTypes := make([]SegmentType, len(rec.Original))
		for i := range gtTypes {
			gtTypes[i] = SegmentPrompt // 默认 prompt
		}
		gtPos := 0
		for _, gt := range gts {
			// 在 original 中查找 gt.Content 的近似位置
			// 注意：ground truth 的内容可能不完全匹配 original（去除了空行等）
			start := strings.Index(rec.Original[gtPos:], trimContentForMatch(gt.Content))
			if start < 0 {
				continue
			}
			start += gtPos
			end := start + len(trimContentForMatch(gt.Content))
			if end > len(rec.Original) {
				end = len(rec.Original)
			}
			for j := start; j < end; j++ {
				gtTypes[j] = gt.Type
			}
			gtPos = end
		}

		// 构建预测的类型映射
		predTypes := make([]SegmentType, len(rec.Original))
		for i := range predTypes {
			predTypes[i] = SegmentPrompt // 默认 prompt
		}
		predPos := 0
		for _, seg := range segs {
			if predPos >= len(rec.Original) {
				break
			}
			trimmed := trimContentForMatch(seg.Content)
			start := strings.Index(rec.Original[predPos:], trimmed)
			if start < 0 {
				start = strings.Index(rec.Original[predPos:], seg.Content)
				if start < 0 {
					predPos += len(seg.Content)
					if predPos > len(rec.Original) {
						predPos = len(rec.Original)
					}
					continue
				}
			}
			start += predPos
			end := start + len(seg.Content)
			if end > len(rec.Original) {
				end = len(rec.Original)
			}
			if start < end {
				for j := start; j < end; j++ {
					predTypes[j] = seg.Type
				}
			}
			predPos = end
		}

		// 字符级正确率统计
		charsCorrect := 0
		pTypeCorrect := 0
		cTypeCorrect := 0
		lTypeCorrect := 0
		pTypeTotal := 0
		cTypeTotal := 0
		lTypeTotal := 0

		for j := 0; j < len(rec.Original); j++ {
			if gtTypes[j] == SegmentPrompt {
				pTypeTotal++
			} else if gtTypes[j] == SegmentCode {
				cTypeTotal++
			} else {
				lTypeTotal++
			}

			if predTypes[j] == gtTypes[j] {
				charsCorrect++
				if gtTypes[j] == SegmentPrompt {
					pTypeCorrect++
				} else if gtTypes[j] == SegmentCode {
					cTypeCorrect++
				} else {
					lTypeCorrect++
				}
			}
		}

		charsAccuracy := float64(charsCorrect) / float64(len(rec.Original))
		totalCorrect += charsCorrect
		totalChars += len(rec.Original)
		promptCorrect += pTypeCorrect
		codeCorrect += cTypeCorrect
		logCorrect += lTypeCorrect
		promptTotal += pTypeTotal
		codeTotal += cTypeTotal
		logTotal += lTypeTotal

		// 找出低准确率的样本
		if charsAccuracy < 0.5 {
			errorCases++
			if errorCases <= 5 {
				t.Logf("⚠️  Record %d accuracy=%.1f%%, input=%q",
					i, charsAccuracy*100, truncateString(rec.Original, 80))
				t.Logf("    expected types: %s", summarizeTypes(rec.Original, gtTypes))
				t.Logf("    predicted types: %s", summarizeTypes(rec.Original, predTypes))
			}
		}
	}

	overallAccuracy := float64(totalCorrect) / float64(totalChars)
	promptPrecision := float64(promptCorrect) / float64(max(1, promptTotal))
	codePrecision := float64(codeCorrect) / float64(max(1, codeTotal))
	logPrecision := float64(logCorrect) / float64(max(1, logTotal))

	t.Logf("=== 批量评估结果（%d 样本）===", len(records))
	t.Logf("整体字符级准确率: %.2f%%", overallAccuracy*100)
	t.Logf("Prompt 准确率: %.2f%% (%d/%d)", promptPrecision*100, promptCorrect, promptTotal)
	t.Logf("Code   准确率: %.2f%% (%d/%d)", codePrecision*100, codeCorrect, codeTotal)
	t.Logf("Log    准确率: %.2f%% (%d/%d)", logPrecision*100, logCorrect, logTotal)
	t.Logf("低准确率样本数: %d", errorCases)
}

// trimContentForMatch 简化内容以便对比
func trimContentForMatch(s string) string {
	return strings.TrimSpace(s)
}

// truncateString 截断字符串到指定长度
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// summarizeTypes 生成类型概览字符串
func summarizeTypes(original string, types []SegmentType) string {
	if len(original) == 0 {
		return ""
	}
	type SummarySegment struct {
		Type SegmentType
		Len  int
		Text string
	}
	var segs []SummarySegment
	current := types[0]
	start := 0
	for i := 1; i <= len(types); i++ {
		var t SegmentType
		if i < len(types) {
			t = types[i]
		} else {
			t = -1 // sentinel
		}
		if t != current || i == len(types) {
			text := original[start:i]
			segs = append(segs, SummarySegment{Type: current, Len: i - start, Text: text})
			current = t
			start = i
		}
	}
	var parts []string
	for _, s := range segs {
		label := s.Type.String()
		text := strings.ReplaceAll(s.Text, "\n", "↵")
		if len(text) > 30 {
			text = text[:30] + "..."
		}
		parts = append(parts, label+"("+text+")")
	}
	return strings.Join(parts, " → ")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
