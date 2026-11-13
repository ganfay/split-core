package utils

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func ParsePurchase(text string) (cost float64, description string, err error) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return 0, "", fmt.Errorf("invalid purchase format, need at least a price")
	}
	costStr := strings.Replace(fields[0], ",", ".", 1)
	cost, err = strconv.ParseFloat(costStr, 64)
	if err != nil || math.IsNaN(cost) || math.IsInf(cost, 0) || cost <= 0 {
		return 0, "", fmt.Errorf("invalid price, please enter a positive number")
	}
	description = "No description"
	if len(fields) > 1 {
		description = strings.Join(fields[1:], " ")
	}
	return cost, description, nil
}
