package xhsutil

import "unicode/utf16"

// CalcTitleLength 计算小红书标题长度
// 规则：非ASCII字符(中文、全角符号等)算2字节，ASCII字符算1字节，最终结果向上取整除以2
func CalcTitleLength(s string) int {
	byteLen := 0
	for _, c := range utf16.Encode([]rune(s)) {
		if c > 127 {
			byteLen += 2
		} else {
			byteLen += 1
		}
	}
	return (byteLen + 1) / 2
}
