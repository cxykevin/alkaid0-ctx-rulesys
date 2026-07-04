package rulesys

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// lineClassification 存储一行文本的分类得分
type lineClassification struct {
	isCodeFence      bool   // 是 ``` 代码围栏
	codeScore        int    // 代码模式得分
	logScore         int    // 日志模式得分
	hasCodeFenceLang string // 围栏附带语言标记
}

// classifyLine 对单行文本进行模式匹配评分，O(len(line)) 时间
func classifyLine(line string) lineClassification {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return lineClassification{}
	}

	cl := lineClassification{}

	// 1) 检查代码围栏 ``` （最高优先级）
	if isCodeFence(trimmed) {
		cl.isCodeFence = true
		cl.codeScore = 30
		rest := strings.TrimPrefix(trimmed, "```")
		rest = strings.TrimSpace(rest)
		cl.hasCodeFenceLang = rest
		return cl
	}

	// 2) 代码模式检测
	cl.codeScore = scoreCodeLine(line)

	// 3) 日志模式检测
	cl.logScore = scoreLogLine(line)

	return cl
}

// isCodeFence 检查行是否为 ``` 代码围栏
func isCodeFence(line string) bool {
	return strings.HasPrefix(line, "```")
}

// stripInlineComment 去除行尾的行内注释，返回纯代码部分用于结构分析
func stripInlineComment(line string) string {
	inStr := false
	for i := 0; i < len(line)-1; i++ {
		ch := line[i]
		if ch == '"' || ch == '\'' || ch == '`' {
			inStr = !inStr
			continue
		}
		if !inStr {
			if (ch == '/' && line[i+1] == '/') || (ch == '#' && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t')) {
				if ch == '#' && i > 0 && line[i-1] != ' ' && line[i-1] != '\t' {
					continue
				}
				return strings.TrimRight(line[:i], " \t")
			}
		}
	}
	return line
}

// naturalLanguageWords 自然语言标记词（用于排除自然语言的误判）
var naturalLanguageWords = map[string]bool{
	"the": true, "this": true, "that": true, "these": true, "those": true,
	"with": true, "from": true, "into": true, "about": true, "after": true,
	"before": true, "between": true, "under": true, "over": true,
	"again": true, "further": true, "then": true, "once": true,
	"here": true, "there": true, "when": true, "where": true, "why": true,
	"what": true, "which": true, "who": true, "whom": true,
	"shall": true, "should": true, "would": true, "could": true, "might": true,
	"must": true, "need": true, "dare": true,
	"just": true, "also": true, "very": true, "too": true, "quite": true,
	"please": true, "fix": true, "check": true, "look": true, "help": true,
	"runs": true, "running": true, "worked": true, "working": true,
	"fails": true, "failed": true, "failing": true,
	"shows": true, "shown": true, "showing": true,
	"isn't": true, "aren't": true, "wasn't": true, "weren't": true,
	"don't": true, "doesn't": true, "didn't": true,
	"can't": true, "couldn't": true, "won't": true, "wouldn't": true,
	"hasn't": true, "haven't": true, "hadn't": true,
	"it's": true, "that's": true, "what's": true, "here's": true,
	"there's": true, "how's": true, "where's": true,
}

// isNaturalLanguage 检查文本是否为自然语言（而非命令行参数）
func isNaturalLanguage(words []string) bool {
	stopCount := 0
	for _, w := range words {
		if naturalLanguageWords[strings.ToLower(w)] {
			stopCount++
		}
	}
	return stopCount >= 2
}

