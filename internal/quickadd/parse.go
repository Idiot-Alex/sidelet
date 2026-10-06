// Package quickadd implements the portable Quick Add rules and lifecycle.
package quickadd

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Parsed struct {
	Title string `json:"title"`
	DueAt int64  `json:"dueAt"`
}

var prefix = regexp.MustCompile(`^(今天|明天|后天)\s*(凌晨|早上|上午|中午|下午|晚上)?\s*([0-9]{1,2}|[一二三四五六七八九十两]{1,3})(?:[:：]([0-9]{2})|点(?:(半)|([0-9]{1,2}|[一二三四五六七八九十两]{1,3})分)?)\s*(.*)$`)

func number(s string) (int, error) {
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	digits := map[rune]int{'一': 1, '二': 2, '两': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	r := []rune(s)
	if len(r) == 1 {
		if r[0] == '十' {
			return 10, nil
		}
		if n, ok := digits[r[0]]; ok {
			return n, nil
		}
	}
	if len(r) == 2 && r[0] == '十' {
		if n, ok := digits[r[1]]; ok {
			return 10 + n, nil
		}
	}
	if (len(r) == 2 || len(r) == 3) && r[1] == '十' {
		if n, ok := digits[r[0]]; ok {
			if len(r) == 2 {
				return n * 10, nil
			}
			if v, ok := digits[r[2]]; ok {
				return n*10 + v, nil
			}
		}
	}
	return 0, errors.New("时间中的数字无法识别，请使用数字，例如明天15:30。")
}

func Parse(input string, enabled bool, now time.Time) (Parsed, error) {
	result := Parsed{Title: strings.TrimSpace(input)}
	if strings.ContainsAny(input, "\x00\r\n") {
		return result, errors.New("请输入一行任务标题。")
	}
	if enabled {
		if m := prefix.FindStringSubmatch(result.Title); m != nil {
			hour, err := number(m[3])
			if err != nil {
				return result, err
			}
			minute := 0
			if m[4] != "" {
				minute, _ = strconv.Atoi(m[4])
			}
			if m[5] != "" {
				minute = 30
			}
			if m[6] != "" {
				minute, err = number(m[6])
				if err != nil {
					return result, err
				}
			}
			if hour > 23 || minute > 59 {
				return result, errors.New("时间无效：小时应为0–23，分钟应为0–59。")
			}
			days := map[string]int{"今天": 0, "明天": 1, "后天": 2}[m[1]]
			if m[2] != "" {
				if hour < 1 || hour > 12 {
					return result, errors.New("带上午／下午等时段时，请使用1–12点；24小时制请直接写15:30。")
				}
				switch m[2] {
				case "凌晨":
					if hour == 12 {
						hour = 0
					}
				case "下午":
					if hour != 12 {
						hour += 12
					}
				case "中午":
					if hour >= 3 && hour <= 10 {
						return result, errors.New("中午时间有歧义，请直接写24小时制，例如12:30。")
					}
					if hour < 3 {
						hour += 12
					}
				case "晚上":
					if hour == 12 {
						hour = 0
						days++
					} else if hour < 6 {
						return result, errors.New("晚上时间有歧义，请直接写24小时制，例如21:30。")
					} else {
						hour += 12
					}
				}
			}
			date := now.AddDate(0, 0, days)
			due := time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, now.Location())
			if due.Year() != date.Year() || due.Month() != date.Month() || due.Day() != date.Day() || due.Hour() != hour || due.Minute() != minute {
				return result, errors.New("这个本地时间不存在，请检查夏令时或选择其他时间。")
			}
			result = Parsed{Title: strings.TrimSpace(m[7]), DueAt: due.UnixMilli()}
		}
	}
	if result.Title == "" {
		return result, errors.New("请在时间后写下要做的事。")
	}
	if utf8.RuneCountInString(result.Title) > 500 {
		return result, errors.New("任务标题最多500字，请缩短后重试。")
	}
	return result, nil
}
