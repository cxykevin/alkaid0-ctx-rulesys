package rulesys

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	minSegLen  = 5
	minLineLen = 8
	minPrefix  = 2
	minSuffix  = 3
)

type fragment struct {
	content string
	typ     SegmentType
}

// splitIntraLine 对所有 segment 执行行内分割后处理
func splitIntraLine(segments []Segment, threshold int) []Segment {
	if threshold <= 0 {
		threshold = 12
	}
	var out []Segment
	for _, seg := range segments {
		out = append(out, splitSegment(seg, threshold)...)
	}
	return out
}

// splitSegment 检查单个 segment 是否含混合类型的行
func splitSegment(seg Segment, th int) []Segment {
	if len(seg.Content) < minSegLen {
		return []Segment{seg}
	}
	lines := strings.Split(seg.Content, "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if len(t) < minLineLen {
			continue
		}
		// 整行得分强烈偏向当前类型时，不尝试分割（避免代码内部误拆）
		cl := classifyLine(t)
		if seg.Type == SegmentCode && cl.codeScore >= th*2 {
			continue
		}
		if seg.Type == SegmentLog && cl.logScore >= th*2 {
			continue
		}
		if frags := detectMixed(t, seg.Type, th); len(frags) > 0 {
			return buildSplit(seg, lines, i, frags)
		}
	}
	return []Segment{seg}
}

// detectMixed 检测单行内的类型边界，返回 nil 表示无边界
func detectMixed(line string, segTyp SegmentType, th int) []fragment {
	if strings.HasPrefix(line, "```") {
		return nil
	}

	switch segTyp {
	case SegmentLog:
		if f := detectExplicitDelimiter(line, th); f != nil {
			return f
		}
		// 纯自然语言行混入 log 段
		cl := classifyLine(line)
		if cl.logScore < th && cl.codeScore < th {
			return []fragment{{content: line, typ: SegmentPrompt}}
		}
		// 行首中文 + 日志内容
		if containsChinese(line) {
			return splitLogAtChinese(line, th)
		}
		return nil

	case SegmentPrompt:
		if f := detectExplicitDelimiter(line, th); f != nil {
			return f
		}
		if f := detectColonBoundary(line, segTyp, th); f != nil {
			return f
		}
		if f := detectPunctuationBoundary(line, segTyp, th); f != nil {
			return f
		}
		if f := detectKeywordBoundary(line, segTyp, th); f != nil {
			return f
		}
		return nil

	case SegmentCode:
		if f := detectExplicitDelimiter(line, th); f != nil {
			return f
		}
		if f := detectColonBoundary(line, segTyp, th); f != nil {
			return f
		}
		if f := detectPunctuationBoundary(line, segTyp, th); f != nil {
			return f
		}
		if f := detectKeywordBoundary(line, segTyp, th); f != nil {
			return f
		}
		// 纯自然语言行混入 code 段（排除注释和 docstring）
		if !isCodeComment(line) && !strings.HasPrefix(line, "\"\"\"") {
			cl := classifyLine(line)
			if cl.logScore < th && cl.codeScore < th {
				return []fragment{{content: line, typ: SegmentPrompt}}
			}
		}
		return nil
	}
	return nil
}

// ─── 显式分隔短语 ─────────────────────────────────────────────────────────────

var codeDelimiters = []string{
	"here's the code:", "here is the code:", "here's my code:", "here is my code:",
	"here it is:", "the code is:", "the code:",
	"代码如下：", "代码是：", "代码是这样的：", "代码：", "这段代码：",
	"参考代码：", "示例代码：", "代码片段：", "代码片段:",
}

var logDelimiters = []string{
	"报错信息：", "报错：", "错误信息：", "错误：",
	"error message:", "error:",
	"报错如下：", "错误如下：", "日志：", "日志如下：", "错误日志：",
}

func detectExplicitDelimiter(line string, th int) []fragment {
	lower := strings.ToLower(line)

	for _, phrase := range codeDelimiters {
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
		if len(suffix) < minSuffix {
			continue
		}
		cl := classifyLine(suffix)
		if cl.codeScore >= th || cl.logScore >= th {
			typ := SegmentCode
			if cl.logScore > cl.codeScore {
				typ = SegmentLog
			}
			prefix := strings.TrimRight(line[:idx+len(phrase)], " ")
			return []fragment{{content: prefix, typ: SegmentPrompt}, {content: suffix, typ: typ}}
		}
	}

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
		if len(suffix) < minSuffix {
			continue
		}
		prefixRaw := strings.TrimRight(line[:idx+len(phrase)], " ")
		// 前缀本身若是日志/代码，不分割
		if pcl := classifyLine(prefixRaw); pcl.logScore >= th || pcl.codeScore >= th {
			continue
		}
		cl := classifyLine(suffix)
		if cl.logScore >= th || cl.codeScore >= th {
			typ := SegmentLog
			if cl.codeScore > cl.logScore {
				typ = SegmentCode
			}
			return []fragment{{content: prefixRaw, typ: SegmentPrompt}, {content: suffix, typ: typ}}
		}
	}
	return nil
}

