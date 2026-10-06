package quickadd

import (
	"strings"
	"testing"
	"time"
)

func TestTimePrefixesAndOriginalTitles(t *testing.T) {
	zone := time.FixedZone("UTC+8", 8*3600)
	now := time.Date(2026, 12, 31, 23, 50, 0, 0, zone)
	for _, tt := range []struct{ input, title, at string }{
		{"明天下午3点 联系客户", "联系客户", "2027-01-01T15:00:00+08:00"},
		{"今天15:30 完成报告", "完成报告", "2026-12-31T15:30:00+08:00"},
		{"后天上午九点半 买菜", "买菜", "2027-01-02T09:30:00+08:00"},
		{"明天 09：05 读书", "读书", "2027-01-01T09:05:00+08:00"},
		{"明天两点十五分写笔记", "写笔记", "2027-01-01T02:15:00+08:00"},
		{"明天上午12点 午餐", "午餐", "2027-01-01T12:00:00+08:00"},
		{"明天中午一点 开会", "开会", "2027-01-01T13:00:00+08:00"},
		{"今天晚上十二点半 睡觉", "睡觉", "2027-01-01T00:30:00+08:00"},
		{"明天凌晨12点 部署", "部署", "2027-01-01T00:00:00+08:00"},
		{"后天23点五十九分 复盘", "复盘", "2027-01-02T23:59:00+08:00"},
		{"明天3点三角函数复习", "三角函数复习", "2027-01-01T03:00:00+08:00"},
		{"明天3点 30份报告", "30份报告", "2027-01-01T03:00:00+08:00"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Parse(tt.input, true, now)
			if err != nil {
				t.Fatal(err)
			}
			if got.Title != tt.title || time.UnixMilli(got.DueAt).In(zone).Format(time.RFC3339) != tt.at {
				t.Fatalf("got %+v at %s", got, time.UnixMilli(got.DueAt).In(zone))
			}
		})
	}
	for _, input := range []string{"买菜 😀", "明天开会", "周三15:00 去图书馆", "明天下午开会", "2027-01-02 读书"} {
		got, err := Parse(" "+input+" ", true, now)
		if err != nil || got.Title != input || got.DueAt != 0 {
			t.Fatalf("ordinary title changed: %+v %v", got, err)
		}
	}
	got, err := Parse("明天下午3点 联系客户", false, now)
	if err != nil || got.Title != "明天下午3点 联系客户" || got.DueAt != 0 {
		t.Fatalf("recognition switch ignored: %+v %v", got, err)
	}
}

func TestInvalidTimesAndTitleLimits(t *testing.T) {
	for _, input := range []string{"", "明天下午3点", "明天25点 开会", "今天15:99 开会", "明天下午15点 开会", "明天12点六十分 买菜", "明天中午九点 买菜", "明天晚上两点 买菜", "明天十十点 开会", "标题\n第二行", "标题\x00", strings.Repeat("字", 501)} {
		if _, err := Parse(input, true, time.Now()); err == nil {
			t.Errorf("invalid input accepted: %q", input)
		}
	}
	if _, err := Parse(strings.Repeat("字", 500), false, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestCalendarDaysAndDST(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, ny)
	got, err := Parse("明天9:00 开会", true, now)
	if err != nil {
		t.Fatal(err)
	}
	if time.UnixMilli(got.DueAt).In(ny).Format(time.RFC3339) != "2026-03-08T09:00:00-04:00" {
		t.Fatal("relative days treated as 24-hour durations")
	}
	if _, err := Parse("明天2:30 开会", true, now); err == nil {
		t.Fatal("nonexistent DST time accepted")
	}
	got, err = Parse("后天9点 买菜", true, time.Date(2028, 2, 28, 20, 0, 0, 0, time.UTC))
	if err != nil || time.UnixMilli(got.DueAt).UTC().Format("2006-01-02") != "2028-03-01" {
		t.Fatalf("leap day: %+v %v", got, err)
	}
}
