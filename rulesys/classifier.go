package rulesys

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// lineClassification 存储单行文本的分类得分
type lineClassification struct {
	isCodeFence      bool
	codeScore        int
	logScore         int
	hasCodeFenceLang string
}

// classifyLine 对单行文本进行模式匹配评分
func classifyLine(line string) lineClassification {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return lineClassification{}
	}
	if strings.HasPrefix(trimmed, "```") {
		lang := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
		return lineClassification{isCodeFence: true, codeScore: 30, hasCodeFenceLang: lang}
	}
	return lineClassification{
		codeScore: scoreCode(trimmed),
		logScore:  scoreLog(trimmed),
	}
}

// ─── Code scoring ─────────────────────────────────────────────────────────────

func scoreCode(line string) int {
	score := 0
	codePart := stripComment(line)
	fw := firstWord(line)
	fwUp := strings.ToUpper(fw)

	// ── 行首关键字（强信号）──
	score += matchCodeKeyword(line, fw, fwUp)

	// ── 结构性语法特征 ──
	if strings.HasSuffix(codePart, "{") {
		score += 15
	}
	cp := strings.TrimSpace(codePart)
	if cp == "}" || cp == "{" || cp == "};" || cp == "}," || cp == "});" || cp == "})" {
		score += 15
	}
	if strings.Contains(line, "=>") {
		score += 15
	}
	if strings.Contains(line, ":=") {
		score += 10
	}
	if strings.HasSuffix(codePart, ";") && len(codePart) > 1 {
		score += 10
	}
	if strings.HasSuffix(codePart, ")") || strings.HasSuffix(codePart, ");") || strings.HasSuffix(codePart, "),") {
		score += 8
	}
	if strings.HasSuffix(codePart, ",") {
		score += 6
	}
	if strings.HasSuffix(codePart, "]") {
		score += 6
	}
	if strings.Contains(line, "${") {
		score += 10
	}

	// ── 函数/方法调用 ──
	if hasFuncCall(codePart) {
		score += 10
	}
	if hasMethodCall(codePart) {
		score += 10
	}

	// ── YAML 列表项（- value）——移到赋值检测之前，以便 hasAssignment 能叠加 ──
	if (strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "  - ")) && !containsChinese(line) {
		score += 8
	}

	// ── 赋值（有其他代码信号时才加分）──
	if score > 0 && hasAssignment(codePart) {
		score += 5
	}

	// ── key: value 模式（YAML/JSON/对象字面量）──
	if s := scoreKeyValue(line, codePart); s > 0 {
		score += s
	}

	// ── JSON 字符串字面量行 ──
	if strings.HasPrefix(line, "\"") && strings.HasSuffix(line, "\"") {
		inner := line[1 : len(line)-1]
		if strings.Contains(inner, "/") {
			score += 15 // import 路径
		} else if len(inner) >= 1 && !strings.Contains(inner, " ") && !containsChinese(inner) {
			score += 12 // 短 import 名（"fmt", "os" 等）
		}
	}

	// ── struct 字段：identifier type 模式 ──
	score += scoreStructField(codePart)

	// ── Shell 命令 ──
	if s := scoreShell(line, fw, fwUp); s > 0 {
		score += s
	}

	// ── 预处理器指令 ──
	if strings.HasPrefix(line, "#") && len(line) > 2 && line[1] != ' ' {
		kw := strings.ToUpper(firstWord(line[1:]))
		prep := map[string]bool{
			"INCLUDE": true, "DEFINE": true, "IFNDEF": true, "IFDEF": true,
			"IF": true, "ELSE": true, "ENDIF": true, "PRAGMA": true, "UNDEF": true,
		}
		if prep[kw] {
			score += 20
		}
	}

	// ── HTML/JSX ──
	if strings.Contains(line, "</") || strings.Contains(line, "/>") ||
		(strings.HasPrefix(line, "<") && strings.Contains(line, ">") && !strings.Contains(line, " ")) {
		score += 12
	}

	// ── JSON 单行 ──
	if (strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}")) ||
		(strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")) {
		score += 10
	}

	// ── YAML 列表项（- value）──
	if (strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "  - ")) && !containsChinese(line) {
		score += 8
	}

	// ── 代码注释 ──
	if isCodeComment(line) {
		score += 5
	}

	// ── 管道操作符 ──
	if strings.Contains(line, " | ") || strings.HasPrefix(line, "|") {
		score += 6
	}

	// ── 首字符为数字（hex/版本号/行号等）──
	if r, _ := utf8.DecodeRuneInString(strings.TrimSpace(codePart)); unicode.IsDigit(r) {
		score += 4
	}

	// ── 行尾中文惩罚：非注释行以汉字结尾大概率是混合内容 ──
	if !isCodeComment(line) && containsChinese(codePart) {
		if r, _ := utf8.DecodeLastRuneInString(strings.TrimSpace(codePart)); unicode.Is(unicode.Han, r) {
			score -= 10
		}
	}

	if score < 0 {
		score = 0
	}
	return score
}

