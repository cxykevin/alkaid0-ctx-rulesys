package rulesys

// SegmentType 表示输入内容的一个分段类型
type SegmentType int

const (
	SegmentPrompt SegmentType = iota // 自然语言描述/问题/指示
	SegmentCode                      // 代码片段（各种语言）
	SegmentLog                       // 错误日志、堆栈跟踪、构建输出
)

func (st SegmentType) String() string {
	switch st {
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

// Segment 表示输入中的一个分段
type Segment struct {
	Type    SegmentType
	Content string
}

func NewSegment(st SegmentType, content string) Segment {
	return Segment{Type: st, Content: content}
}
