package lib

import "unicode"

func IsFirstLetterUppercase(s string) bool {
	// 处理空字符串
	if len(s) == 0 {
		return false
	}

	// 遍历字符串，获取第一个rune（UTF-8字符）
	for _, r := range s {
		// 先判断是否为字母，再判断是否为大写
		return unicode.IsLetter(r) && unicode.IsUpper(r)
	}

	// 理论上不会执行到这里（空字符串已提前处理）
	return false
}