// matchCodeKeyword 匹配行首代码关键字，返回加分
func matchCodeKeyword(line, fw, fwUp string) int {
	// 函数定义关键字（func/def/fn 需要后接空格或括号）
	funcKW := map[string]bool{"FUNC": true, "FUNCTION": true, "DEF": true, "FN": true}
	if funcKW[fwUp] {
		after := strings.TrimPrefix(line, fw)
		if strings.HasPrefix(after, " ") || strings.HasPrefix(after, "(") {
			return 20
		}
		return 0
	}

	// this.xxx → 代码（属性访问）；句首 This → 自然语言
	if fwUp == "THIS" {
		after := strings.TrimPrefix(line, fw)
		if strings.HasPrefix(after, ".") || strings.HasPrefix(after, "(") {
			return 20
		}
		return 0
	}

	// 通用代码关键字
	codeKW := map[string]bool{
		"CONST": true, "LET": true, "VAR": true, "TYPE": true,
		"IMPORT": true, "EXPORT": true, "CLASS": true, "STRUCT": true, "ENUM": true,
		"INTERFACE": true, "TRAIT": true, "IMPL": true, "TYPEDEF": true,
		"PACKAGE": true, "NAMESPACE": true, "MODULE": true,
		"RETURN": true, "IF": true, "ELSE": true, "FOR": true, "WHILE": true,
		"SWITCH": true, "CASE": true, "BREAK": true, "CONTINUE": true,
		"DO": true, "END": true, "BEGIN": true, "RESCUE": true,
		"ASYNC": true, "AWAIT": true, "DEFER": true,
		"TRY": true, "CATCH": true, "THROW": true, "RAISE": true,
		"EXCEPT": true, "FINALLY": true,
		"PUBLIC": true, "PRIVATE": true, "PROTECTED": true, "STATIC": true,
		"READONLY": true, "DECLARE": true, "EXTENDS": true, "IMPLEMENTS": true,
		"VOID": true, "BOOL": true, "INT": true, "FLOAT": true, "DOUBLE": true,
		"CHAR": true, "STRING": true, "BYTE": true, "NULL": true, "NIL": true,
		"TRUE": true, "FALSE": true,
		"SELECT": true, "UPDATE": true, "DELETE": true, "INSERT": true,
		"CREATE": true, "ALTER": true, "DROP": true, "WHERE": true,
		"FROM": true, "JOIN": true, "GROUP": true, "ORDER": true,
		"HAVING": true, "LIMIT": true, "VALUES": true, "SET": true,
		"PRINT": true, "PRINTF": true, "SPRINTF": true,
		"NEW": true, "SUPER": true,
		"MUT": true, "GLOBAL": true, "NONLOCAL": true, "PASS": true, "DEL": true,
		"ELIF": true, "ASSERT": true, "LAMBDA": true, "YIELD": true,
		"OVERRIDE": true, "USING": true, "REQUIRE": true, "INCLUDE": true,
		"WITH": true,
	}
	if codeKW[fwUp] {
		return 20
	}

	// Dockerfile 关键字
	dockerKW := map[string]bool{
		"FROM": true, "RUN": true, "COPY": true, "ADD": true,
		"CMD": true, "ENTRYPOINT": true, "WORKDIR": true, "ENV": true,
		"EXPOSE": true, "LABEL": true, "VOLUME": true, "USER": true,
		"ARG": true, "SHELL": true, "HEALTHCHECK": true,
	}
	if dockerKW[fwUp] {
		rest := strings.TrimSpace(strings.TrimPrefix(line, fw))
		if rest != "" && !containsChinese(rest) {
			return 15
		}
	}

	// Nginx 配置
	nginxKW := map[string]bool{
		"SERVER": true, "LOCATION": true, "UPSTREAM": true,
		"LISTEN": true, "SERVER_NAME": true, "PROXY_PASS": true,
	}
	if nginxKW[fwUp] || strings.HasPrefix(fwUp, "PROXY_") {
		return 15
	}

	// 测试 DSL
	testKW := map[string]bool{
		"DESCRIBE": true, "IT": true, "TEST": true, "EXPECT": true,
		"BEFORE": true, "AFTER": true, "CONTEXT": true,
	}
	if testKW[fwUp] {
		after := strings.TrimPrefix(line, fw)
		if strings.HasPrefix(strings.TrimSpace(after), "(") {
			return 15
		}
	}

	// this.xxx / self.xxx（JS/Python 属性访问）
	fwLow := strings.ToLower(fw)
	if (strings.HasPrefix(fwLow, "this.") || strings.HasPrefix(fwLow, "self.")) && len(fw) > 5 {
		return 20
	}

	// object.property 模式（非数字、非 IP 地址）
	if strings.Contains(fw, ".") && !strings.HasPrefix(fw, ".") {
		parts := strings.SplitN(fw, ".", 2)
		if len(parts[1]) > 0 && unicode.IsLetter(rune(parts[1][0])) {
			return 10
		}
	}

	return 0
}

