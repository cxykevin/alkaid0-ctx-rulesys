package rulesys

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// segmentFragment 表示行内分割后的一个片段
type segmentFragment struct {
	content string
	segType SegmentType
}

// 行内分割最小长度常量
const (
	minSegmentSplitLen = 5  // segment 最小长度才触发行内检测
	minLineSplitLen    = 10 // 单行最小长度才触发行内检测
	minPrefixLen       = 2  // 前缀最小长度
	minSuffixLen       = 3  // 后缀最小长度
)

// splitIntraLine 在所有 segment 上执行行内分割后处理
func splitIntraLine(segments []Segment, threshold int) []Segment {
	if threshold <= 0 {
		threshold = 12
	}
	if len(segments) == 0 {
		return segments
	}

	var result []Segment
	for _, seg := range segments {
		sub := splitSegmentAtBoundaries(seg, threshold)
		result = append(result, sub...)
	}
	return result
}

// splitSegmentAtBoundaries 检查单个 segment 是否包含混合类型的行
func splitSegmentAtBoundaries(seg Segment, threshold int) []Segment {
	content := seg.Content
	if len(content) < minSegmentSplitLen {
		return []Segment{seg}
	}

	lines := strings.Split(content, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) < minLineSplitLen {
			continue
		}

		fragments := detectMixedLine(trimmed, seg.Type, threshold)
		if len(fragments) < 2 {
			continue
		}

		// 在此行找到边界 → 拆分段
		return buildSplitSegments(seg, lines, i, fragments)
	}

	return []Segment{seg}
}

// detectMixedLine 检测单行中是否存在类型边界
// 返回 nil 表示未找到边界
func detectMixedLine(line string, segType SegmentType, threshold int) []segmentFragment {
	// 如果行包含代码围栏标记，不分割
	if isCodeFence(strings.TrimSpace(line)) {
		return nil
	}

	// 对 log segment 只做显式分隔短语检测（如 "错误信息:"）
	// 防止整行被误判为 log 的场景
	if segType == SegmentLog {
		return detectExplicitDelimiter(line, threshold)
	}

	// 只对 prompt 和 code 类型的 segment 进行完整分割
	if segType != SegmentPrompt && segType != SegmentCode {
		return nil
	}

	// 优先级 1：显式分隔短语
	if result := detectExplicitDelimiter(line, threshold); result != nil {
		return result
	}

	// 优先级 2：冒号边界
	if result := detectColonBoundary(line, segType, threshold); result != nil {
		return result
	}

	// 优先级 3：句尾标点边界
	if result := detectPunctuationBoundary(line, segType, threshold); result != nil {
		return result
	}

	// 优先级 4：词法边界（无标点的代码关键词转换）
	if result := detectCodeKeywordBoundary(line, segType, threshold); result != nil {
		return result
	}

	return nil
}

// ─── 显式分隔短语 ──────────────────────────────────────────────

// codeDelimiters 指示后面内容为代码的分隔短语
var codeDelimiters = []string{
	"here's the code:",
	"here is the code:",
	"here's my code:",
	"here is my code:",
	"here it is:",
	"the code is:",
	"the code:",
	"代码如下：",
	"代码是：",
	"代码是这样的：",
	"代码：",
	"这段代码：",
	"参考代码：",
	"示例代码：",
	"代码片段：",
	"代码片段:",
}

// logDelimiters 指示后面内容为日志/错误的分隔短语
var logDelimiters = []string{
	"报错信息：",
	"报错：",
	"错误信息：",
	"错误：",
	"error message:",
	"error:",
	"报错如下：",
	"错误如下：",
	"日志：",
	"日志如下：",
	"错误日志：",
}