// ─── 冒号边界 ─────────────────────────────────────────────────────────────────

func detectColonBoundary(line string, segTyp SegmentType, th int) []fragment {
	for i, r := range line {
		if r != ':' && r != '：' {
			continue
		}
		// 跳过 http:// https://
		if r == ':' && i >= 4 && strings.HasPrefix(line[i-4:], "http") {
			continue
		}
		// 跳过时间戳中的冒号（HH:MM:SS）
		if r == ':' && i >= 2 && i+2 < len(line) {
			if isDigit(rune(line[i-1])) && isDigit(rune(line[i-2])) &&
				isDigit(rune(line[i+1])) && isDigit(rune(line[i+2])) {
				continue
			}
		}
		// 跳过 :=
		if r == ':' && i+1 < len(line) && line[i+1] == '=' {
			continue
		}
		// 跳过盘符（C:）
		if r == ':' && i == 1 && unicode.IsLetter(rune(line[0])) {
			continue
		}

		prefix := strings.TrimSpace(line[:i])
		charLen := len(string(r))
		suffix := strings.TrimSpace(line[i+charLen:])

		if len(prefix) < minPrefix || len(suffix) < minSuffix {
			continue
		}

		// ★ 关键修复：前缀是引号包裹的 token（JSON/JS 对象 key）→ 不分割
		if isQuotedToken(prefix) {
			continue
		}

		// 冒号后紧跟引号/&/* 等代码字面量起始 → 不分割
		if suffix[0] == '"' || suffix[0] == '\'' || suffix[0] == '`' ||
			suffix[0] == '&' || suffix[0] == '*' || suffix[0] == '[' ||
			suffix[0] == '{' || suffix[0] == '(' {
			continue
		}

		// 前缀必须看起来是自然语言（有 NL 词 或 包含中文）
		if !isNLPrefix(prefix) {
			continue
		}

		pcl := classifyLine(prefix)
		if pcl.codeScore >= th || pcl.logScore >= th {
			continue
		}

		scl := classifyLine(suffix)
		if scl.codeScore >= th || scl.logScore >= th {
			typ := SegmentCode
			if scl.logScore > scl.codeScore {
				typ = SegmentLog
			}
			pfx := strings.TrimRight(line[:i+charLen], " ")
			return []fragment{{content: pfx, typ: SegmentPrompt}, {content: suffix, typ: typ}}
		}
	}
	return nil
}

// isQuotedToken 判断字符串是否是单个引号包裹的 token（JSON/JS 对象 key）
func isQuotedToken(s string) bool {
	if len(s) < 2 {
		return false
	}
	for _, q := range []byte{'"', '\'', '`'} {
		if s[0] == q && s[len(s)-1] == q && !strings.Contains(s[1:len(s)-1], " ") {
			return true
		}
	}
	return false
}

// isNLPrefix 前缀是否像自然语言（有≥1 个 NL 词，或含中文，或≥3 个词）
func isNLPrefix(prefix string) bool {
	if containsChinese(prefix) {
		return true
	}
	words := strings.Fields(prefix)
	if len(words) >= 3 {
		return true
	}
	for _, w := range words {
		if naturalLanguageWords[strings.ToLower(w)] {
			return true
		}
	}
	return false
}

// ─── 句尾标点边界 ─────────────────────────────────────────────────────────────

var sentenceEndPunct = map[rune]bool{'.': true, '。': true, '!': true, '！': true, '?': true, '？': true}