// scoreKeyValue 匹配 key: value 或 key: 模式（YAML/JSON/对象字面量）
func scoreKeyValue(line, codePart string) int {
	if containsChinese(line) {
		return 0
	}
	// YAML/对象 key: value（key 无空格，value 非空）
	idx := strings.Index(codePart, ": ")
	if idx > 0 {
		key := strings.TrimSpace(codePart[:idx])
		if !strings.Contains(key, " ") && len(key) >= 2 {
			return 12
		}
	}
	// YAML 嵌套键（key: 行尾，无空格）
	if strings.HasSuffix(strings.TrimSpace(codePart), ":") {
		key := strings.TrimSuffix(strings.TrimSpace(codePart), ":")
		if !strings.Contains(key, " ") && len(key) >= 2 {
			return 12
		}
	}
	// 行中含 "key": value（JSON 对象行，即使被 {}/[] 包裹）
	if strings.Contains(line, "\": ") || strings.Contains(line, "\":") {
		return 12
	}
	// 结构体字面量 key:"value" 或 key:`value`
	if strings.Contains(line, ":\"") || strings.Contains(line, ":`") {
		return 12
	}
	return 0
}

// scoreStructField 匹配 Go struct 字段声明模式：identifier  type
func scoreStructField(codePart string) int {
	if containsChinese(codePart) {
		return 0
	}
	fields := strings.Fields(codePart)
	if len(fields) < 2 {
		return 0
	}
	// 第一个词必须是合法标识符（只含字母/数字/下划线），否则不是字段名
	if !isIdentifier(fields[0]) {
		return 0
	}
	second := fields[1]
	// 已知原始类型
	builtinTypes := map[string]bool{
		"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true, "complex64": true, "complex128": true,
		"bool": true, "string": true, "byte": true, "rune": true, "error": true,
		"any": true, "nil": true,
		"void": true, "char": true, "double": true, "long": true, "short": true,
	}
	if builtinTypes[strings.ToLower(second)] {
		return 12
	}
	// 指针类型 *TypeName
	if strings.HasPrefix(second, "*") {
		rest := second[1:]
		if len(rest) > 1 && !containsChinese(rest) {
			return 12
		}
	}
	// 大写开头的自定义类型（如 LogEntry, MyStruct）
	if r, _ := utf8.DecodeRuneInString(second); unicode.IsUpper(r) && len(second) > 1 {
		return 12
	}
	return 0
}