// detectExplicitDelimiter 查找已知的分隔短语
func detectExplicitDelimiter(line string, threshold int) []segmentFragment {
	lower := strings.ToLower(line)

	// 检查代码分隔短语
	for _, phrase := range codeDelimiters {
		idx := strings.Index(lower, strings.ToLower(phrase))
		if idx < 0 {
			continue
		}
		// 短语必须在词边界处（行首或前面是空格/标点）
		if idx > 0 {
			prev, _ := utf8.DecodeLastRuneInString(line[:idx])
			if !isWordBoundary(prev) {
				continue
			}
		}

		suffix := strings.TrimSpace(line[idx+len(phrase):])
		if len(suffix) < minSuffixLen {
			continue
		}

		// 验证后缀确实像代码
		cl := classifyLine(suffix)
		if cl.codeScore >= threshold || cl.logScore >= threshold {
			suffixType := SegmentCode
			if cl.logScore > cl.codeScore {
				suffixType = SegmentLog
			}
			prefix := strings.TrimRight(line[:idx+len(phrase)], " ")
			return []segmentFragment{
				{content: prefix, segType: SegmentPrompt},
				{content: suffix, segType: suffixType},
			}
		}
	}

	// 检查日志分隔短语
	for _, phrase := range logDelimiters {
		idx := strings.Index(lower, strings.ToLower(phrase))
		if idx < 0 {
			continue
		}
		if idx > 0 {
			prev, _ := utf8.DecodeLastRuneInString(line[:idx])
			if !isWordBoundary(prev) {
				continue
			}
		}

		suffix := strings.TrimSpace(line[idx+len(phrase):])
		if len(suffix) < minSuffixLen {
			continue
		}

		prefixRaw := strings.TrimRight(line[:idx+len(phrase)], " ")
		
		// 验证前缀本身不是日志/代码内容（防止 "Error:" 行被误分割）
		prefixCL := classifyLine(prefixRaw)
		if prefixCL.logScore >= threshold || prefixCL.codeScore >= threshold {
			continue
		}
		
		cl := classifyLine(suffix)
		if cl.logScore >= threshold || cl.codeScore >= threshold {
			suffixType := SegmentLog
			if cl.codeScore > cl.logScore && cl.logScore < threshold {
				suffixType = SegmentCode
			}
			return []segmentFragment{
				{content: prefixRaw, segType: SegmentPrompt},
				{content: suffix, segType: suffixType},
			}
		}
	}

	return nil
}

// ─── 冒号边界 ───────────────────────────────────────────────────

// detectColonBoundary 查找 : 或 ：分隔的边界
func detectColonBoundary(line string, segType SegmentType, threshold int) []segmentFragment {
	// 收集所有冒号位置（: U+003A 和 ：U+FF1A）
	var colons []struct {
		idx  int
		char rune
	}
	for i, r := range line {
		if r == ':' || r == '：' {
			colons = append(colons, struct {
				idx  int
				char rune
			}{i, r})
		}
	}

	for _, c := range colons {
		// URL 中的冒号不处理（http:// https://）
		if c.char == ':' && c.idx >= 4 {
			if line[c.idx-4:c.idx+1] == "http:" {
				continue
			}
			if c.idx >= 5 && line[c.idx-5:c.idx+1] == "https:" {
				continue
			}
		}
		// 时间戳中的冒号（如 12:00:00）
		if c.char == ':' && c.idx >= 2 && c.idx+2 < len(line) {
			if isDigit(rune(line[c.idx-1])) && isDigit(rune(line[c.idx-2])) &&
				isDigit(rune(line[c.idx+1])) && isDigit(rune(line[c.idx+2])) {
				continue
			}
		}
		// 行首就是冒号模式（如 C: 盘符）
		if c.idx == 1 && unicode.IsLetter(rune(line[0])) {
			continue
		}

		prefix := strings.TrimSpace(line[:c.idx])
		suffix := strings.TrimSpace(line[c.idx+len(string(c.char)):])

		if len(prefix) < minPrefixLen || len(suffix) < minSuffixLen {
			continue
		}

		// 前缀验证：前缀不应该像代码或日志
		prefixCL := classifyLine(prefix)
		if prefixCL.codeScore >= threshold || prefixCL.logScore >= threshold {
			continue
		}

		// 后缀验证：后缀应该像代码或日志
		suffixCL := classifyLine(suffix)
		if suffixCL.codeScore >= threshold || suffixCL.logScore >= threshold {
			prefixContent := strings.TrimRight(line[:c.idx+len(string(c.char))], " ")
			suffixType := SegmentCode
			if suffixCL.logScore > suffixCL.codeScore {
				suffixType = SegmentLog
			}
			return []segmentFragment{
				{content: prefixContent, segType: SegmentPrompt},
				{content: suffix, segType: suffixType},
			}
		}
	}

	return nil
}