// scoreCodeLine 对一行文本进行代码模式评分
func scoreCodeLine(line string) int {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return 0
	}

	score := 0

	// 剥离行内注释以便分析代码结构
	codePart := stripInlineComment(trimmed)
	codePart = strings.TrimSpace(codePart)
	isComment := isCodeComment(trimmed)
	firstWord := firstWord(trimmed)
	firstUpper := strings.ToUpper(firstWord)

	// ── 代码关键字匹配 ──
	codeKeywords := map[string]bool{
		"FUNCTION": true, "CONST": true, "LET": true, "VAR": true,
		"IMPORT": true, "EXPORT": true, "CLASS": true, "RETURN": true,
		"IF": true, "ELSE": true, "FOR": true, "WHILE": true,
		"SWITCH": true, "CASE": true, "BREAK": true, "CONTINUE": true,
		"TYPEDEF": true, "STRUCT": true, "ENUM": true, "INTERFACE": true,
		"PACKAGE": true, "NAMESPACE": true, "USING": true,
		"PUBLIC": true, "PRIVATE": true, "PROTECTED": true, "STATIC": true,
		"VOID": true, "BOOL": true, "INT": true, "FLOAT": true,
		"DOUBLE": true, "CHAR": true, "STRING": true, "BYTE": true,
		"DEF": true, "FROM": true, "YIELD": true, "RAISE": true,
		"TRY": true, "EXCEPT": true, "FINALLY": true, "WITH": true,
		"ASYNC": true, "AWAIT": true,
		"SELECT": true, "UPDATE": true, "DELETE": true, "INSERT": true,
		"CREATE": true, "ALTER": true, "DROP": true, "WHERE": true,
		"JOIN": true, "GROUP": true, "ORDER": true, "HAVING": true,
		"LIMIT": true, "INTO": true, "VALUES": true, "SET": true,
		"FUNC": true, "TYPE": true, "DEFER": true, "GO": true,
		"MODULE": true, "REQUIRE": true, "INCLUDE": true, "DEFINE": true,
		"EXTENDS": true, "IMPLEMENTS": true, "OVERRIDE": true,
		"FALSE": true, "TRUE": true, "NIL": true, "NULL": true,
		"THROW": true, "CATCH": true, "NEW": true, "THIS": true,
		"SUPER": true, "READONLY": true, "DECLARE": true,
		"FN": true, "MUT": true, "IMPL": true, "TRAIT": true,
		"DO": true, "END": true, "BEGIN": true, "RESCUE": true,
		"PRINT": true, "PRINTF": true, "SPRINTF": true,
		"RENDER": true, "COMPONENT": true, "USEEFFECT": true, "USESTATE": true,
		"ANY": true, "NEVER": true, "UNKNOWN": true,
		"GLOBAL": true, "NONLOCAL": true, "PASS": true, "DEL": true,
		"ELIF": true, "ASSERT": true, "LAMBDA": true,
	}

	if codeKeywords[firstUpper] {
		if firstUpper == "FUNC" || firstUpper == "FUNCTION" || firstUpper == "DEF" || firstUpper == "FN" {
			// 函数定义关键字需要后面跟空格或括号
			after := strings.TrimPrefix(trimmed, firstWord)
			if strings.HasPrefix(after, " ") || strings.HasPrefix(after, "(") || trimmed == "fn" {
				score += 20
			}
		} else if firstUpper == "THIS" {
			// "THIS" 是 JS/TS 代码关键词，但 "This" 在句首是自然语言
			// 仅当后面跟 . (this.property) 或 ( (This(...)) 时算代码
			after := strings.TrimPrefix(trimmed, firstWord)
			if strings.HasPrefix(after, ".") || strings.HasPrefix(after, "(") {
				score += 20
			}
		} else {
			score += 20
		}
	}

	// ── 测试/BDD 关键字 ──
	testKeywords := map[string]bool{
		"DESCRIBE": true, "IT": true, "TEST": true, "BEFORE": true,
		"AFTER": true, "BEFOREEACH": true, "AFTEREACH": true,
		"EXPECT": true, "CONTEXT": true,
	}
	if testKeywords[firstUpper] {
		after := strings.TrimPrefix(trimmed, firstWord)
		if strings.HasPrefix(after, "(") || after == "" || strings.HasPrefix(strings.TrimSpace(after), "(") {
			score += 15
		}
	}

	// ── 预处理器指令 ──
	if strings.HasPrefix(trimmed, "#") && len(trimmed) > 1 && !strings.HasPrefix(trimmed, "# ") {
		next := strings.Fields(trimmed[1:])
		if len(next) > 0 {
			upper := strings.ToUpper(next[0])
			preproc := map[string]bool{
				"INCLUDE": true, "DEFINE": true, "IFNDEF": true, "IFDEF": true,
				"IF": true, "ELSE": true, "ENDIF": true, "PRAGMA": true,
				"ERROR": true, "WARNING": true, "UNDEF": true, "IMPORT": true,
			}
			if preproc[upper] {
				score += 20
			}
		}
	}

	// ── 代码结构特征 ──

	// 以 { 结尾
	if strings.HasSuffix(codePart, "{") {
		score += 15
	}
	cpTrimmed := strings.TrimSpace(codePart)
	if cpTrimmed == "}" || cpTrimmed == "{" || cpTrimmed == "};" || cpTrimmed == "}," {
		score += 15
	}
	// 包含 =>
	if strings.Contains(trimmed, "=>") {
		score += 15
	}
	// 以 ) 或 ); 或 ), 结尾
	if strings.HasSuffix(codePart, ")") || strings.HasSuffix(codePart, ");") || strings.HasSuffix(codePart, "),") {
		score += 10
	}
	// 以 ; 结尾
	if strings.HasSuffix(codePart, ";") && len(codePart) > 1 && codePart != ";" {
		score += 10
	}
	// 以逗号结尾
	if strings.HasSuffix(codePart, ",") {
		score += 8
	}
	// 以 ] 或 [] 结尾（数组/列表）
	if strings.HasSuffix(codePart, "]") || codePart == "[]" {
		score += 8
	}
	// 逗号分隔表达式
	if strings.Contains(codePart, ", ") && !containsChinese(codePart) && len(codePart) > 8 {
		score += 5
	}
	// 赋值运算符
	if strings.Contains(codePart, "=") && !strings.Contains(codePart, "==") &&
		!strings.Contains(codePart, "!=") && !strings.Contains(codePart, ">=") &&
		!strings.Contains(codePart, "<=") {
		if score > 0 || !containsChinese(trimmed) {
			score += 5
		}
	}
	// 函数调用 identifier( 模式
	if hasFuncCall(codePart) {
		score += 10
	}
	// 方法调用 .identifier( 模式
	if hasMethodCall(codePart) {
		score += 10
	}
	// 链式调用
	if strings.Count(codePart, ".") >= 2 && !containsChinese(codePart) {
		score += 5
	}
	// Python self.xxx 或 object.property 属性访问模式
	if matched, s := matchPropertyAccess(firstWord, trimmed); matched {
		score += s
	}
	// 模板字面量
	if strings.Contains(trimmed, "${") {
		score += 10
	}
	// ── key: value 模式（对象字面量/字典） ──
	if matched, kvscore := matchKeyValuePattern(codePart, trimmed); matched {
		score += kvscore
	}
	// ── Shell/CLI 命令检测 ──
	if shellScore := matchShellCommand(trimmed, firstWord, firstUpper); shellScore > 0 {
		if !containsChinese(trimmed) {
			after := strings.TrimPrefix(trimmed, firstWord)
			after = strings.TrimSpace(after)
			words := strings.Fields(after)
			if len(words) == 0 || !isNaturalLanguage(words) {
				score += shellScore
			}
		}
	}
	// 注释行
	if isComment {
		score += 5
	}
	// 以数字开头
	firstRune, _ := utf8.DecodeRuneInString(strings.TrimSpace(codePart))
	if unicode.IsDigit(firstRune) {
		score += 5
	}
	// 管道操作符
	if strings.Contains(trimmed, " | ") || strings.HasPrefix(trimmed, "|") {
		score += 8
	}
	// 导入路径/包名（以引号开头的 import string）
	if strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"") {
		rest := strings.TrimPrefix(trimmed, "\"")
		rest = strings.TrimSuffix(rest, "\"")
		if strings.Contains(rest, "/") {
			score += 15
		} else if len(rest) > 1 && !strings.Contains(rest, " ") && !containsChinese(rest) {
			score += 12
		}
	}
	// 结构体字段/变量声明（identifier type 模式，如 logScore int）
	if !containsChinese(codePart) {
		fieldWords := strings.Fields(codePart)
		if len(fieldWords) >= 2 {
			second := fieldWords[1]
			isFieldType := false
			// 已知类型关键字
			typeKW := map[string]bool{
				"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
				"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
				"float32": true, "float64": true, "complex64": true, "complex128": true,
				"bool": true, "string": true, "byte": true, "rune": true, "error": true,
				"any": true, "nil": true,
				"void": true, "char": true, "double": true, "long": true, "short": true,
				"size_t": true, "ssize_t": true, "uintptr_t": true,
				"Map": true, "List": true, "Set": true, "Option": true, "Result": true,
			}
			if typeKW[strings.ToLower(second)] {
				isFieldType = true
			}
			// 指针类型 *TypeName
			if strings.HasPrefix(second, "*") {
				typeName := strings.TrimPrefix(second, "*")
				if len(typeName) > 1 && !containsChinese(typeName) {
					isFieldType = true
				}
			}
			// 大写开头的自定义类型（如 LogEntry, MyStruct）
			if len(second) > 1 {
				firstR, _ := utf8.DecodeRuneInString(second)
				if unicode.IsUpper(firstR) {
					isFieldType = true
				}
			}
			if isFieldType {
				score += 12
			}
		}
	}
	// HTML/JSX/XML
	if strings.Contains(trimmed, "</") || strings.Contains(trimmed, "/>") ||
		strings.HasPrefix(trimmed, "<") && strings.Contains(trimmed, ">") {
		score += 15
	}
	// Dockerfile 指令
	if dkScore := matchDockerfileKeyword(trimmed, firstUpper, firstWord); dkScore > 0 {
		score += dkScore
	}
	// Nginx 配置
	if firstUpper == "SERVER" || firstUpper == "LOCATION" || firstUpper == "UPSTREAM" {
		score += 15
	}
	if firstUpper == "LISTEN" || firstUpper == "SERVER_NAME" || firstUpper == "PROXY_PASS" ||
		firstUpper == "PROXY_SET_HEADER" || firstUpper == "PROXY_HTTP_VERSION" ||
		firstUpper == "PROXY_REDIRECT" || strings.HasPrefix(firstUpper, "PROXY_") {
		score += 15
	}
	// YAML
	if trimmed == "---" || strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "  - ") {
		if !containsChinese(trimmed) {
			score += 5
		}
	}
	// JSON 单行
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		score += 12
	}

	// 行尾中文惩罚：如果代码部分（不含注释）以中文结尾，可能是混合内容
	if !isComment {
		codePartTrimmed := strings.TrimSpace(codePart)
		if containsChinese(codePartTrimmed) {
			lastRune, _ := utf8.DecodeLastRuneInString(codePartTrimmed)
			if unicode.Is(unicode.Han, lastRune) {
				score -= 10
			}
		}
	}

	return score
}

