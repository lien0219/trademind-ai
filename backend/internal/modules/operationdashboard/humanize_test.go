package operationdashboard

import (
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
)

func TestHumanizeTaskStatusSuccessWithWarnings(t *testing.T) {
	got := humanizeTaskStatus(imagetask.StatusSuccessWithWarnings)
	if got != "成功（有警告）" {
		t.Fatalf("expected 成功（有警告）, got %q", got)
	}
}

func TestHumanizeImageTaskTypeTranslate(t *testing.T) {
	got := humanizeImageTaskType(imagetask.TaskTypeTranslateImageText)
	if got != "图片文字翻译" {
		t.Fatalf("expected 图片文字翻译, got %q", got)
	}
}

func TestHumanizeImageTaskSubtitleTranslateWarnings(t *testing.T) {
	output := []byte(`{
	  "quality": {
	    "warnings": ["系统已自动精简部分翻译文案以适配排版。"],
	    "layout": {"warnings": ["partial_text_detected"]}
	  }
	}`)
	sub := humanizeImageTaskSubtitle(imagetask.TaskTypeTranslateImageText, imagetask.StatusSuccessWithWarnings, output, "")
	if sub != "系统已自动精简部分翻译文案以适配排版。" {
		t.Fatalf("unexpected subtitle: %q", sub)
	}
}

func TestHumanizeImageTaskSubtitleFailed(t *testing.T) {
	sub := humanizeImageTaskSubtitle(imagetask.TaskTypeTranslateImageText, imagetask.StatusFailed, nil, "[OCR_FAILED] 文字识别结果解析失败")
	if sub == "" {
		t.Fatal("expected failed subtitle")
	}
}
