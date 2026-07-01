// Command eval 评估规则引擎在 testdata.jsonl 上的性能
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cxykevin/alkaid0-ctx-rulesys/rulesys"
)

type testRecord struct {
	Original string `json:"original"`
	Tagged   string `json:"tagged"`
}

type groundTruth struct {
	Type    rulesys.SegmentType
	Content string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "用法: %s <testdata.jsonl>\n", os.Args[0])
		os.Exit(1)
	}

	path := os.Args[1]
	records, err := loadRecords(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载数据失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("加载 %d 条记录\n\n", len(records))

	totalCorrect := 0
	totalChars := 0
	promptCorrect := 0
	codeCorrect := 0
	logCorrect := 0
	promptTotal := 0
	codeTotal := 0
	logTotal := 0
	errorCases := 0

	for i, rec := range records {
		if len(rec.Original) < 3 {
			continue
		}

		gts := parseTagged(rec.Tagged)
		if len(gts) == 0 {
			continue
		}

		segs := rulesys.SplitString(rec.Original)

		// Build ground truth type map
		gtTypes := buildTypeMap(rec.Original, gts)

		// Build predicted type map
		predTypes := buildPredTypeMap(rec.Original, segs)

		// Count correct chars
		charsCorrect := 0
		pCorrect, cCorrect, lCorrect := 0, 0, 0
		pTotal, cTotal, lTotal := 0, 0, 0

		for j := 0; j < len(rec.Original) && j < len(gtTypes) && j < len(predTypes); j++ {
			switch gtTypes[j] {
			case rulesys.SegmentPrompt:
				pTotal++
			case rulesys.SegmentCode:
				cTotal++
			case rulesys.SegmentLog:
				lTotal++
			}

			if predTypes[j] == gtTypes[j] {
				charsCorrect++
				switch gtTypes[j] {
				case rulesys.SegmentPrompt:
					pCorrect++
				case rulesys.SegmentCode:
					cCorrect++
				case rulesys.SegmentLog:
					lCorrect++
				}
			}
		}

		promptCorrect += pCorrect
		codeCorrect += cCorrect
		logCorrect += lCorrect
		promptTotal += pTotal
		codeTotal += cTotal
		logTotal += lTotal
		totalCorrect += charsCorrect
		totalChars += len(rec.Original)

		accuracy := float64(charsCorrect) / float64(len(rec.Original))
		if accuracy < 0.5 {
			errorCases++
			if errorCases <= 3 {
				fmt.Printf("⚠️  Record %d accuracy=%.1f%%\n", i, accuracy*100)
				fmt.Printf("   expected: %s\n", summarizeTypes(rec.Original, gtTypes))
				fmt.Printf("   predicted: %s\n", summarizeTypes(rec.Original, predTypes))
			}
		}
	}

	overallAccuracy := float64(totalCorrect) / float64(totalChars)
	promptPrecision := float64(promptCorrect) / float64(max(1, promptTotal))
	codePrecision := float64(codeCorrect) / float64(max(1, codeTotal))
	logPrecision := float64(logCorrect) / float64(max(1, logTotal))

	fmt.Println("\n=== 批量评估结果 ===")
	fmt.Printf("样本数:     %d\n", len(records))
	fmt.Printf("总字符数:   %d\n", totalChars)
	fmt.Printf("------------------------\n")
	fmt.Printf("整体准确率: %.2f%%\n", overallAccuracy*100)
	fmt.Printf("Prompt 准确率: %.2f%% (%d/%d)\n", promptPrecision*100, promptCorrect, promptTotal)
	fmt.Printf("Code   准确率: %.2f%% (%d/%d)\n", codePrecision*100, codeCorrect, codeTotal)
	fmt.Printf("Log    准确率: %.2f%% (%d/%d)\n", logPrecision*100, logCorrect, logTotal)
	fmt.Printf("低准确率样本数: %d\n", errorCases)
}

func loadRecords(path string) ([]testRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var records []testRecord
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec testRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		records = append(records, rec)
	}
	return records, scanner.Err()
}