// matchKeyValuePattern 匹配 key: value 模式（对象字面量/字典）
func matchKeyValuePattern(codePart, original string) (bool, int) {
	trimmed := strings.TrimSpace(codePart)
	if containsChinese(trimmed) {
		return false, 0
	}
	// 找第一个 : 分割
	colonIdx := strings.Index(trimmed, ":")
	if colonIdx <= 0 || colonIdx >= len(trimmed)-1 {
		return false, 0
	}
	// key 部分只能含字母、数字、下划线、引号（不能含空格）
	key := strings.TrimSpace(trimmed[:colonIdx])
	if len(key) == 0 {
		return false, 0
	}
	// key 不能含有空格（否则是自然语言"we have: x"）
	if strings.Contains(key, " ") && !strings.HasPrefix(key, "\"") && !strings.HasPrefix(key, "'") {
		return false, 0
	}
	// value 部分以字母/数字/{/[ 开头
	value := strings.TrimSpace(trimmed[colonIdx+1:])
	if len(value) == 0 {
		return false, 0
	}
	firstVal, _ := utf8.DecodeRuneInString(value)
	if !unicode.IsLetter(firstVal) && !unicode.IsDigit(firstVal) && firstVal != '{' && firstVal != '[' && firstVal != '"' && firstVal != '\'' && firstVal != '`' {
		return false, 0
	}
	// 检查 key 是否合理（非自然语言词）
	keyUpper := strings.ToUpper(key)
	naturalKeys := map[string]bool{
		"NOTE": true, "WARNING": true, "CAUTION": true, "IMPORTANT": true,
		"HINT": true, "TODO": true, "FIXME": true, "XXX": true,
		"BUG": true, "HACK": true, "OPTIMIZE": true,
		"STEP": true, "STAGE": true, "PHASE": true, "ERROR": true,
	}
	if naturalKeys[keyUpper] {
		return false, 0
	}
	return true, 8
}

