// Package domain 定义业务枚举与默认值。枚举只在这里出现，前后端共用一套口径。
package domain

import "strings"

// ProjectTypes 项目类型（按原型：4 类，无外协合作）。
var ProjectTypes = []string{"合同实施项目", "售前项目", "自研项目", "售后项目"}

// StageTemplates 项目类型 → 阶段序列。顺序与名称是业务固定枚举，不可增删改。
var StageTemplates = map[string][]string{
	"合同实施项目": {"项目策划", "需求分析", "系统设计", "开发实现", "测试定版", "交付试用", "合同验收", "整改结项"},
	"自研项目":   {"项目策划", "需求分析", "系统设计", "开发实现", "测试定版", "交付试用"},
	"售前项目":   {"需求对接", "评估验证", "持续跟进"},
	"售后项目":   {},
}

// ProjectStatuses 项目状态六值。
var ProjectStatuses = []string{"未开始", "进行中", "暂停中", "已结束", "已取消", "售后中"}

// MilestoneStatuses 里程碑状态四值。
var MilestoneStatuses = []string{"未开始", "进行中", "已完成", "已延期"}

// TaskStatuses 任务状态五值。
var TaskStatuses = []string{"未开始", "进行中", "已完成", "已延期", "已取消"}

// RiskLevels 风险等级。
var RiskLevels = []string{"高", "中", "低"}

// RiskStatuses 风险状态（原型口径：监控中 / 已解决 / 已发生）。
var RiskStatuses = []string{"监控中", "已解决", "已发生"}

// RiskTypes 风险类型。
var RiskTypes = []string{"资源", "进度", "经费", "客户", "技术", "其他"}

// RiskFlags 项目风险标识（人工标注，独立于风险表）。
var RiskFlags = []string{"高风险", "低风险", "正常"}

// IssueSeverities 困难上报级别（与风险等级一致）。
var IssueSeverities = []string{"高", "中", "低"}

// IssueStatuses 困难上报状态。
var IssueStatuses = []string{"待处理", "已转风险", "已归档"}

// JobTypes 岗位类型。由钉钉职位映射而来。
var JobTypes = []string{"部门经理", "项目经理", "开发", "测试", "UI", "其他"}

// Levels 级别。
var Levels = []string{"初级", "中级", "高级", "专家"}

// ProjectRoles 项目成员的项目角色（原型口径，硬件开发保留但不再写入新数据）。
var ProjectRoles = []string{"项目经理", "软件开发", "硬件开发", "测试", "UI设计"}

// ChangeTypes 变更类型（本期只保留枚举，变更模块后续接钉钉审批）。
var ChangeTypes = []string{"里程碑变更", "任务计划变更", "预算变更", "范围变更", "放弃跟进"}

// PermKeys 界面权限键，与前端导航一一对应。
var PermKeys = []string{"myproj", "pending", "mytask", "resview", "myrisk", "mychange", "help", "sysadmin"}

// PermLabels 界面权限显示名。
var PermLabels = map[string]string{
	"myproj":   "我的项目",
	"pending":  "我的待办",
	"mytask":   "我的任务",
	"resview":  "资源视图",
	"myrisk":   "风险管理",
	"mychange": "变更管理",
	"help":     "困难上报",
	"sysadmin": "系统管理",
}

// DefaultPerms 岗位默认可见界面。个人的界面权限非空时以个人配置为准。
var DefaultPerms = map[string][]string{
	"部门经理": {"myproj", "pending", "mytask", "resview", "myrisk", "mychange", "help", "sysadmin"},
	"项目经理": {"myproj", "pending", "mytask", "resview", "myrisk", "mychange", "help"},
	"开发":   {"myproj", "pending", "mytask", "resview", "myrisk", "mychange", "help"},
	"测试":   {"myproj", "pending", "mytask", "resview", "myrisk", "mychange", "help"},
	"UI":   {"myproj", "pending", "mytask", "resview", "myrisk", "mychange", "help"},
	"其他":   {},
}

// jobTitleRules 钉钉职位 → 岗位类型。按顺序匹配，先匹配到的生效。
// 注意「项目经理」必须排在「经理」之前，否则「项目经理」会被归成部门经理。
// 「部门经理」只认研发/项目条线的头衔（本系统里的部门经理 = 研发部经理），
// 其他部门的「XX部经理」不参与本系统管理。
var jobTitleRules = []struct {
	keywords []string
	job      string
}{
	{[]string{"项目经理"}, "项目经理"},
	{[]string{"研发部经理", "项目部经理", "研发经理", "技术经理", "室主任"}, "部门经理"},
	{[]string{"测试", "QA", "质量"}, "测试"},
	{[]string{"UI", "ui", "设计", "交互", "美工"}, "UI"},
	{[]string{"开发", "工程师", "程序", "算法", "前端", "后端", "研发"}, "开发"},
}

// JobTypeForTitle 把钉钉职位文本映射成岗位类型；职位为空或未命中时归「其他」。
func JobTypeForTitle(title string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		return "其他"
	}
	for _, rule := range jobTitleRules {
		for _, kw := range rule.keywords {
			if strings.Contains(t, kw) {
				return rule.job
			}
		}
	}
	return "其他"
}

// EffectivePerms 计算某人最终可访问的界面：个人配置优先，为空则按岗位默认。
func EffectivePerms(jobType string, custom []string) []string {
	if len(custom) > 0 {
		out := make([]string, 0, len(custom))
		for _, key := range custom {
			if _, ok := PermLabels[key]; ok {
				out = append(out, key)
			}
		}
		return out
	}
	if def, ok := DefaultPerms[jobType]; ok {
		return def
	}
	return nil
}

// HasPerm 判断界面权限集合里是否包含某个键。
func HasPerm(perms []string, key string) bool {
	for _, p := range perms {
		if p == key {
			return true
		}
	}
	return false
}

// IsValid 判断值是否在枚举里。
func IsValid(options []string, value string) bool {
	for _, item := range options {
		if item == value {
			return true
		}
	}
	return false
}
