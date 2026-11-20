// Package metrics 是全部计算口径的唯一实现。
//
// 原则：同一指标只在这里算一次，各页面调用同一函数，避免「台账 60%、驾驶舱 50%」。
package metrics

import (
	"math"
	"time"
)

// Round1 保留一位小数（原型用 0.1 精度）。
func Round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// IsOpenTask 在办任务：未开始 / 进行中 / 已延期。
func IsOpenTask(status string) bool {
	return status == "未开始" || status == "进行中" || status == "已延期"
}

// IsActiveTask 参与统计的任务：不含已取消。
func IsActiveTask(status string) bool {
	return status != "已取消"
}

// Progress 任务完成率。total 为 0 时返回 nil（页面显示「—」）。
func Progress(total, done int) *int {
	if total <= 0 {
		return nil
	}
	p := int(math.Round(float64(done) / float64(total) * 100))
	return &p
}

// StageProgress 阶段完成率（里程碑）。
func StageProgress(total, done int) *int {
	if total <= 0 {
		return nil
	}
	p := int(math.Round(float64(done) / float64(total) * 100))
	return &p
}

// IsTaskOverdue 任务逾期：未开始 / 进行中 / 已延期，且计划完成日早于今天。
func IsTaskOverdue(status string, planDue *time.Time, today time.Time) bool {
	if planDue == nil {
		return false
	}
	return IsOpenTask(status) && dayStart(*planDue).Before(dayStart(today))
}

// IsMilestoneOverdue 里程碑超期：未完成且截止日早于今天。
func IsMilestoneOverdue(status string, due *time.Time, today time.Time) bool {
	if due == nil || status == "已完成" {
		return false
	}
	return dayStart(*due).Before(dayStart(today))
}

// DelayDays 延期完成天数：实际完成日晚于计划完成日时返回正数，否则 0。
func DelayDays(planDue, actualDone *time.Time) int {
	if planDue == nil || actualDone == nil {
		return 0
	}
	diff := int(dayStart(*actualDone).Sub(dayStart(*planDue)).Hours() / 24)
	if diff <= 0 {
		return 0
	}
	return diff
}

// DayPlan 是工时确认弹窗里的一行。
type DayPlan struct {
	Date      string  `json:"date"`
	Suggested float64 `json:"suggested"`
	Already   float64 `json:"already"`
	Default   float64 `json:"default"`
}

// SuggestedHours 按任务计划区间与预估工时生成每日建议。
//
// 与原型一致：只摊到工作日；每日 = 预估/工作日数（保留一位小数），
// 最后一天补齐差额；本次可填 = max(建议 − 当天已报, 0)。
func SuggestedHours(estimate float64, start, due time.Time, booked map[string]float64) []DayPlan {
	if estimate <= 0 || start.After(due) {
		return nil
	}
	days := Workdays(start, due)
	if len(days) == 0 {
		return nil
	}
	per := Round1(estimate / float64(len(days)))
	out := make([]DayPlan, 0, len(days))
	for i, d := range days {
		key := d.Format("2006-01-02")
		suggested := per
		if i == len(days)-1 {
			suggested = Round1(estimate - per*float64(len(days)-1))
		}
		already := booked[key]
		def := Round1(suggested - already)
		if def < 0 {
			def = 0
		}
		out = append(out, DayPlan{Date: key, Suggested: suggested, Already: already, Default: def})
	}
	return out
}

// Workdays 返回区间内的周一至周五（含首尾），最多 400 天，防止非法输入卡死。
func Workdays(start, due time.Time) []time.Time {
	var out []time.Time
	cur := dayStart(start)
	end := dayStart(due)
	for !cur.After(end) && len(out) < 400 {
		if wd := cur.Weekday(); wd != time.Saturday && wd != time.Sunday {
			out = append(out, cur)
		}
		cur = cur.AddDate(0, 0, 1)
	}
	return out
}

// WeekBounds 返回某天所在周的周一与周日（周一为一周开始）。
func WeekBounds(t time.Time) (time.Time, time.Time) {
	d := dayStart(t)
	offset := (int(d.Weekday()) + 6) % 7
	monday := d.AddDate(0, 0, -offset)
	return monday, monday.AddDate(0, 0, 6)
}

// InRange 判断日期是否落在 [start, end]（按天比较）。
func InRange(d, start, end time.Time) bool {
	dd := dayStart(d)
	return !dd.Before(dayStart(start)) && !dd.After(dayStart(end))
}

func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