// matchPropertyAccess 检查 Python self.xxx 或 object.property 模式
func matchPropertyAccess(firstWord, trimmed string) (bool, int) {
	// Python self.xxx = yyy 或 self.xxx()
	if strings.HasPrefix(firstWord, "self.") && len(firstWord) > 5 {
		return true, 10
	}
	// 一般 object.attribute 模式（属性访问或赋值，不是方法调用）
	if strings.Contains(firstWord, ".") && !strings.HasPrefix(firstWord, ".") {
		afterDot := strings.SplitN(firstWord, ".", 2)
		if len(afterDot) == 2 && len(afterDot[0]) > 0 && len(afterDot[1]) > 0 {
			// 确保不匹配 IP 地址或数字
			dotPart := afterDot[1]
			if unicode.IsLetter(rune(dotPart[0])) {
				return true, 8
			}
		}
	}
	return false, 0
}

// matchShellCommand 检查是否为 shell 命令
func matchShellCommand(trimmed, firstWord, firstUpper string) int {
	shellCommands := map[string]bool{
		"SH": true, "BASH": true, "ZSH": true,
		"ECHO": true, "CURL": true, "WGET": true,
		"RM": true, "CP": true, "MV": true, "MKDIR": true,
		"CD": true, "LS": true, "CAT": true, "GREP": true, "SED": true,
		"PIP": true, "NPX": true, "YARN": true,
		"DOCKER": true, "KUBECTL": true, "GIT": true, "MAKE": true,
		"CARGO": true, "RUSTC": true,
		"PYTHON": true, "PYTHON3": true, "NODE": true,
		"JAVA": true, "JAVAC": true, "MAVEN": true,
		"HELM": true, "TERRAFORM": true,
		"APT": true, "BREW": true,
		"CHMOD": true, "TOUCH": true, "FIND": true,
		"TAR": true, "GZIP": true, "UNZIP": true,
		"SSH": true, "SCP": true, "RSYNC": true,
		"EXPORT": true, "ALIAS": true, "SOURCE": true,
	}
	if shellCommands[firstUpper] && firstUpper != "GO" {
		after := strings.TrimPrefix(trimmed, firstWord)
		after = strings.TrimSpace(after)
		if after != "" && !strings.HasPrefix(after, "(") {
			return 15
		}
	}
	return 0
}

