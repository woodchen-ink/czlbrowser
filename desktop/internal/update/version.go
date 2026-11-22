package update

import (
	"strconv"
	"strings"
)

// 版本号由 CI 生成，格式为 yyyy.MM.dd.N（N 为 GitHub Actions run_number），允许带前缀 v。

// IsReleaseVersion 判断是否为正式构建版本号；本地构建的 "dev" 返回 false，不参与更新检查。
func IsReleaseVersion(v string) bool {
	_, ok := parseVersion(v)
	return ok
}

// CompareVersions 逐段按数字比较，返回 -1 / 0 / 1；无法解析的版本视为最小。
func CompareVersions(a, b string) int {
	pa, okA := parseVersion(a)
	pb, okB := parseVersion(b)
	switch {
	case !okA && !okB:
		return 0
	case !okA:
		return -1
	case !okB:
		return 1
	}
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func parseVersion(v string) ([]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return nil, false
	}
	parts := strings.Split(v, ".")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, false
		}
		nums = append(nums, n)
	}
	return nums, true
}
