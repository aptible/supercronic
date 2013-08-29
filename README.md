gocronexpression
================

Go language (golang) cron expression parser. Given a cron expression and a time stamp, you can get the next time stamp which satisfy the cron expression.

The reference documentation for this implementation is found at
https://en.wikipedia.org/wiki/Cron#CRON_expression

Install
-------

    go get github.com/gorhill/cronexpression

Usage
-----

Import the library:

    import "github.com/gorhill/cronexpression"
    import "time"

Simplest way:

    nextTime := cronexpression.NextTimeFromCronString("0 0 29 2 *", time.Now())

Assuming `time.Now()` is "2013-08-29 09:28:00", then `nextTime` will be "2016-02-29 00:00:00".

If you need to reuse many times a cron expression in your code, it is more efficient
to create a `CronExpression` object once and keep a copy of it for reuse:

    cronexpr := cronexpression.NewCronExpression("0 0 29 2 *")
    nextTime := cronexpr.NextTime(time.Now())
    