// matchDockerfileKeyword 检查 Dockerfile 关键字
func matchDockerfileKeyword(trimmed, firstUpper, firstWord string) int {
	dockerKeywords := map[string]bool{
		"FROM": true, "RUN": true, "COPY": true, "ADD": true,
		"CMD": true, "ENTRYPOINT": true, "WORKDIR": true,
		"ENV": true, "EXPOSE": true, "LABEL": true, "MAINTAINER": true,
		"VOLUME": true, "USER": true, "ARG": true, "SHELL": true,
		"HEALTHCHECK": true, "ONBUILD": true, "STOPSIGNAL": true,
	}
	if dockerKeywords[firstUpper] {
		restTrim := strings.TrimSpace(trimmed[len(firstWord):])
		if restTrim != "" && !containsChinese(restTrim) {
			return 15
		}
	}
	return 0
}

// scoreLogLine 对一行文本进行日志模式评分
func scoreLogLine(line string) int {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return 0
	}

	score := 0

	// 高权重（25-30）
	if matched, s := matchErrorPrefix(trimmed); matched {
		score += s
	}
	if isStackTraceLine(trimmed) {
		score += 25
	}
	if strings.HasPrefix(trimmed, "npm ERR!") || strings.HasPrefix(trimmed, "npm WARN") ||
		strings.HasPrefix(trimmed, "npm INFO") || strings.HasPrefix(trimmed, "npm ERR") {
		score += 25
	}
	// 日志级别作为行首独立词（ERROR/WARN/INFO/DEBUG/FATAL/TRACE）
	firstLogWord := strings.ToUpper(firstWord(trimmed))
	logWords := map[string]bool{"ERROR": true, "WARN": true, "INFO": true, "DEBUG": true, "FATAL": true, "TRACE": true}
	if logWords[firstLogWord] && !containsChinese(trimmed) && len(trimmed) > len(firstLogWord) {
		// 确保不是代码关键字
		codeSkipWords := map[string]bool{"ERROR": true, "INFO": true, "DEBUG": true, "TRACE": true}
		if codeSkipWords[firstLogWord] {
			after := strings.TrimSpace(trimmed[len(firstLogWord):])
			if after != "" && !strings.HasPrefix(after, "(") {
				score += 18
			}
		} else {
			score += 18
		}
	}
	// Jenkins Pipeline 输出
	if strings.HasPrefix(trimmed, "[Pipeline] ") {
		score += 18
	}
	// Log output 等日志头部
	if strings.HasPrefix(strings.ToLower(trimmed), "log output") || strings.HasPrefix(strings.ToLower(trimmed), "log example") || strings.HasPrefix(strings.ToLower(trimmed), "example log") || strings.HasPrefix(strings.ToLower(trimmed), "sample log") || strings.HasPrefix(strings.ToLower(trimmed), "test run output") {
		score += 12
	}
	// 中权重（15-20）
	if strings.HasPrefix(trimmed, "> ") {
		score += 15
	}
	if strings.HasPrefix(trimmed, "E: ") && len(trimmed) > 4 {
		score += 20
	}
	if isDockerStepLine(trimmed) {
		score += 20
	}
	if strings.HasPrefix(trimmed, "--->") {
		score += 15
	}
	if strings.HasPrefix(trimmed, "COPY failed") || strings.HasPrefix(trimmed, "COPY failed:") {
		score += 20
	}
	if matched, levelScore := matchLogLevel(trimmed); matched {
		score += levelScore
	}
	if strings.HasPrefix(trimmed, "FAILED ") || strings.HasPrefix(trimmed, "FAIL -") ||
		strings.HasPrefix(trimmed, "FAIL ") || trimmed == "FAIL" || trimmed == "FAILED" {
		score += 20
	}
	if strings.HasPrefix(trimmed, "ERROR: Job failed") || strings.HasPrefix(trimmed, "Pipeline failed") {
		score += 25
	}
	if matchTimestamp(trimmed) {
		score += 15
	}
	if matchGotWantPrefix(trimmed) {
		score += 15
	}
	if strings.HasPrefix(trimmed, "Build failed") || strings.HasPrefix(trimmed, "Build FAILED") {
		score += 15
	}
	if strings.Contains(trimmed, "exit code") || strings.Contains(trimmed, "exited with code") ||
		strings.Contains(trimmed, "Exit code") || strings.Contains(trimmed, "Exited with code") {
		score += 15
	}
	if matchCompilerError(trimmed) {
		score += 15
	}
	if strings.HasPrefix(trimmed, "Step ") && strings.Contains(trimmed, " : ") {
		score += 18
	}
	if containsExceptionName(trimmed) && !isCodeComment(trimmed) {
		score += 12
	}
	// 低权重通用日志
	if (strings.Contains(trimmed, "failed:") || strings.Contains(trimmed, "failed,")) &&
		!strings.Contains(trimmed, "function") && !strings.Contains(trimmed, "var ") &&
		!strings.Contains(trimmed, "func ") && !strings.Contains(trimmed, "= failed") {
		score += 10
	}
	// 行中含 "Error" 作为独立词
	upperWithSpaces := " " + strings.ToUpper(trimmed) + " "
	if strings.Contains(upperWithSpaces, " ERROR ") || strings.HasPrefix(upperWithSpaces[1:], "ERROR ") {
		if !strings.Contains(trimmed, "func ") && !strings.Contains(trimmed, "function ") {
			score += 10
		}
	}
	// 异常类名（但排除 prompt 中的 "syntax error"）
	if strings.Contains(trimmed, "SyntaxError:") || strings.Contains(trimmed, "TypeError:") ||
		strings.Contains(trimmed, "ReferenceError:") {
		if !isCodeComment(trimmed) {
			score += 12
		}
	}
	// 特殊字符开头
	firstRune, _ := utf8.DecodeRuneInString(trimmed)
	if firstRune == '│' || firstRune == '├' || firstRune == '─' ||
		firstRune == '┌' || firstRune == '┐' || firstRune == '┘' ||
		firstRune == '└' || firstRune == '┴' || firstRune == '┬' {
		score += 10
	}

	return score
}