func parseTagged(tagged string) []groundTruth {
	var gts []groundTruth
	tagDefs := []struct {
		start string
		end   string
		st    rulesys.SegmentType
	}{
		{"<|tag_prompt_start|>", "<|tag_prompt_end|>", rulesys.SegmentPrompt},
		{"<|tag_code_start|>", "<|tag_code_end|>", rulesys.SegmentCode},
		{"<|tag_log_start|>", "<|tag_log_end|>", rulesys.SegmentLog},
		{"<|tag_info_start|>", "<|tag_info_end|>", rulesys.SegmentPrompt},
		{"<|tag_rule_start|>", "<|tag_rule_end|>", rulesys.SegmentPrompt},
		{"<|tag_link_start|>", "<|tag_link_end|>", rulesys.SegmentPrompt},
	}

	pos := 0
	for pos < len(tagged) {
		bestIdx := -1
		var bestDef struct {
			start string
			end   string
			st    rulesys.SegmentType
		}

		for _, d := range tagDefs {
			idx := strings.Index(tagged[pos:], d.start)
			if idx >= 0 && (bestIdx < 0 || idx < bestIdx) {
				bestIdx = idx
				bestDef = d
			}
		}

		if bestIdx < 0 {
			break
		}

		contentStart := pos + bestIdx + len(bestDef.start)
		contentEnd := strings.Index(tagged[contentStart:], bestDef.end)
		if contentEnd < 0 {
			break
		}

		content := tagged[contentStart : contentStart+contentEnd]
		gts = append(gts, groundTruth{Type: bestDef.st, Content: content})
		pos = contentStart + contentEnd + len(bestDef.end)
	}
	return gts
}

func buildTypeMap(original string, gts []groundTruth) []rulesys.SegmentType {
	types := make([]rulesys.SegmentType, len(original))
	for i := range types {
		types[i] = rulesys.SegmentPrompt
	}

	gtPos := 0
	for _, gt := range gts {
		trimmed := strings.TrimSpace(gt.Content)
		start := strings.Index(original[gtPos:], trimmed)
		if start < 0 {
			start = 0
		} else {
			start += gtPos
		}
		end := start + len(trimmed)
		if end > len(original) {
			end = len(original)
		}
		for j := start; j < end; j++ {
			types[j] = gt.Type
		}
		gtPos = end
	}
	return types
}

func buildPredTypeMap(original string, segs []rulesys.Segment) []rulesys.SegmentType {
	types := make([]rulesys.SegmentType, len(original))
	for i := range types {
		types[i] = rulesys.SegmentPrompt
	}

	predPos := 0
	for _, seg := range segs {
		if predPos >= len(original) {
			break
		}
		trimmed := strings.TrimSpace(seg.Content)
		start := strings.Index(original[predPos:], trimmed)
		if start < 0 {
			predPos += len(trimmed)
			if predPos > len(original) {
				predPos = len(original)
			}
			continue
		}
		start += predPos
		end := start + len(trimmed)
		if end > len(original) {
			end = len(original)
		}
		for j := start; j < end; j++ {
			types[j] = seg.Type
		}
		predPos = end
	}
	return types
}

func summarizeTypes(original string, types []rulesys.SegmentType) string {
	if len(original) == 0 {
		return ""
	}
	type summaryItem struct {
		Type rulesys.SegmentType
		Text string
	}
	var items []summaryItem
	current := types[0]
	start := 0
	for i := 1; i <= len(types); i++ {
		var t rulesys.SegmentType
		if i < len(types) {
			t = types[i]
		} else {
			t = -1
		}
		if t != current || i == len(types) {
			text := original[start:i]
			items = append(items, summaryItem{Type: current, Text: text})
			current = t
			start = i
		}
	}
	var parts []string
	for _, item := range items {
		label := item.Type.String()
		text := strings.ReplaceAll(item.Text, "\n", "↵")
		if len(text) > 40 {
			text = text[:40] + "..."
		}
		parts = append(parts, label+"("+text+")")
	}
	return strings.Join(parts, " → ")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
