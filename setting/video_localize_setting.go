package setting

import (
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// 视频本地化相关系统设置项（存于 options 表，由 common.OptionMap 缓存）。
const (
	OptVideoLocalizeEnabled        = "VideoLocalizeEnabled"        // 总开关 bool
	OptVideoLocalizeConcurrency    = "VideoLocalizeConcurrency"    // 最大并发下载数 int
	OptVideoLocalizeDir            = "VideoLocalizeDir"            // 本地存储目录 string
	OptVideoLocalizeTimeoutSeconds = "VideoLocalizeTimeoutSeconds" // 单文件下载超时(秒) int
	OptVideoLocalizeMaxRetry       = "VideoLocalizeMaxRetry"       // 最大重试次数 int
	OptVideoLocalizeRetainDays     = "VideoLocalizeRetainDays"     // 本地文件保留天数(0=不清理) int
	OptVideoLocalizePublicBaseURL  = "VideoLocalizePublicBaseURL"  // 对外本地地址的公网 Base URL(空=用系统地址) string
)

func optString(key, def string) string {
	if v, ok := common.OptionMap[key]; ok && v != "" {
		return v
	}
	return def
}

func optBool(key string, def bool) bool {
	v, ok := common.OptionMap[key]
	if !ok || v == "" {
		return def
	}
	return v == "true" || v == "1"
}

func optInt(key string, def int) int {
	v, ok := common.OptionMap[key]
	if !ok || v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func VideoLocalizeEnabledBool() bool {
	return optBool(OptVideoLocalizeEnabled, true)
}

func VideoLocalizeConcurrencyInt() int {
	if n := optInt(OptVideoLocalizeConcurrency, 3); n > 0 {
		return n
	}
	return 3
}

func VideoLocalizeDirStr() string {
	return optString(OptVideoLocalizeDir, "data/video_files")
}

func VideoLocalizeTimeoutDur() time.Duration {
	return time.Duration(optInt(OptVideoLocalizeTimeoutSeconds, 120)) * time.Second
}

func VideoLocalizeMaxRetryInt() int {
	return optInt(OptVideoLocalizeMaxRetry, 3)
}

func VideoLocalizeRetainDaysInt() int {
	return optInt(OptVideoLocalizeRetainDays, 0)
}

// VideoLocalizePublicBaseURLStr 返回对外本地地址的公网 Base URL；为空则回退到系统 ServerAddress。
func VideoLocalizePublicBaseURLStr() string {
	return optString(OptVideoLocalizePublicBaseURL, "")
}