// matchErrorPrefix 匹配行首的 Error: 模式
func matchErrorPrefix(line string) (bool, int) {
	prefixes := []string{"Error:", "ERROR:", "error:"}
	for _, p := range prefixes {
		if strings.HasPrefix(line, p) {
			rest := line[len(p):]
			if len(rest) == 0 || !unicode.IsLetter(rune(rest[0])) {
				return true, 30
			}
		}
	}
	return false, 0
}

// hasFuncCall 检查是否包含 identifier( 模式
func hasFuncCall(line string) bool {
	for i := 0; i < len(line)-1; i++ {
		if line[i] == '(' && i > 0 {
			prev := line[i-1]
			if unicode.IsLetter(rune(prev)) || unicode.IsDigit(rune(prev)) || prev == ')' || prev == ']' {
				return true
			}
		}
	}
	return false
}

// hasMethodCall 检查是否包含 .identifier( 模式
func hasMethodCall(line string) bool {
	for i := 1; i < len(line)-1; i++ {
		if line[i] == '.' && i+1 < len(line)-1 && unicode.IsLetter(rune(line[i+1])) {
			for j := i + 1; j < len(line); j++ {
				if line[j] == '(' {
					return true
				}
				if !unicode.IsLetter(rune(line[j])) && line[j] != '_' {
					break
				}
			}
		}
	}
	return false
}

