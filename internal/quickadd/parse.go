package quickadd

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Title validates plain task text without interpreting dates or times.
func Title(input string) (string, error) {
	title := strings.TrimSpace(input)
	if title == "" {
		return title, errors.New("请输入任务标题。")
	}
	if strings.ContainsAny(title, "\r\n\x00") {
		return title, errors.New("任务标题只能有一行。")
	}
	if utf8.RuneCountInString(title) > 500 {
		return title, errors.New("任务标题最多500字，请缩短后重试。")
	}
	return title, nil
}
