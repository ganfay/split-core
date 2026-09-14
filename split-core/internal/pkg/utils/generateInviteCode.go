package utils

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

const words = "qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM1234567890"

func GenerateInviteCode(lenCode int) string {
	slice := strings.Split(words, "")
	var sliceIC []string
	for i := 0; i < lenCode; i++ {
		randW := rand.IntN(62)
		tempIC := slice[randW]
		sliceIC = append(sliceIC, tempIC)
	}
	return strings.Join(sliceIC, "")
}

func GenerateInviteCodeURL(inviteCode string, botName string) string {
	return fmt.Sprintf("t.me/%s?start=invite_%s", botName, inviteCode)
}

func GenerateLoginCodeURL(inviteCode string, botName string) string {
	return fmt.Sprintf("t.me/%s?start=auth_%s", botName, inviteCode)
}