// regexMatch 简单模式匹配（避免导入 regexp）
func regexMatch(line, pattern string) bool {
	if strings.HasPrefix(pattern, "(?i)") {
		sub := pattern[4:]
		return strings.Contains(strings.ToLower(line), strings.ToLower(sub))
	}
	if strings.HasPrefix(pattern, "^") {
		sub := pattern[1:]
		if strings.HasPrefix(line, sub) {
			return true
		}
		if strings.Contains(sub, `\s`) {
			parts := strings.SplitN(sub, `\s`, 2)
			if len(parts) == 2 && strings.HasPrefix(line, parts[0]) {
				rest := strings.TrimPrefix(line, parts[0])
				rest = strings.TrimSpace(rest)
				return strings.HasPrefix(rest, parts[1])
			}
		}
		return false
	}
	return strings.Contains(line, pattern)
}

// matchGotWantPrefix 匹配测试输出前缀
func matchGotWantPrefix(line string) bool {
	prefixes := []string{"got:", "want:", "expected:", "actual:",
		"Got:", "Want:", "Expected:", "Actual:"}
	for _, p := range prefixes {
		if strings.HasPrefix(line, p) {
			return len(line) > len(p)
		}
	}
	return false
}

// isCodeComment 检查是否为代码注释行
func isCodeComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "//") ||
		strings.HasPrefix(trimmed, "#") && (len(trimmed) == 1 || trimmed[1] == ' ') ||
		strings.HasPrefix(trimmed, "/*") ||
		strings.HasPrefix(trimmed, "* ") ||
		strings.HasPrefix(trimmed, "*/") ||
		strings.HasPrefix(trimmed, "<!--")
}

// firstWord 返回行中第一个非空单词
func firstWord(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}
	fields := strings.Fields(trimmed)
	if len(fields) > 0 {
		return fields[0]
	}
	return ""
}