// ─── 句尾标点边界 ───────────────────────────────────────────────

// sentenceEndPunctuation 句尾标点集合
var sentenceEndPunctuation = map[rune]bool{
	'.': true, '。': true,
	'!': true, '！': true,
	'?': true, '？': true,
}

// detectPunctuationBoundary 查找句尾标点后的代码/日志
func detectPunctuationBoundary(line string, segType SegmentType, threshold int) []segmentFragment {
	runes := []rune(line)

	for i, r := range runes {
		if !sentenceEndPunctuation[r] {
			continue
		}

		// 确保后面有内容
		if i+1 >= len(runes) {
			continue
		}

		// 跳过标点后的空格
		nextIdx := i + 1
		for nextIdx < len(runes) && (runes[nextIdx] == ' ' || runes[nextIdx] == '\t') {
			nextIdx++
		}
		if nextIdx >= len(runes) {
			continue
		}

		// 检查标点后是否是 URL 路径（如 ./path/to/file）
		if r == '.' && nextIdx < len(runes) && runes[nextIdx] == '/' {
			continue
		}
		// 数字中的小数点
		if r == '.' && i > 0 && i+1 < len(runes) &&
			isDigit(runes[i-1]) && isDigit(runes[nextIdx]) {
			continue
		}

		prefixByteLen := len(string(runes[:i+1]))
		suffix := strings.TrimSpace(line[prefixByteLen:])

		if len(suffix) < minSuffixLen {
			continue
		}

		prefixStr := strings.TrimSpace(line[:prefixByteLen])
		if len(prefixStr) < minPrefixLen {
			continue
		}

		// 前缀验证：前缀不应该像代码或日志
		prefixCL := classifyLine(prefixStr)
		if prefixCL.codeScore >= threshold || prefixCL.logScore >= threshold {
			continue
		}

		// 后缀验证：后缀应该像代码或日志（要求强信号）
		suffixCL := classifyLine(suffix)
		if suffixCL.codeScore >= threshold || suffixCL.logScore >= threshold {
			firstW := strings.ToUpper(firstWord(suffix))
			if isStrongCodeStartWord(firstW) || isStrongLogStartWord(firstW) ||
				suffixCL.codeScore >= threshold+5 || suffixCL.logScore >= threshold+5 {
				suffixType := SegmentCode
				if suffixCL.logScore > suffixCL.codeScore {
					suffixType = SegmentLog
				}
				return []segmentFragment{
					{content: prefixStr, segType: SegmentPrompt},
					{content: suffix, segType: suffixType},
				}
			}
		}
	}

	return nil
}