// scoreShell 匹配 shell 命令行
func scoreShell(line, fw, fwUp string) int {
	shellCmds := map[string]bool{
		"SH": true, "BASH": true, "ZSH": true,
		"ECHO": true, "CURL": true, "WGET": true,
		"RM": true, "CP": true, "MV": true, "MKDIR": true,
		"CD": true, "LS": true, "CAT": true, "GREP": true, "SED": true, "AWK": true,
		"PIP": true, "NPX": true, "YARN": true, "NPM": true,
		"DOCKER": true, "KUBECTL": true, "GIT": true, "MAKE": true,
		"CARGO": true, "RUSTC": true, "GO": true,
		"PYTHON": true, "PYTHON3": true, "NODE": true,
		"JAVA": true, "JAVAC": true, "MVN": true,
		"HELM": true, "TERRAFORM": true,
		"APT": true, "APT-GET": true, "BREW": true,
		"CHMOD": true, "TOUCH": true, "FIND": true, "CHOWN": true,
		"TAR": true, "GZIP": true, "UNZIP": true, "ZIP": true,
		"SSH": true, "SCP": true, "RSYNC": true,
		"EXPORT": true, "ALIAS": true, "SOURCE": true,
	}
	if !shellCmds[fwUp] {
		return 0
	}
	after := strings.TrimSpace(strings.TrimPrefix(line, fw))
	if after == "" || strings.HasPrefix(after, "(") {
		return 0
	}
	if containsChinese(fw) {
		return 0
	}
	words := strings.Fields(after)
	if isNaturalLanguage(words) {
		return 0
	}
	return 15
}

// ─── Log scoring ──────────────────────────────────────────────────────────────

