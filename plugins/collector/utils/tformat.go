package utils

import (
	"time"
)

// FormatTimestamp 将Unix时间戳格式化为YYYYMMDDHHMMSS格式的字符串
func FormatTimestamp(timestamp int64) string {
	// 使用time.Unix将时间戳转换为Time类型
	// 第一个参数是秒级时间戳，第二个参数是纳秒部分（这里设为0）
	t := time.Unix(timestamp, 0)
	// 使用Format方法按照指定格式输出
	// Go的时间格式化使用固定的参考时间：2006-01-02 15:04:05
	return t.Format("20060102150405")
}
