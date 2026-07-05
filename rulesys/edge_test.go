package rulesys

import (
	"strings"
	"testing"
)

// ── 第一批边界样例（50 个） ──
func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, segs []Segment)
	}{
		// ── 纯自然语言 ──
		{"pure_chinese", "axios拦截器怎么取消", func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentPrompt {
				t.Errorf("expected 1 prompt, got %+v", segs)
			}
		}},
		{"pure_english", "hello world", func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentPrompt {
				t.Errorf("expected 1 prompt, got %+v", segs)
			}
		}},
		{"sentence_start_this", "This is a normal English sentence", func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentPrompt {
				t.Errorf("expected 1 prompt, got %+v", segs)
			}
		}},
		{"short_prompt", "fix it", func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentPrompt {
				t.Errorf("expected 1 prompt, got %+v", segs)
			}
		}},
		// ── 纯代码 ──
		{"go_func_def", "func main() {\n\treturn\n}", func(t *testing.T, segs []Segment) {
			if len(segs) < 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code, got %+v", segs)
			}
		}},
		{"go_package", "package main", func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code, got %+v", segs)
			}
		}},
		{"js_arrow", "const add = (a, b) => a + b", func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code, got %+v", segs)
			}
		}},
		{"python_def", "def factorial(n):\n    if n == 0:\n        return 1", func(t *testing.T, segs []Segment) {
			if len(segs) < 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code, got %+v", segs)
			}
		}},
		{"go_import_block", "import (\n\t\"fmt\"\n\t\"os\"\n\t\"strings\"\n)", func(t *testing.T, segs []Segment) {
			for _, s := range segs {
				if s.Type != SegmentCode {
					t.Errorf("expected all code, got prompt: %q", s.Content)
				}
			}
		}},
		{"import_strings_inline", `"fmt"`, func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code for import string, got %+v", segs)
			}
		}},
		{"import_path", `"github.com/user/pkg"`, func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code for import path, got %+v", segs)
			}
		}},
		{"go_struct_field_ptr", "entry    *LogEntry", func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code for struct field, got %+v", segs)
			}
		}},
		{"go_struct_field_custom", "user    User", func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code for struct field, got %+v", segs)
			}
		}},
		{"go_struct_field_basic", "name     string", func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code for struct field, got %+v", segs)
			}
		}},
		{"go_method_call", "fmt.Fprintln(os.Stderr, \"hello\")", func(t *testing.T, segs []Segment) {
			if len(segs) < 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code for method call, got %+v", segs)
			}
		}},
		{"go_assignment", "verbose := false", func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code for assignment, got %+v", segs)
			}
		}},
		{"go_args_slice", "args := os.Args[1:]", func(t *testing.T, segs []Segment) {
			if len(segs) < 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code, got %+v", segs)
			}
		}},
		{"go_string_join", `input := strings.Join(args, " ")`, func(t *testing.T, segs []Segment) {
			if len(segs) < 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code, got %+v", segs)
			}
		}},
		{"go_struct_literal", "entry: &LogEntry{", func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code, got %+v", segs)
			}
		}},
		{"go_struct_literal_ptr", "ref: *entry", func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code, got %+v", segs)
			}
		}},
		// ── 冒号后跟引号（代码字面量）──
		{"colon_quote_string", `line:"2025/12/07 14:04:35"`, func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code for colon+quote, got %+v", segs)
			}
		}},
		{"colon_quote_path", `path:"/usr/local/bin"`, func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code for colon+quote, got %+v", segs)
			}
		}},
		// ── 英文冒号分隔（应分割）──
		{"en_colon_fix_error", "fix this error: Module not found in ./src", func(t *testing.T, segs []Segment) {
			if len(segs) < 2 {
				t.Errorf("expected split, got %d segs: %+v", len(segs), segs)
			}
		}},
		{"en_colon_code", "Here's the code: def add(a, b): return a + b", func(t *testing.T, segs []Segment) {
			if len(segs) < 2 {
				t.Errorf("expected split, got %d segs: %+v", len(segs), segs)
			}
		}},
		// ── 中文冒号分隔（应分割）──
		{"zh_colon_code", "代码如下：def my_function(x): return x * 2", func(t *testing.T, segs []Segment) {
			if len(segs) < 2 {
				t.Errorf("expected split, got %d segs: %+v", len(segs), segs)
			}
		}},
		{"zh_colon_error", "报错：Error: Cannot find module 'express'", func(t *testing.T, segs []Segment) {
			if len(segs) < 2 {
				t.Errorf("expected split, got %d segs: %+v", len(segs), segs)
			}
		}},
		{"zh_colon_error_long", "错误信息：ModuleNotFoundError: No module named 'requests'", func(t *testing.T, segs []Segment) {
			if len(segs) < 2 {
				t.Errorf("expected split, got %d segs: %+v", len(segs), segs)
			}
		}},
		// ── 句尾标点分隔（应分割）──
		{"period_sql", "This query is slow. SELECT * FROM users WHERE id = 1", func(t *testing.T, segs []Segment) {
			if len(segs) < 2 {
				t.Errorf("expected split, got %d segs: %+v", len(segs), segs)
			}
		}},
		{"period_shell", "I need a command. ls -la | grep foo", func(t *testing.T, segs []Segment) {
			if len(segs) < 2 {
				t.Errorf("expected split, got %d segs: %+v", len(segs), segs)
			}
		}},
		{"zh_period_sql", "优化一下这个查询。SELECT o.id FROM orders", func(t *testing.T, segs []Segment) {
			if len(segs) < 2 {
				t.Errorf("expected split, got %d segs: %+v", len(segs), segs)
			}
		}},
		// ── 行内混合（无分隔符）──
		{"errfunc_main", "fix errfunc main()", func(t *testing.T, segs []Segment) {
			if len(segs) < 2 {
				t.Errorf("expected split, got %d segs: %+v", len(segs), segs)
			}
			if len(segs) >= 2 && segs[1].Type != SegmentCode {
				t.Errorf("second segment should be code, got %s: %q", segs[1].Type, segs[1].Content)
			}
		}},
		{"errfunc_main_braces", "fix errfunc main() {}", func(t *testing.T, segs []Segment) {
			if len(segs) < 2 {
				t.Errorf("expected split, got %d segs: %+v", len(segs), segs)
			}
			if len(segs) >= 2 && segs[1].Type != SegmentCode {
				t.Errorf("second segment should be code, got %s: %q", segs[1].Type, segs[1].Content)
			}
		}},
		{"embedded_func", "check thisfunc test() { return 1 }", func(t *testing.T, segs []Segment) {
			if len(segs) < 2 {
				t.Errorf("expected split, got %d segs: %+v", len(segs), segs)
			}
		}},
		// ── 代码段末尾中文混入 ──
		{"code_tail_chinese", "                        },这是什么", func(t *testing.T, segs []Segment) {
			chineseInPrompt := false
			for _, s := range segs {
				if s.Type == SegmentPrompt && strings.Contains(s.Content, "这是什么") {
					chineseInPrompt = true
				}
			}
			if !chineseInPrompt {
				t.Errorf("expected '这是什么' in prompt, got %+v", segs)
			}
		}},
		{"code_tail_chinese2", "                        },\n                        },\n                        这段代码有问题吗", func(t *testing.T, segs []Segment) {
			chineseInPrompt := false
			for _, s := range segs {
				if s.Type == SegmentPrompt && strings.Contains(s.Content, "这段代码有问题吗") {
					chineseInPrompt = true
				}
			}
			if !chineseInPrompt {
				t.Errorf("expected Chinese in prompt, got %+v", segs)
			}
		}},
		// ── 日志段末尾中文混入 ──
		{"log_tail_chinese", "Error: test\n咋回事啊", func(t *testing.T, segs []Segment) {
			chineseInPrompt := false
			for _, s := range segs {
				if s.Type == SegmentPrompt && strings.Contains(s.Content, "咋回事啊") {
					chineseInPrompt = true
				}
			}
			if !chineseInPrompt {
				t.Errorf("expected Chinese in prompt, got %+v", segs)
			}
		}},
		{"log_tail_chinese2", "2026/07/04 [ERROR] failed\n怎么办", func(t *testing.T, segs []Segment) {
			chineseInPrompt := false
			for _, s := range segs {
				if s.Type == SegmentPrompt && strings.Contains(s.Content, "怎么办") {
					chineseInPrompt = true
				}
			}
			if !chineseInPrompt {
				t.Errorf("expected Chinese in prompt, got %+v", segs)
			}
		}},
		// ── 行首中文混入日志 ──
		{"log_head_chinese", "咋回事2026/07/04 [INFO] started", func(t *testing.T, segs []Segment) {
			promptFound := false
			logFound := false
			for _, s := range segs {
				if s.Type == SegmentPrompt && strings.Contains(s.Content, "咋回事") {
					promptFound = true
				}
				if s.Type == SegmentLog {
					logFound = true
				}
			}
			if !promptFound || !logFound {
				t.Errorf("expected prompt+log split, got %+v", segs)
			}
		}},
		{"log_head_chinese_multi", "为啥呀2026/07/04 [ERROR] crash\n2026/07/04 [INFO] recovery", func(t *testing.T, segs []Segment) {
			promptFound := false
			logFound := false
			for _, s := range segs {
				if s.Type == SegmentPrompt && strings.Contains(s.Content, "为啥呀") {
					promptFound = true
				}
				if s.Type == SegmentLog {
					logFound = true
				}
			}
			if !promptFound || !logFound {
				t.Errorf("expected prompt+log split, got %+v", segs)
			}
		}},
		// ── JSON 内容 ──
		{"json_with_chinese_tail", "{\n  \"files\": [],\n  \"references\": []\n}\n解释一下", func(t *testing.T, segs []Segment) {
			codeFound := false
			chineseInPrompt := false
			for _, s := range segs {
				if s.Type == SegmentCode {
					codeFound = true
				}
				if s.Type == SegmentPrompt && strings.Contains(s.Content, "解释一下") {
					chineseInPrompt = true
				}
			}
			if !codeFound || !chineseInPrompt {
				t.Errorf("expected code + prompt(\"解释一下\"), got %+v", segs)
			}
		}},
		{"json_key_value", `"references": [`, func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code for JSON key, got %+v", segs)
			}
		}},
		{"json_object_comma", `    { "path": "./tsconfig.json" },`, func(t *testing.T, segs []Segment) {
			if len(segs) != 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code for JSON object, got %+v", segs)
			}
		}},
		{"json_full_block", "{\n  \"files\": [],\n  \"references\": [\n    { \"path\": \"./a.json\" },\n    { \"path\": \"./b.json\" }\n  ]\n}", func(t *testing.T, segs []Segment) {
			if len(segs) < 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code for JSON, got %+v", segs)
			}
		}},
		// ── 多行混合 ──
		{"multiline_prompt_code", "fix this\n\nfunction foo() {\n  return 1;\n}", func(t *testing.T, segs []Segment) {
			promptFound := false
			codeFound := false
			for _, s := range segs {
				if s.Type == SegmentPrompt {
					promptFound = true
				}
				if s.Type == SegmentCode {
					codeFound = true
				}
			}
			if !promptFound || !codeFound {
				t.Errorf("expected prompt+code, got %+v", segs)
			}
		}},
		{"multiline_prompt_log", "看下这个报错\n\nError: listen EADDRINUSE\n    at net.js:1330", func(t *testing.T, segs []Segment) {
			promptFound := false
			logFound := false
			for _, s := range segs {
				if s.Type == SegmentPrompt {
					promptFound = true
				}
				if s.Type == SegmentLog {
					logFound = true
				}
			}
			if !promptFound || !logFound {
				t.Errorf("expected prompt+log, got %+v", segs)
			}
		}},
		// ── 杂项 ──
		{"shell_command", "pip install requests", func(t *testing.T, segs []Segment) {
			if len(segs) < 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code for shell cmd, got %+v", segs)
			}
		}},
		{"inline_sql", "optimize this. EXPLAIN ANALYZE SELECT * FROM users", func(t *testing.T, segs []Segment) {
			if len(segs) < 2 {
				t.Errorf("expected split, got %d segs: %+v", len(segs), segs)
			}
		}},
		{"struct_with_struct", "type Config struct {\n    DB     *DBConfig\n    Port   int\n    Name   string\n}", func(t *testing.T, segs []Segment) {
			for _, s := range segs {
				if s.Type != SegmentCode {
					t.Errorf("expected all code for struct, got prompt: %q", s.Content)
				}
			}
		}},
		{"inline_type_assertion", `val := x.(string)`, func(t *testing.T, segs []Segment) {
			if len(segs) < 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code, got %+v", segs)
			}
		}},
		{"docker_build_log", "Step 5/10 : RUN apt-get update\n ---> abc123\nE: Unable to locate package", func(t *testing.T, segs []Segment) {
			logFound := false
			for _, s := range segs {
				if s.Type == SegmentLog {
					logFound = true
				}
			}
			if !logFound {
				t.Errorf("expected log segment, got %+v", segs)
			}
		}},
		{"stack_trace", "Error: Cannot read properties of undefined\n    at processData (/app/src/utils.js:15:23)", func(t *testing.T, segs []Segment) {
			logFound := false
			for _, s := range segs {
				if s.Type == SegmentLog {
					logFound = true
				}
			}
			if !logFound {
				t.Errorf("expected log with stack trace, got %+v", segs)
			}
		}},
		{"code_comment_chinese", "func foo() {\n    // 这段代码处理数据\n    return x;\n}", func(t *testing.T, segs []Segment) {
			for _, s := range segs {
				if s.Type == SegmentLog && strings.Contains(s.Content, "处理数据") {
					t.Errorf("Chinese comment should not be log: %q", s.Content)
				}
			}
		}},
		{"this_keyword_code", "this.value = 42", func(t *testing.T, segs []Segment) {
			if len(segs) < 1 || segs[0].Type != SegmentCode {
				t.Errorf("expected code for this.property, got %+v", segs)
			}
		}},
		{"trailing_newline_preserved", "line1\nline2", func(t *testing.T, segs []Segment) {
			if len(segs) == 1 && !strings.Contains(segs[0].Content, "\n") {
				t.Errorf("expected newline between lines preserved: %q", segs[0].Content)
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segs := SplitString(tt.input)
			tt.check(t, segs)
		})
	}
}