func scoreLog(line string) int {
	score := 0

	// ── 高权重：Error:/PANIC:/FATAL: 前缀 ──
	for _, p := range []string{"Error:", "ERROR:", "error:", "PANIC:", "Panic:", "FATAL:", "Fatal:"} {
		if strings.HasPrefix(line, p) {
			rest := line[len(p):] // 不 TrimSpace：保留空格以判断 "Error: msg" vs "ErrorHandler"
			if len(rest) == 0 || !unicode.IsLetter(rune(rest[0])) {
				score += 30
			}
		}
	}

	// ── 堆栈跟踪 ──
	if isStackTrace(line) {
		score += 25
	}

	// ── npm/yarn 输出 ──
	if strings.HasPrefix(line, "npm ERR") || strings.HasPrefix(line, "npm WARN") || strings.HasPrefix(line, "npm INFO") {
		score += 25
	}

	// ── 行首日志级别（独立词，非代码上下文）──
	fw := firstWord(line)
	fwUp := strings.ToUpper(fw)
	logLevels := map[string]bool{"ERROR": true, "WARN": true, "WARNING": true, "INFO": true, "DEBUG": true, "FATAL": true, "TRACE": true}
	if logLevels[fwUp] && !containsChinese(line) {
		after := strings.TrimSpace(strings.TrimPrefix(line, fw))
		if after != "" && !strings.HasPrefix(after, "(") {
			score += 18
		}
	}

	// ── Jenkins/CI 前缀 ──
	if strings.HasPrefix(line, "[Pipeline]") {
		score += 18
	}

	// ── Docker 构建步骤 ──
	if strings.HasPrefix(line, "Step ") && strings.Contains(line, " : ") {
		if parts := strings.SplitN(strings.TrimPrefix(line, "Step "), " ", 2); len(parts) > 0 && strings.Contains(parts[0], "/") {
			score += 20
		}
	}
	if strings.HasPrefix(line, "--->") {
		score += 15
	}
	if strings.HasPrefix(line, "E: ") && len(line) > 3 {
		score += 20
	}
	if strings.HasPrefix(line, "> ") {
		score += 12
	}
	if strings.HasPrefix(line, "COPY failed") {
		score += 20
	}

	// ── [LEVEL] 括号日志级别 ──
	up := strings.ToUpper(line)
	if strings.Contains(up, "[ERROR]") || strings.Contains(up, "[FATAL]") {
		score += 20
	} else if strings.Contains(up, "[WARN]") || strings.Contains(up, "[WARNING]") {
		score += 18
	} else if strings.Contains(up, "[INFO]") || strings.Contains(up, "[DEBUG]") || strings.Contains(up, "[TRACE]") {
		score += 15
	}

	// ── 时间戳开头 ──
	if matchTimestamp(line) {
		score += 15
	}

	// ── 测试框架输出（got:/want:）──
	for _, p := range []string{"got:", "Got:", "want:", "Want:", "expected:", "Expected:", "actual:", "Actual:"} {
		if strings.HasPrefix(line, p) && len(line) > len(p) {
			score += 15
		}
	}

	// ── 构建失败 ──
	if strings.HasPrefix(line, "Build failed") || strings.HasPrefix(line, "Build FAILED") {
		score += 15
	}
	if strings.HasPrefix(line, "FAILED") || strings.HasPrefix(line, "FAIL ") || line == "FAIL" {
		score += 18
	}
	if strings.Contains(line, "exit code") || strings.Contains(line, "exited with code") {
		score += 15
	}

	// ── 编译错误（./file:N: 模式）──
	if matchCompilerError(line) {
		score += 15
	}

	// ── 异常类名 ──
	if containsException(line) && !isCodeComment(line) {
		score += 12
	}

	// ── 特殊 Unicode 框线字符 ──
	if r, _ := utf8.DecodeRuneInString(line); r == '│' || r == '├' || r == '└' || r == '┌' || r == '─' {
		score += 10
	}

	// ── 行中含独立 Error 词 ──
	if strings.Contains(" "+strings.ToUpper(line)+" ", " ERROR ") {
		if !strings.Contains(line, "func ") && !strings.Contains(line, "function ") {
			score += 8
		}
	}

	return score
}

func isStackTrace(line string) bool {
	if strings.HasPrefix(line, "at ") && len(line) > 3 {
		rest := line[3:]
		return strings.ContainsAny(rest, "/(.")
	}
	if strings.HasPrefix(line, "File \"") && strings.Contains(line, ", line ") {
		return true
	}
	if strings.HasPrefix(line, "goroutine ") && strings.Contains(line, " [") {
		return true
	}
	return false
}

func matchTimestamp(line string) bool {
	// 2020-01-01... 格式
	if len(line) >= 10 && line[4] == '-' && line[7] == '-' && line[:4] >= "2020" && line[:4] <= "2099" {
		return true
	}
	// [2020-... 格式
	if len(line) >= 12 && line[0] == '[' && line[5] == '-' && line[8] == '-' {
		return true
	}
	return false
}

func matchCompilerError(line string) bool {
	if strings.HasPrefix(line, "./") || strings.HasPrefix(line, "/") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) >= 2 {
			n := strings.TrimSpace(parts[1])
			return len(n) > 0 && n[0] >= '0' && n[0] <= '9'
		}
	}
	return false
}

func containsException(line string) bool {
	exceptions := []string{
		"Exception", "RuntimeError", "TypeError", "SyntaxError",
		"ReferenceError", "AssertionError", "KeyError", "ValueError",
		"IndexError", "AttributeError", "IOError", "OSError",
		"ImportError", "ModuleNotFoundError", "FileNotFoundError",
		"NullPointerException", "ClassNotFoundException",
		"IllegalArgumentException", "IllegalStateException",
		"OutOfMemoryError", "StackOverflowError",
		"panic: ", "fatal error: ", "runtime error: ",
		"EADDRINUSE", "ECONNREFUSED", "ECONNRESET",
	}
	for _, e := range exceptions {
		if strings.Contains(line, e) {
			return true
		}
	}
	return false
}

