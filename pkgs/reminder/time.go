package reminder

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var relativeDurationPattern = regexp.MustCompile(`(?i)^(?:(\d+)d)?(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$`)

// ParseTimeSpec 解析相对时间或 cron 表达式。
func ParseTimeSpec(spec string) (kind string, runAt *time.Time, cronExpr string, err error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", nil, "", fmt.Errorf("time 不能为空")
	}

	if looksLikeCron(spec) {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, parseErr := parser.Parse(spec)
		if parseErr != nil {
			return "", nil, "", fmt.Errorf("cron 表达式无效: %w", parseErr)
		}
		next := schedule.Next(time.Now())
		return ScheduleCron, &next, spec, nil
	}

	duration, parseErr := ParseRelativeDuration(spec)
	if parseErr != nil {
		return "", nil, "", parseErr
	}
	next := time.Now().Add(duration)
	return ScheduleRelative, &next, "", nil
}

func looksLikeCron(spec string) bool {
	fields := strings.Fields(spec)
	return len(fields) == 5
}

// ParseRelativeDuration 解析 5m / 1h / 1h30m / 2d 等相对时间。
func ParseRelativeDuration(spec string) (time.Duration, error) {
	spec = strings.ToLower(strings.TrimSpace(spec))
	if spec == "" {
		return 0, fmt.Errorf("相对时间不能为空")
	}
	match := relativeDurationPattern.FindStringSubmatch(spec)
	if match == nil {
		return 0, fmt.Errorf("无法解析相对时间 %q，示例：10s、5m、1h、1h30m、2d", spec)
	}
	days := atoiDefault(match[1])
	hours := atoiDefault(match[2])
	minutes := atoiDefault(match[3])
	seconds := atoiDefault(match[4])
	if days == 0 && hours == 0 && minutes == 0 && seconds == 0 {
		return 0, fmt.Errorf("相对时间至少包含一个有效单位")
	}
	total := time.Duration(days)*24*time.Hour +
		time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second
	return total, nil
}

func atoiDefault(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	var n int
	_, _ = fmt.Sscanf(value, "%d", &n)
	return n
}

const (
	ScheduleRelative = "relative"
	ScheduleCron     = "cron"
)

func DefaultReminderName(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return "提醒"
	}
	runes := []rune(content)
	if len(runes) <= 20 {
		return content
	}
	return string(runes[:20])
}

func NextCronRun(expr string, from time.Time) (time.Time, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(strings.TrimSpace(expr))
	if err != nil {
		return time.Time{}, err
	}
	return schedule.Next(from), nil
}
