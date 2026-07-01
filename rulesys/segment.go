package rulesys

// SegmentType 表示输入内容的一个分段类型
type SegmentType int

const (
	// SegmentPrompt 是自然语言描述/问题/指示（包括 info/rule/link 等辅助文本）
	SegmentPrompt SegmentType = iota
	// SegmentCode 是代码片段（各种语言）
	SegmentCode
	// SegmentLog 是错误日志、堆栈跟踪、构建输出等
	SegmentLog
)

// String 返回 SegmentType 的可读名称
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

// NewSegment 创建一个新的分段
func NewSegment(st SegmentType, content string) Segment {
	return Segment{Type: st, Content: content}
}
