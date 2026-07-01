# alkaid0-ctx-rulesys

一个基于规则的状态机引擎，将用户向 Coding Agent 的输入自动分割为 **prompt / code / log** 三类片段。

## 解决的问题

Coding Agent 的用户输入常混合自然语言描述、代码片段和错误日志。本引擎在 O(n) 时间内流式处理输入，自动识别每段内容的类型，为下游任务（如上下文压缩、prompt 构造、日志分析）提供结构化分段。

## 工作原理

1. **逐行分类**：每行通过多维模式匹配分别获得 code 和 log 的评分
2. **状态机转换**：基于评分驱动 PROMPT ↔ CODE ↔ LOG 状态转换，带惰性确认机制（连续 3 行失配才切换），避免误触发
3. **代码围栏跟踪**：` ``` ` 成对处理，围栏内强制为代码块
4. **零第三方依赖**：仅使用 Go 标准库

## 安装

```bash
go get github.com/cxykevin/alkaid0-ctx-rulesys
```

## 使用

```go
package main

import (
    "fmt"
    "github.com/cxykevin/alkaid0-ctx-rulesys/rulesys"
)

func main() {
    input := "fix this port conflict\n\nError: listen EADDRINUSE: address already in use :::3000"
    segs := rulesys.SplitString(input)

    for _, s := range segs {
        fmt.Printf("[%s] %s\n", s.Type, s.Content)
    }
}
// 输出:
// [prompt] fix this port conflict
// [log] Error: listen EADDRINUSE: address already in use :::3000
```

## API

| 函数 | 说明 |
|------|------|
| `SplitString(input string) []Segment` | 处理字符串输入 |
| `SplitReader(reader io.Reader) ([]Segment, error)` | 从 io.Reader 流式读取处理 |
| `NewEngine() *Engine` | 创建可复用的引擎实例（含 ProcessString/Process 方法） |

### Segment 类型

```go
type SegmentType int
const (
    SegmentPrompt SegmentType = iota  // 自然语言描述/问题
    SegmentCode                       // 代码片段
    SegmentLog                        // 错误日志/运行输出
)

type Segment struct {
    Type    SegmentType
    Content string
}
```

## 支持的代码语言

Go / Python / JavaScript / TypeScript / Java / C / C++ / Rust / Ruby / PHP / SQL / Shell / Dockerfile / Nginx 配置 / YAML / JSON 等。

## 支持的日志格式

- JavaScript/Python/Java 堆栈跟踪
- npm/yarn 输出
- Docker 构建日志
- CI/CD 错误输出
- 带时间戳和日志级别的日志行
- 编译错误信息
- 测试失败输出
- apt/dpkg 错误
- 各类异常类名

## 算法特性

- **O(n) 流式处理**：单遍扫描，每行 O(1) 评分
- **状态机**：PROMPT（初始）→ CODE/LOG 双向转换
- **惰性转换**：连续 3 行匹配新类型才切换，避免毛刺
- **代码围栏优先**：` ``` ` 成对处理，围栏内强制代码
- **自然语言过滤**：shell 命令检测时排除自然语言误判
- **Key-Value 模式**：识别 `key: value` 对象字面量结构

## 评估

在 5312 条样本上的字符级准确率：

| 类型 | 准确率 |
|------|--------|
| 整体 | ~70% |
| code | ~88% |
| log  | ~54% |
| prompt | ~65% |

> 注：低准确率样本主要来自 ground truth 标注不一致（如中文标题"代码："被标注为代码块，或部分文本未被标注）。