func detectPunctuationBoundary(line string, segTyp SegmentType, th int) []fragment {
	runes := []rune(line)
	for i, r := range runes {
		if !sentenceEndPunct[r] {
			continue
		}
		if i+1 >= len(runes) {
			continue
		}
		next := i + 1
		for next < len(runes) && (runes[next] == ' ' || runes[next] == '\t') {
			next++
		}
		if next >= len(runes) {
			continue
		}
		// 排除：小数点、文件路径 ./
		if r == '.' {
			if i > 0 && isDigit(runes[i-1]) && isDigit(runes[next]) {
				continue
			}
			if runes[next] == '/' {
				continue
			}
		}

		prefixBytes := len(string(runes[:i+1]))
		prefix := strings.TrimSpace(line[:prefixBytes])
		suffix := strings.TrimSpace(line[prefixBytes:])

		if len(prefix) < minPrefix || len(suffix) < minSuffix {
			continue
		}

		// 前缀须是自然语言
		if !isNLPrefix(prefix) {
			continue
		}

		pcl := classifyLine(prefix)
		if pcl.codeScore >= th || pcl.logScore >= th {
			continue
		}

		scl := classifyLine(suffix)
		if scl.codeScore < th && scl.logScore < th {
			continue
		}

		// 后缀须以强代码/日志词开头
		fw := strings.ToUpper(firstWord(suffix))
		if !isStrongCodeStart(fw) && !isStrongLogStart(fw) && matchShellCommand(suffix, firstWord(suffix), fw) == 0 {
			continue
		}

		typ := SegmentCode
		if scl.logScore > scl.codeScore {
			typ = SegmentLog
		}
		return []fragment{{content: prefix, typ: SegmentPrompt}, {content: suffix, typ: typ}}
	}
	return nil
}

func isStrongCodeStart(w string) bool {
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
		"#INCLUDE": true, "#DEFINE": true,
		"MODULE": true, "REQUIRE": true, "DECLARE": true, "NAMESPACE": true,
	}
	return strong[w]
}

func isStrongLogStart(w string) bool {
	strong := map[string]bool{
		"ERROR": true, "ERROR:": true, "PANIC": true, "PANIC:": true,
		"FATAL": true, "FATAL:": true, "FAILED": true, "FAIL": true,
		"WARN": true, "WARN:": true, "WARNING": true, "WARNING:": true,
		"EXCEPTION:": true, "RUNTIME": true,
	}
	return strong[w]
}

// ─── 词法边界（无标点代码关键词）────────────────────────────────────────────────

// unambiguousCodeKW 无歧义的代码关键词（不会出现在自然语言中）
var unambiguousCodeKW = map[string]bool{
	"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true,
	"CREATE": true, "ALTER": true, "DROP": true, "TRUNCATE": true,
	"FUNC": true, "FUNCTION": true, "DEF": true, "FN": true,
	"PACKAGE": true, "CLASS": true, "STRUCT": true, "ENUM": true,
	"TRAIT": true, "IMPL": true, "TYPEDEF": true, "NAMESPACE": true,
	"CONST": true, "LET": true, "VAR": true,
	"PUBLIC": true, "PRIVATE": true, "PROTECTED": true, "READONLY": true,
	"ASYNC": true, "AWAIT": true, "DEFER": true,
	"#INCLUDE": true, "#DEFINE": true, "#IFDEF": true, "#IFNDEF": true,
}

// embeddedKW 可能嵌入在单词中的关键词（需要前缀验证）
var embeddedKW = []string{
	"FUNCTION", "FUNC(", "FUNC ",
	"SELECT ", "INSERT ", "UPDATE ", "DELETE ", "CREATE ",
	"PACKAGE ", "IMPORT ", "CLASS ", "STRUCT ", "ENUM ",
	"DEF ", "FN ", "CONST ", "LET ", "VAR ", "TYPE ",
}

