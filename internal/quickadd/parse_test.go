package quickadd

import (
	"strings"
	"testing"
)

func TestTitlePreservesDateText(t *testing.T) {
	for _, input := range []string{"明天下午3点 联系客户", "明天25:00 只是标题", "明天下午3点", "买菜 😀"} {
		got, err := Title(" " + input + " ")
		if err != nil || got != input {
			t.Fatalf("title changed: %q %v", got, err)
		}
	}
}
func TestTitleValidation(t *testing.T) {
	for _, input := range []string{"", "  ", "标题\n第二行", "标题\x00", strings.Repeat("字", 501)} {
		if _, err := Title(input); err == nil {
			t.Fatalf("invalid title accepted: %q", input)
		}
	}
	if _, err := Title(strings.Repeat("字", 500)); err != nil {
		t.Fatal(err)
	}
}
