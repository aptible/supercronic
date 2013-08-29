gocronexpression
================

Go language (golang) cron expression parser. Given a cron expression and a time stamp, you can get the next time stamp which satisfy the cron expression.

Install
-------

    go get github.com/gorhill/cronexpression

Usage
-----

Import the library:

    import "github.com/gorhill/cronexpression"
    import "time"

Simplest way:

    ...
    nextTime := cronexpression.NextTimeFromCronString("* * 29 2 *", time.Now())

Assuming *time.Now()* is "2013-08-29 09:28:00", then *nextTime* will be "Monday, February 29, 2016 00:00:00".