func detectKeywordBoundary(line string, segTyp SegmentType, th int) []fragment {
	words := strings.Fields(line)
	if len(words) < 2 {
		return nil
	}

	for i := 1; i < len(words); i++ {
		w := words[i]
		wUp := strings.ToUpper(w)

		if len(w) < 2 {
			continue
		}

		if unambiguousCodeKW[wUp] || isStrongLogStart(wUp) {
			prefix := strings.TrimSpace(strings.Join(words[:i], " "))
			suffix := strings.TrimSpace(strings.Join(words[i:], " "))
			if len(prefix) < minPrefix || len(suffix) < minSuffix {
				continue
			}
			pcl := classifyLine(prefix)
			if pcl.codeScore >= th || pcl.logScore >= th {
				continue
			}
			scl := classifyLine(suffix)
			if scl.codeScore >= th || scl.logScore >= th {
				gap := scl.codeScore - pcl.codeScore
				if scl.logScore > scl.codeScore {
					gap = scl.logScore - pcl.logScore
				}
				if gap >= th/2 {
					typ := SegmentCode
					if scl.logScore > scl.codeScore {
						typ = SegmentLog
					}
					return []fragment{{content: prefix, typ: SegmentPrompt}, {content: suffix, typ: typ}}
				}
			}
		}

		// 单词内嵌关键词（如 "errfunc" → "err" + "func"）
		if len(w) > 5 {
			if embIdx := findEmbedded(w); embIdx > 1 {
				prefWords := strings.Join(words[:i], " ")
				fullPrefix := strings.TrimSpace(prefWords + " " + w[:embIdx])
				fullSuffix := strings.TrimSpace(w[embIdx:] + " " + strings.Join(words[i+1:], " "))
				if len(fullPrefix) >= minPrefix && len(fullSuffix) >= minSuffix {
					pcl := classifyLine(fullPrefix)
					scl := classifyLine(fullSuffix)
					if pcl.codeScore < th && pcl.logScore < th &&
						(scl.codeScore >= th || scl.logScore >= th) {
						typ := SegmentCode
						if scl.logScore > scl.codeScore {
							typ = SegmentLog
						}
						return []fragment{{content: fullPrefix, typ: SegmentPrompt}, {content: fullSuffix, typ: typ}}
					}
				}
			}
		}
	}
	return nil
}

func findEmbedded(word string) int {
	up := strings.ToUpper(word)
	for _, kw := range embeddedKW {
		idx := strings.Index(up, kw)
		if idx > 0 {
			return idx
		}
	}
	// 特殊处理 "func" 边界（func 后接空格/括号/结尾）
	if idx := strings.Index(up, "FUNC"); idx > 0 {
		after := idx + 4
		if after >= len(up) || up[after] == ' ' || up[after] == '(' {
			return idx
		}
	}
	return -1
}

// ─── 行首中文 + 日志内容分割 ──────────────────────────────────────────────────

func splitLogAtChinese(line string, th int) []fragment {
	runes := []rune(line)
	// 找第一个非汉字非空格字符
	firstNonHan := -1
	for i, r := range runes {
		if !unicode.Is(unicode.Han, r) && !unicode.IsSpace(r) {
			firstNonHan = i
			break
		}
	}
	if firstNonHan <= 0 || firstNonHan >= len(runes)-1 {
		return nil
	}
	// 跳过空白
	start := firstNonHan
	for start < len(runes) && (runes[start] == ' ' || runes[start] == '\t') {
		start++
	}
	if start >= len(runes) {
		return nil
	}
	// 后续内容须以日志特征开头（年份数字/[/( 等）
	r0 := runes[start]
	if !((r0 >= '0' && r0 <= '9' && start+3 < len(runes) &&
		runes[start+1] >= '0' && runes[start+1] <= '9' &&
		runes[start+2] >= '0' && runes[start+2] <= '9' &&
		runes[start+3] >= '0' && runes[start+3] <= '9') ||
		r0 == '[' || r0 == '(') {
		return nil
	}
	prefix := strings.TrimSpace(string(runes[:firstNonHan]))
	suffix := strings.TrimSpace(string(runes[start:]))
	if len(prefix) < minPrefix || len(suffix) < minSuffix {
		return nil
	}
	pcl := classifyLine(prefix)
	if pcl.logScore >= th || pcl.codeScore >= th {
		return nil
	}
	scl := classifyLine(suffix)
	if scl.logScore < th {
		return nil
	}
	return []fragment{{content: prefix, typ: SegmentPrompt}, {content: suffix, typ: SegmentLog}}
}

// ─── 构建分割结果 ─────────────────────────────────────────────────────────────

func buildSplit(orig Segment, lines []string, splitIdx int, frags []fragment) []Segment {
	var result []Segment

	if splitIdx > 0 {
		before := strings.Join(lines[:splitIdx], "\n")
		if strings.TrimSpace(before) != "" {
			result = append(result, NewSegment(orig.Type, before))
		}
	}

	for _, f := range frags {
		if strings.TrimSpace(f.content) != "" {
			result = append(result, NewSegment(f.typ, strings.TrimSpace(f.content)))
		}
	}

	if splitIdx+1 < len(lines) {
		after := strings.Join(lines[splitIdx+1:], "\n")
		if strings.TrimSpace(after) != "" {
			lastTyp := frags[len(frags)-1].typ
			result = append(result, NewSegment(lastTyp, after))
		}
	}

	if len(result) == 0 {
		return []Segment{orig}
	}
	return result
}

// ─── 辅助 ─────────────────────────────────────────────────────────────────────

func isWordBoundary(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}