// isStrongCodeStartWord 检查单词是否是强烈的代码起始信号
func isStrongCodeStartWord(word string) bool {
	strong := map[string]bool{
		"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true,
		"CREATE": true, "ALTER": true, "DROP": true,
		"FUNC": true, "FUNCTION": true, "DEF": true, "CLASS": true,
		"PACKAGE": true, "IMPORT": true, "EXPORT": true,
		"CONST": true, "LET": true, "VAR": true, "TYPE": true,
		"PUBLIC": true, "PRIVATE": true, "PROTECTED": true,
		"FROM": true, "WHERE": true,
		"FN": true, "STRUCT": true, "ENUM": true, "TRAIT": true, "IMPL": true,
		"FOR": true, "IF": true, "WHILE": true, "SWITCH": true,
		"RETURN": true, "ASYNC": true, "AWAIT": true,
		"PRINT": true, "PRINTF": true,
		"#INCLUDE": true, "#DEFINE": true, "#IFNDEF": true, "#IFDEF": true,
		"MODULE": true, "REQUIRE": true,
		"DECLARE": true, "NAMESPACE": true,
	}
	return strong[word]
}

// isStrongLogStartWord 检查单词是否是强烈的日志起始信号
func isStrongLogStartWord(word string) bool {
	strong := map[string]bool{
		"ERROR:": true, "ERROR": true,
		"PANIC:": true, "PANIC": true,
		"FATAL:": true, "FATAL": true,
		"E:": true,
		"FAILED": true, "FAIL": true,
		"WARN:": true, "WARNING:": true,
		"TRACE:": true, "DEBUG:": true,
		"EXCEPTION:": true,
		"RUNTIME": true,
	}
	return strong[word]
}

// ─── 词法边界 ───────────────────────────────────────────────────

// detectCodeKeywordBoundary 检测无标点分隔的代码关键词转换
func detectCodeKeywordBoundary(line string, segType SegmentType, threshold int) []segmentFragment {
	if segType != SegmentCode && segType != SegmentPrompt {
		return nil
	}

	words := strings.Fields(line)
	if len(words) < 2 {
		return nil
	}

	// 从第二个词开始扫描，寻找代码关键词转换点
	for i := 1; i < len(words); i++ {
		word := words[i]
		upperWord := strings.ToUpper(word)

		if len(word) < 2 {
			continue
		}

		// 场景1：独立代码关键词
		if isUnambiguousCodeKeyword(upperWord) || isStrongLogStartWord(upperWord) {
			prefix := strings.TrimSpace(strings.Join(words[:i], " "))
			suffix := strings.TrimSpace(strings.Join(words[i:], " "))

			if len(prefix) < minPrefixLen || len(suffix) < minSuffixLen {
				continue
			}

			prefixCL := classifyLine(prefix)
			if prefixCL.codeScore >= threshold || prefixCL.logScore >= threshold {
				continue
			}

			suffixCL := classifyLine(suffix)
			if suffixCL.codeScore >= threshold || suffixCL.logScore >= threshold {
				scoreGap := suffixCL.codeScore - prefixCL.codeScore
				if suffixCL.logScore > suffixCL.codeScore {
					scoreGap = suffixCL.logScore - prefixCL.logScore
				}
				if scoreGap >= threshold/2 {
					suffixType := SegmentCode
					if suffixCL.logScore > suffixCL.codeScore {
						suffixType = SegmentLog
					}
					return []segmentFragment{
						{content: prefix, segType: SegmentPrompt},
						{content: suffix, segType: suffixType},
					}
				}
			}
		}

		// 场景2：单词内嵌代码关键词
		if len(word) > 5 {
			embeddedIdx := findEmbeddedKeyword(word)
			if embeddedIdx > 1 {
				prefixWords := strings.Join(words[:i], " ")
				prefixPart := word[:embeddedIdx]
				fullPrefix := strings.TrimSpace(prefixWords + " " + prefixPart)

				codePart := word[embeddedIdx:]
				restWords := strings.Join(words[i+1:], " ")
				fullSuffix := strings.TrimSpace(codePart + " " + restWords)

				if len(fullPrefix) >= minPrefixLen && len(fullSuffix) >= minSuffixLen {
					prefixCL := classifyLine(fullPrefix)
					suffixCL := classifyLine(fullSuffix)

					if prefixCL.codeScore < threshold && prefixCL.logScore < threshold &&
						(suffixCL.codeScore >= threshold || suffixCL.logScore >= threshold) {
						suffixType := SegmentCode
						if suffixCL.logScore > suffixCL.codeScore {
							suffixType = SegmentLog
						}
						return []segmentFragment{
							{content: fullPrefix, segType: SegmentPrompt},
							{content: fullSuffix, segType: suffixType},
						}
					}
				}
			}
		}
	}

	return nil
}

