package crontab

import "time"

const StartExpressionValue = "@start"

var StartExpr = &StartExpression{}

type StartExpression struct{}

func (se StartExpression) Next(fromTime time.Time) time.Time {
	return time.Date(9999, 12, 31, 23, 59, 59, 0, time.Local)
}
