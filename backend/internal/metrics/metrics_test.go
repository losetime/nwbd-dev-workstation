package metrics

import (
	"testing"
	"time"
)

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestSuggestedHoursEvenlySpread(t *testing.T) {
	// 2026-11-02 是周一，16 个工作日到 11-23（周一）。
	start, due := date("2026-11-02"), date("2026-11-23")
	plan := SuggestedHours(60, start, due, nil)
	if len(plan) != 16 {
		t.Fatalf("工作日数 = %d, 期望 16", len(plan))
	}
	var sum float64
	for _, p := range plan {
		sum += p.Suggested
	}
	if Round1(sum) != 60 {
		t.Fatalf("建议合计 = %v, 期望 60", Round1(sum))
	}
	if plan[0].Suggested != 3.8 {
		t.Fatalf("首日建议 = %v, 期望 3.8", plan[0].Suggested)
	}
	if plan[15].Suggested != 3 {
		t.Fatalf("末日建议 = %v, 期望 3（补差额）", plan[15].Suggested)
	}
}

func TestSuggestedHoursSubtractsReported(t *testing.T) {
	plan := SuggestedHours(8, date("2026-11-02"), date("2026-11-03"), map[string]float64{"2026-11-02": 4})
	if plan[0].Default != 0 {
		t.Fatalf("已报 4h/建议 4h，可填应为 0，实际 %v", plan[0].Default)
	}
	if plan[1].Default != 4 {
		t.Fatalf("次日可填应为 4，实际 %v", plan[1].Default)
	}
}

func TestSuggestedHoursWeekendOnly(t *testing.T) {
	// 2026-11-07 周六、11-08 周日。
	if plan := SuggestedHours(4, date("2026-11-07"), date("2026-11-08"), nil); plan != nil {
		t.Fatalf("纯周末区间应返回 nil，实际 %d 条", len(plan))
	}
}

func TestSuggestedHoursNoPlan(t *testing.T) {
	if plan := SuggestedHours(0, date("2026-11-02"), date("2026-11-03"), nil); plan != nil {
		t.Fatal("预估工时为 0 应返回 nil")
	}
	if plan := SuggestedHours(8, date("2026-11-05"), date("2026-11-03"), nil); plan != nil {
		t.Fatal("开始晚于结束应返回 nil")
	}
}

func TestProgress(t *testing.T) {
	if p := Progress(0, 0); p != nil {
		t.Fatalf("无任务应返回 nil，实际 %v", *p)
	}
	if p := Progress(3, 1); p == nil || *p != 33 {
		t.Fatalf("1/3 应为 33，实际 %v", p)
	}
	if p := Progress(3, 3); p == nil || *p != 100 {
		t.Fatal("3/3 应为 100")
	}
}

func TestTaskOverdue(t *testing.T) {
	today := date("2026-11-17")
	plan := date("2026-11-16")
	if !IsTaskOverdue("进行中", &plan, today) {
		t.Fatal("进行中且已过期应判逾期")
	}
	if IsTaskOverdue("已完成", &plan, today) {
		t.Fatal("已完成不算逾期")
	}
	if IsTaskOverdue("进行中", nil, today) {
		t.Fatal("无计划完成日不算逾期")
	}
	// 今天到期不算逾期
	same := date("2026-11-17")
	if IsTaskOverdue("进行中", &same, today) {
		t.Fatal("当天到期不算逾期")
	}
}

func TestWeekBounds(t *testing.T) {
	// 2026-11-17 是周二
	monday, sunday := WeekBounds(date("2026-11-17"))
	if monday.Format("2006-01-02") != "2026-11-16" || sunday.Format("2006-01-02") != "2026-11-22" {
		t.Fatalf("周范围错误：%s ~ %s", monday.Format("2006-01-02"), sunday.Format("2006-01-02"))
	}
}

func TestDelayDays(t *testing.T) {
	plan, actual := date("2026-11-10"), date("2026-11-13")
	if got := DelayDays(&plan, &actual); got != 3 {
		t.Fatalf("延期天数 = %d, 期望 3", got)
	}
	early := date("2026-11-09")
	if got := DelayDays(&plan, &early); got != 0 {
		t.Fatalf("提前完成应为 0，实际 %d", got)
	}
}
