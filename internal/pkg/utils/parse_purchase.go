package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func ParsePurchase(text string) (cost float64, description string, err error) {
	slice := strings.Split(text, " ")
	if len(slice) < 2 {
		return 0, "", fmt.Errorf("invalid purchase format, need at least a price")
	}
	costStr := strings.Replace(slice[0], ",", ".", 1)
	cost, err = strconv.ParseFloat(costStr, 64)
	if err != nil || cost <= 0 {
		return 0, "", fmt.Errorf("invalid price, please enter a positive number")
	}
	description = "No description"
	if len(slice) > 1 {
		description = strings.Join(slice[1:], " ")
	}
	return cost, description, nil
}