// ── 第二批边界样例（用户真实反例）──
func TestEdgeCases2(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, segs []Segment)
	}{
		// OpenAPI/YAML key-value 不应在 colon 处被分割
		{"yaml_openapi", "openapi: 3.0.3\ninfo:\n  title: eqsys backend API", func(t *testing.T, segs []Segment) {
			promptParts := 0
			for _, s := range segs {
				if s.Type == SegmentPrompt {
					promptParts++
				}
			}
			if promptParts > 1 {
				t.Errorf("expected max 1 prompt segment (leading), got %d: %+v", promptParts, segs)
			}
		}},
		// Python def + docstring + code body 应保持为 code
		{"python_docstring_code", "def main114():\n    \"\"\"主函数\"\"\"\n    # 加载配置\n    if not os.path.exists(\"config.json\"):\n        print(\"加载中\")", func(t *testing.T, segs []Segment) {
			codeFound := false
			for _, s := range segs {
				if s.Type == SegmentCode && strings.Contains(s.Content, "print") {
					codeFound = true
				}
			}
			if !codeFound {
				t.Errorf("expected code for Python body, got %+v", segs)
			}
		}},
		// Shell echo 命令应为 code
		{"shell_echo", "cd \"$PROJECT_ROOT\"\necho \"同步依赖...\"", func(t *testing.T, segs []Segment) {
			echoInCode := false
			for _, s := range segs {
				if s.Type == SegmentCode && strings.Contains(s.Content, "echo") {
					echoInCode = true
				}
			}
			if !echoInCode {
				t.Errorf("expected echo in code, got %+v", segs)
			}
		}},
		// Docker Compose 全部应为 code
		{"yaml_compose", "services:\n  nats:\n    image: nats:latest\n    environment:\n      - NATS_USER=root", func(t *testing.T, segs []Segment) {
			allCode := true
			for _, s := range segs {
				if s.Type != SegmentCode {
					allCode = false
					t.Errorf("unexpected prompt: %q", s.Content)
				}
			}
			if !allCode {
				t.Errorf("expected all code for compose")
			}
		}},
		// Vue config + Chinese tail
		{"vue_config", "export default defineConfig({\n  plugins: [vue()],\n  resolve: {\n    alias: {\n      \"@\": fileURLToPath()\n    },\n  },\n})修一下", func(t *testing.T, segs []Segment) {
			chineseInPrompt := false
			codeFound := false
			for _, s := range segs {
				if s.Type == SegmentPrompt && strings.Contains(s.Content, "修一下") {
					chineseInPrompt = true
				}
				if s.Type == SegmentCode && strings.Contains(s.Content, "plugins") {
					codeFound = true
				}
			}
			if !codeFound || !chineseInPrompt {
				t.Errorf("expected code+prompt(\"修一下\"), got %+v", segs)
			}
		}},
		// YAML/JSON with Chinese tail
		{"yaml_chinese_tail", "  \"deepseek_key\": \"sk-xxx\",\n  \"log_level\": \"INFO\"\n这是咋回事", func(t *testing.T, segs []Segment) {
			chineseInPrompt := false
			codeFound := false
			for _, s := range segs {
				if s.Type == SegmentPrompt && strings.Contains(s.Content, "这是咋回事") {
					chineseInPrompt = true
				}
				if s.Type == SegmentCode {
					codeFound = true
				}
			}
			if !codeFound || !chineseInPrompt {
				t.Errorf("expected code+prompt, got %+v", segs)
			}
		}},
		// Python ANSI code body
		{"python_ansi_code", "    for i, char in enumerate(text):\n        if (char == '\\n'):\n            result.append(current_line)", func(t *testing.T, segs []Segment) {
			codeFound := false
			for _, s := range segs {
				if s.Type == SegmentCode {
					codeFound = true
				}
			}
			if !codeFound {
				t.Errorf("expected code for Python, got %+v", segs)
			}
		}},
		// Compose/YAML port mapping + Chinese tail
		{"yaml_port_chinese_tail", "services:\n  nats:\n    ports:\n      - \"4222:4222\"\n给端口号改了", func(t *testing.T, segs []Segment) {
			chineseInPrompt := false
			codeFound := false
			for _, s := range segs {
				if s.Type == SegmentPrompt && strings.Contains(s.Content, "给端口号改了") {
					chineseInPrompt = true
				}
				if s.Type == SegmentCode {
					codeFound = true
				}
			}
			if !codeFound || !chineseInPrompt {
				t.Errorf("expected code+prompt, got %+v", segs)
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segs := SplitString(tt.input)
			tt.check(t, segs)
		})
	}
}