// ─── 辅助函数 ─────────────────────────────────────────────────────────────────

func stripComment(line string) string {
	inStr := false
	for i := 0; i < len(line)-1; i++ {
		ch := line[i]
		if ch == '"' || ch == '\'' || ch == '`' {
			inStr = !inStr
		}
		if !inStr {
			if ch == '/' && line[i+1] == '/' {
				return strings.TrimRight(line[:i], " \t")
			}
			if ch == '#' && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t') {
				return strings.TrimRight(line[:i], " \t")
			}
		}
	}
	return line
}

func isCodeComment(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "//") ||
		(strings.HasPrefix(t, "#") && (len(t) == 1 || t[1] == ' ')) ||
		strings.HasPrefix(t, "/*") ||
		strings.HasPrefix(t, "* ") ||
		strings.HasPrefix(t, "*/") ||
		strings.HasPrefix(t, "<!--")
}

func hasFuncCall(line string) bool {
	for i := 1; i < len(line); i++ {
		if line[i] == '(' {
			prev := rune(line[i-1])
			if unicode.IsLetter(prev) || unicode.IsDigit(prev) || prev == '_' || prev == ')' || prev == ']' {
				return true
			}
		}
	}
	return false
}

func hasMethodCall(line string) bool {
	for i := 1; i+1 < len(line); i++ {
		if line[i] == '.' && unicode.IsLetter(rune(line[i+1])) {
			for j := i + 2; j < len(line); j++ {
				if line[j] == '(' {
					return true
				}
				if !unicode.IsLetter(rune(line[j])) && line[j] != '_' && !unicode.IsDigit(rune(line[j])) {
					break
				}
			}
		}
	}
	return false
}

func hasAssignment(codePart string) bool {
	return strings.Contains(codePart, "=") &&
		!strings.Contains(codePart, "==") &&
		!strings.Contains(codePart, "!=") &&
		!strings.Contains(codePart, ">=") &&
		!strings.Contains(codePart, "<=")
}

func firstWord(line string) string {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) > 0 {
		return fields[0]
	}
	return ""
}

func containsChinese(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// isIdentifier 检查字符串是否为合法的 Go/通用标识符（字母/数字/下划线，首字母非数字）
func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 && !unicode.IsLetter(r) && r != '_' {
			return false
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

func isBlankLine(line string) bool {
	return len(strings.TrimSpace(line)) == 0
}

// naturalLanguageWords 常见英文虚词/助词，用于判断是否为自然语言
var naturalLanguageWords = map[string]bool{
	"the": true, "this": true, "that": true, "these": true, "those": true,
	"with": true, "into": true, "about": true, "after": true, "before": true,
	"between": true, "under": true, "over": true, "again": true, "then": true,
	"here": true, "there": true, "when": true, "where": true, "why": true,
	"what": true, "which": true, "who": true,
	"shall": true, "should": true, "would": true, "could": true, "might": true,
	"must": true, "just": true, "also": true, "please": true,
	"fix": true, "check": true, "help": true, "look": true,
	"runs": true, "running": true, "fails": true, "failed": true, "failing": true,
	"isn't": true, "aren't": true, "don't": true, "doesn't": true, "can't": true,
	"it's": true, "that's": true, "here's": true,
}

func isNaturalLanguage(words []string) bool {
	n := 0
	for _, w := range words {
		if naturalLanguageWords[strings.ToLower(w)] {
			n++
		}
	}
	return n >= 2
}

// matchShellCommand 供 intraline 调用
func matchShellCommand(line, fw, fwUp string) int {
	return scoreShell(line, fw, fwUp)
}