// containsChinese 检查是否包含中文字符
func containsChinese(s string) bool {
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if unicode.Is(unicode.Han, r) {
			return true
		}
		s = s[size:]
	}
	return false
}

// isStackTraceLine 检查是否为堆栈跟踪行
func isStackTraceLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "at ") && len(trimmed) > 3 {
		rest := trimmed[3:]
		if strings.Contains(rest, "/") || strings.Contains(rest, "(") || strings.Contains(rest, ".") {
			return true
		}
		if strings.Contains(rest, " ") {
			return true
		}
	}
	if strings.HasPrefix(trimmed, "File \"") && strings.Contains(trimmed, ", line ") {
		return true
	}
	if strings.HasPrefix(trimmed, "goroutine ") && strings.Contains(trimmed, " [") {
		return true
	}
	return false
}

// isDockerStepLine 检查是否为 Docker 构建步骤行
func isDockerStepLine(line string) bool {
	if strings.HasPrefix(line, "Step ") && strings.Contains(line, " : ") {
		rest := strings.TrimPrefix(line, "Step ")
		parts := strings.SplitN(rest, " ", 2)
		return len(parts) > 0 && strings.Contains(parts[0], "/")
	}
	return false
}

// matchLogLevel 匹配 [LEVEL] 日志级别
func matchLogLevel(line string) (bool, int) {
	upper := strings.ToUpper(line)
	if strings.Contains(upper, "[ERROR]") || strings.Contains(upper, "[FATAL]") {
		return true, 20
	}
	if strings.Contains(upper, "[WARN]") || strings.Contains(upper, "[WARNING]") {
		return true, 18
	}
	if strings.Contains(upper, "[INFO]") || strings.Contains(upper, "[DEBUG]") ||
		strings.Contains(upper, "[TRACE]") || strings.Contains(upper, "[VERBOSE]") {
		return true, 15
	}
	return false, 0
}

// matchTimestamp 检查是否以日期时间戳开头
func matchTimestamp(line string) bool {
	if len(line) >= 10 && line[4] == '-' && line[7] == '-' {
		year := line[:4]
		if year >= "2020" && year <= "2099" {
			return true
		}
	}
	if len(line) >= 11 && line[0] == '[' && line[5] == '-' && line[8] == '-' {
		return true
	}
	return false
}

// matchCompilerError 检查编译错误模式
func matchCompilerError(line string) bool {
	if (strings.HasPrefix(line, "./") || strings.HasPrefix(line, "/")) && strings.Contains(line, ":") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) >= 2 {
			lineStr := strings.TrimSpace(parts[1])
			if len(lineStr) > 0 && lineStr[0] >= '0' && lineStr[0] <= '9' {
				return true
			}
		}
	}
	return false
}

// containsExceptionName 检查是否包含常见异常类名
func containsExceptionName(line string) bool {
	exceptions := []string{
		"Exception", "RuntimeError", "TypeError", "SyntaxError",
		"ReferenceError", "AssertionError", "KeyError", "ValueError",
		"IndexError", "AttributeError", "IOError", "OSError",
		"ImportError", "ModuleNotFoundError", "FileNotFoundError",
		"NameError", "ZeroDivisionError", "StopIteration",
		"ArithmeticError", "EOFError", "MemoryError",
		"EADDRINUSE", "ECONNREFUSED", "ECONNRESET", "ETIMEOUT",
		"ConcurrentModificationException", "NullPointerException",
		"ClassNotFoundException", "NoSuchMethodException",
		"InterruptedException", "IllegalArgumentException",
		"IllegalStateException", "UnsupportedOperationException",
		"InvocationTargetException", "IOException",
		"SQLException", "TimeoutException", "ExecutionException",
		"RejectedExecutionException", "CancellationException",
		"OutOfMemoryError", "StackOverflowError",
		"panic: ", "fatal error: ", "runtime error: ",
	}
	for _, exc := range exceptions {
		if strings.Contains(line, exc) {
			return true
		}
	}
	return false
}

// isBlankLine 检查是否为空行
func isBlankLine(line string) bool {
	return len(strings.TrimSpace(line)) == 0
}