// isUnambiguousCodeKeyword 检查单词是否为无歧义的代码关键词
func isUnambiguousCodeKeyword(word string) bool {
	unambiguous := map[string]bool{
		"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true,
		"CREATE": true, "ALTER": true, "DROP": true, "TRUNCATE": true,
		"FUNC": true, "FUNCTION": true, "DEF": true, "FN": true,
		"PACKAGE": true, "CLASS": true, "STRUCT": true, "ENUM": true,
		"TRAIT": true, "IMPL": true, "TYPEDEF": true, "NAMESPACE": true,
		"CONST": true, "LET": true, "VAR": true,
		"PUBLIC": true, "PRIVATE": true, "PROTECTED": true, "READONLY": true,
		"ASYNC": true, "AWAIT": true,
		"DEFER": true,
		"#INCLUDE": true, "#DEFINE": true, "#IFDEF": true, "#IFNDEF": true,
	}
	return unambiguous[word]
}

// embeddedKeywords 可能在单词内部出现的代码关键词
var embeddedKeywords = []string{
	"FUNCTION", "FUNC ", "FUNC(",
	"SELECT ", "INSERT ", "UPDATE ", "DELETE ", "CREATE ",
	"PACKAGE ", "IMPORT ", "CLASS ", "STRUCT ", "ENUM ",
	"DEF ", "FN ", "CONST ", "LET ", "VAR ", "TYPE ",
	"ERROR:", "PANIC:", "FATAL:",
}

// findEmbeddedKeyword 在单词中查找嵌入的代码关键词
func findEmbeddedKeyword(word string) int {
	upper := strings.ToUpper(word)
	for _, kw := range embeddedKeywords {
		idx := strings.Index(upper, kw)
		if idx > 0 && idx < len(upper)-2 {
			return idx
		}
	}
	// 特别处理 "func" 独立出现
	idx := strings.Index(upper, "FUNC")
	if idx > 0 {
		after := idx + 4
		if after >= len(upper) || upper[after] == ' ' || upper[after] == '(' || upper[after] == '\t' {
			return idx
		}
	}
	return -1
}

// ─── 辅助函数 ───────────────────────────────────────────────────

// buildSplitSegments 根据分割结果构建新的 segment 列表
func buildSplitSegments(orig Segment, lines []string, splitLineIdx int, fragments []segmentFragment) []Segment {
	var result []Segment

	// 分割行之前的所有行
	if splitLineIdx > 0 {
		beforeContent := strings.Join(lines[:splitLineIdx], "\n")
		if strings.TrimSpace(beforeContent) != "" {
			result = append(result, NewSegment(fragments[0].segType, beforeContent))
		}
	}

	// 分割行本身：拆分为两个 fragment
	for _, frag := range fragments {
		trimmed := strings.TrimSpace(frag.content)
		if trimmed != "" {
			result = append(result, NewSegment(frag.segType, trimmed))
		}
	}

	// 分割行之后的所有行 → 归入最后一个 fragment 的类型
	if splitLineIdx+1 < len(lines) {
		afterContent := strings.Join(lines[splitLineIdx+1:], "\n")
		if strings.TrimSpace(afterContent) != "" {
			lastType := fragments[len(fragments)-1].segType
			result = append(result, NewSegment(lastType, afterContent))
		}
	}

	if len(result) == 0 {
		return []Segment{orig}
	}

	return result
}

// isWordBoundary 检查 rune 是否为词边界
func isWordBoundary(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
}

// isDigit 检查 rune 是否为数字
func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}
