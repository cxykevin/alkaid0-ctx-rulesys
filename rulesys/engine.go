package rulesys

import (
	"bufio"
	"io"
	"strings"
)

type engineState int

const (
	statePrompt engineState = iota
	stateCode
	stateLog
)

// Engine 将输入流分割为 prompt/code/log 段
type Engine struct {
	transitionThreshold int
	fenceOpen           bool
	fenceLang           string
	consecutiveNonMatch int
	blanksSinceSwitch   int // 切换后积累的空行数
}

func NewEngine() *Engine {
	return &Engine{transitionThreshold: 12}
}

func SplitString(input string) []Segment {
	return NewEngine().ProcessString(input)
}

func SplitReader(r io.Reader) ([]Segment, error) {
	return NewEngine().Process(r)
}

func (e *Engine) ProcessString(input string) []Segment {
	return e.processLines(strings.Split(input, "\n"))
}

func (e *Engine) Process(reader io.Reader) ([]Segment, error) {
	var lines []string
	sc := bufio.NewScanner(reader)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return e.processLines(lines), nil
}

func (e *Engine) processLines(lines []string) []Segment {
	e.reset()

	var segments []Segment
	var cur strings.Builder
	state := statePrompt
	th := e.transitionThreshold

	flush := func(st engineState) {
		if cur.Len() == 0 {
			return
		}
		t := SegmentPrompt
		switch st {
		case stateCode:
			t = SegmentCode
		case stateLog:
			t = SegmentLog
		}
		segments = append(segments, NewSegment(t, cur.String()))
		cur.Reset()
	}

	for _, line := range lines {
		cl := classifyLine(line)

		// ── 代码围栏（最高优先级）──
		if cl.isCodeFence {
			if e.fenceOpen {
				cur.WriteString(line)
				cur.WriteByte('\n')
				e.fenceOpen = false
				flush(stateCode)
				state = statePrompt
				e.blanksSinceSwitch = 0
			} else {
				flush(state)
				e.fenceOpen = true
				e.fenceLang = cl.hasCodeFenceLang
				state = stateCode
				e.blanksSinceSwitch = 0
				cur.WriteString(line)
				cur.WriteByte('\n')
			}
			continue
		}

		// ── 围栏内：强制代码 ──
		if e.fenceOpen {
			cur.WriteString(line)
			cur.WriteByte('\n')
			continue
		}

		// ── 空行处理 ──
		if isBlankLine(line) {
			cur.WriteByte('\n')
			e.blanksSinceSwitch++
			continue
		}

		// ── 计算最佳类型 ──
		best := statePrompt
		if cl.codeScore >= th && cl.codeScore >= cl.logScore {
			best = stateCode
		} else if cl.logScore >= th && cl.logScore > cl.codeScore {
			best = stateLog
		}

		// ── 状态转换 ──
		if best == state {
			e.consecutiveNonMatch = 0
			cur.WriteString(line)
			cur.WriteByte('\n')
			continue
		}

		// 当前行判定为 prompt，但在 code/log 段内
		if best == statePrompt && (state == stateCode || state == stateLog) {
			// 空行后允许更快切换（threshold=1）
			nonMatchThreshold := 2
			if e.blanksSinceSwitch > 0 {
				nonMatchThreshold = 1
			}
			e.consecutiveNonMatch++
			cur.WriteString(line)
			cur.WriteByte('\n')
			if e.consecutiveNonMatch >= nonMatchThreshold {
				// 把最后 consecutiveNonMatch 行划入新 prompt 段
				content := cur.String()
				// 找到倒数 nonMatchThreshold 行的起点
				splitPos := findSplitPos(content, nonMatchThreshold)
				codeContent := content[:splitPos]
				promptContent := content[splitPos:]
				cur.Reset()
				if strings.TrimSpace(codeContent) != "" {
					segments = append(segments, NewSegment(segTypeOf(state), codeContent))
				}
				cur.WriteString(promptContent)
				state = statePrompt
				e.consecutiveNonMatch = 0
				e.blanksSinceSwitch = 0
			}
			continue
		}

		// prompt → code/log 或 code ↔ log
		flush(state)
		state = best
		e.consecutiveNonMatch = 0
		e.blanksSinceSwitch = 0
		cur.WriteString(line)
		cur.WriteByte('\n')
	}

	if cur.Len() > 0 {
		flush(state)
	}

	trimSegments(segments)
	return splitIntraLine(segments, th)
}

// findSplitPos 在 content 中找到"倒数 n 个非空行"的起始字节位置
func findSplitPos(content string, n int) int {
	lines := strings.Split(content, "\n")
	// 从后往前找 n 个非空行
	count := 0
	splitLine := len(lines)
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			count++
			if count == n {
				splitLine = i
				break
			}
		}
	}
	if splitLine == len(lines) {
		return 0
	}
	// 计算字节偏移
	pos := 0
	for i := 0; i < splitLine; i++ {
		pos += len(lines[i]) + 1 // +1 for '\n'
	}
	return pos
}

func segTypeOf(st engineState) SegmentType {
	switch st {
	case stateCode:
		return SegmentCode
	case stateLog:
		return SegmentLog
	default:
		return SegmentPrompt
	}
}

func (e *Engine) reset() {
	e.fenceOpen = false
	e.fenceLang = ""
	e.consecutiveNonMatch = 0
	e.blanksSinceSwitch = 0
}

func trimSegments(segments []Segment) {
	for i, seg := range segments {
		segments[i].Content = strings.TrimRight(seg.Content, "\n")
	}
	for i := 0; i < len(segments)-1; i++ {
		segments[i].Content += "\n"
	}
}
