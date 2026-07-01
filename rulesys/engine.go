package rulesys

import (
	"bufio"
	"io"
	"strings"
)

// engineState 状态机内部状态
type engineState int

const (
	statePrompt engineState = iota
	stateCode
	stateLog
)

// Engine 是流式规则引擎，将输入分割为 prompt/code/log 段
type Engine struct {
	// transitionThreshold 从 PROMPT 转换的最小得分
	transitionThreshold int
	// fenceOpen 标记当前是否在 ``` 代码围栏内
	fenceOpen bool
	// fenceLang 记录围栏的语言标记
	fenceLang string
	// consecutiveNonMatch 记录当前状态下连续不匹配的行数
	consecutiveNonMatch int
}

// NewEngine 创建一个新的规则引擎
func NewEngine() *Engine {
	return &Engine{
		transitionThreshold: 12,
	}
}

// ProcessString 处理输入字符串并返回分割后的段
func (e *Engine) ProcessString(input string) []Segment {
	return e.processLines(strings.Split(input, "\n"))
}

// Process 从 io.Reader 中流式读取并处理
func (e *Engine) Process(reader io.Reader) ([]Segment, error) {
	var lines []string
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return e.processLines(lines), nil
}

// processLines 处理按行分割的输入
func (e *Engine) processLines(lines []string) []Segment {
	e.reset()
	var segments []Segment
	var current strings.Builder
	currentState := statePrompt

	// 延迟 flush 辅助函数
	flush := func(forceState engineState) {
		if current.Len() > 0 {
			segType := SegmentPrompt
			switch forceState {
			case stateCode:
				segType = SegmentCode
			case stateLog:
				segType = SegmentLog
			}
			segments = append(segments, NewSegment(segType, current.String()))
			current.Reset()
		}
	}

	for _, line := range lines {
		cl := classifyLine(line)

		// ── 代码围栏处理 —— 优先级最高 ──
		if cl.isCodeFence {
			if e.fenceOpen {
				// 闭合围栏
				current.WriteString(line)
				current.WriteByte('\n')
				e.fenceOpen = false
				e.fenceLang = ""
				// 闭合后刷新当前段，下段回到 prompt
				flush(stateCode)
				currentState = statePrompt
				continue
			} else {
				// 打开围栏
				flush(currentState)
				e.fenceOpen = true
				e.fenceLang = cl.hasCodeFenceLang
				currentState = stateCode
				current.WriteString(line)
				current.WriteByte('\n')
				continue
			}
		}

		// ── 在代码围栏内：强制为代码 ──
		if e.fenceOpen {
			current.WriteString(line)
			current.WriteByte('\n')
			currentState = stateCode
			continue
		}

		// ── 空行处理 ──
		if isBlankLine(line) {
			// 空行不触发状态切换，仅追加到当前段
			current.WriteByte('\n')
			continue
		}

		// ── 根据得分确定行的最佳类型 ──
		bestType := statePrompt
		codeScore := cl.codeScore
		logScore := cl.logScore

		if codeScore >= e.transitionThreshold && codeScore >= logScore {
			bestType = stateCode
		} else if logScore >= e.transitionThreshold && logScore > codeScore {
			bestType = stateLog
		}

		// ── 状态转换逻辑 ──

		if bestType == currentState {
			// 同一状态：追加行，重置计数
			e.consecutiveNonMatch = 0
			current.WriteString(line)
			current.WriteByte('\n')
			continue
		}

		if bestType == statePrompt {
			// 当前行判定为 prompt，但可能在 code/log 段内
			// 在 CODE 或 LOG 段内出现 prompt 行时，惰性转换
			if currentState == stateCode || currentState == stateLog {
				e.consecutiveNonMatch++
				if e.consecutiveNonMatch >= 3 {
					// 连续 2 行非代码/日志 → 刷新并切换回 prompt
					flush(currentState)
					currentState = statePrompt
					e.consecutiveNonMatch = 0
					current.WriteString(line)
					current.WriteByte('\n')
				} else {
					// 仅 1 行非匹配：暂不切换，但记录
					current.WriteString(line)
					current.WriteByte('\n')
				}
				continue
			}
			// 已在 prompt 状态下，直接追加
			current.WriteString(line)
			current.WriteByte('\n')
			e.consecutiveNonMatch = 0
			continue
		}

		// bestType != currentState 且 bestType 非 prompt
		// 从 prompt → code 或 prompt → log
		if currentState == statePrompt {
			if current.Len() > 0 {
				flush(statePrompt)
			}
			currentState = bestType
			e.consecutiveNonMatch = 0
			current.WriteString(line)
			current.WriteByte('\n')
			continue
		}

		// CODE ↔ LOG 之间的转换
		if bestType == stateCode && currentState == stateLog {
			// 从 log → code
			flush(stateLog)
			currentState = stateCode
			e.consecutiveNonMatch = 0
			current.WriteString(line)
			current.WriteByte('\n')
			continue
		}

		if bestType == stateLog && currentState == stateCode {
			// 从 code → log
			flush(stateCode)
			currentState = stateLog
			e.consecutiveNonMatch = 0
			current.WriteString(line)
			current.WriteByte('\n')
			continue
		}

		// fallback：追加到当前段
		current.WriteString(line)
		current.WriteByte('\n')
	}

	// 刷新最后一段
	if current.Len() > 0 {
		flush(currentState)
	} else if e.fenceOpen {
		// 未闭合的围栏：将围栏之前的代码段刷新
		// fenceOpen 状态下 current 为空时不做额外处理
	}

	// 清理：去除每段首尾的空白/换行
	trimSegments(segments)

	return segments
}

// reset 重置引擎状态
func (e *Engine) reset() {
	e.fenceOpen = false
	e.fenceLang = ""
	e.consecutiveNonMatch = 0
}

// trimSegments 去除每段首尾的换行和空白
func trimSegments(segments []Segment) {
	for i, seg := range segments {
		segments[i].Content = strings.Trim(seg.Content, "\n")
	}
}

// SplitString 便捷函数：直接处理字符串
func SplitString(input string) []Segment {
	engine := NewEngine()
	return engine.ProcessString(input)
}

// SplitReader 便捷函数：从 io.Reader 处理
func SplitReader(reader io.Reader) ([]Segment, error) {
	engine := NewEngine()
	return engine.Process(reader)
}
