// Package passwordpolicy validates newly assigned console passwords. Existing
// passwords are still verified as entered so policy changes cannot lock out users.
package passwordpolicy

import "errors"

const Hint = "密码须为12–72位，包含英文字母和数字，可使用英文符号，不支持中文或空格"

func Validate(password string) error {
	if len(password) < 12 || len(password) > 72 {
		return errors.New(Hint)
	}
	hasLetter, hasDigit := false, false
	for i := 0; i < len(password); i++ {
		char := password[i]
		if char < '!' || char > '~' {
			return errors.New(Hint)
		}
		hasLetter = hasLetter || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
		hasDigit = hasDigit || char >= '0' && char <= '9'
	}
	if !hasLetter || !hasDigit {
		return errors.New(Hint)
	}
	return nil
}
