package utility

import (
	"strings"
	"testing"
)

func TestSanitizeOutput_BracketRemoval(t *testing.T) {
	input := "统计口径：来源于网格平台（case_list），处理步骤（pending_step）"
	output := SanitizeOutput(input)
	if strings.Contains(output, "case_list") {
		t.Errorf("should remove bracket: got %q", output)
	}
	if strings.Contains(output, "pending_step") {
		t.Errorf("should remove bracket: got %q", output)
	}
	if !strings.Contains(output, "网格平台") {
		t.Errorf("should keep surrounding text: got %q", output)
	}
}

func TestSanitizeOutput_DirectReplace(t *testing.T) {
	input := "从 case_list 中查询 report_time 和 update_time"
	output := SanitizeOutput(input)
	if strings.Contains(output, "case_list") {
		t.Errorf("should replace case_list: got %q", output)
	}
	if !strings.Contains(output, "案件记录") {
		t.Errorf("should have 案件记录: got %q", output)
	}
	if !strings.Contains(output, "上报时间") {
		t.Errorf("should replace report_time: got %q", output)
	}
	if !strings.Contains(output, "更新时间") {
		t.Errorf("should replace update_time: got %q", output)
	}
}

func TestSanitizeOutput_LongPatternFirst(t *testing.T) {
	input := "case_type1 和 case_type"
	output := SanitizeOutput(input)
	if strings.Contains(output, "case_type1") {
		t.Errorf("longer pattern should be replaced first: got %q", output)
	}
}

func TestSanitizeOutput_EnglishBracket(t *testing.T) {
	input := "数据来源(grid_case_record)"
	output := SanitizeOutput(input)
	if strings.Contains(output, "grid_case_record") {
		t.Errorf("should remove english bracket form: got %q", output)
	}
}

func TestSanitizeOutput_NoFalsePositive(t *testing.T) {
	input := "这是一个普通的中文句子，不需要修改。"
	output := SanitizeOutput(input)
	if output != input {
		t.Errorf("should not change normal text: got %q", output)
	}
}
