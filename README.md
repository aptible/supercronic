cronexpression for Go
=====================
Cron expression parser in Go language (golang).

Given a cron expression and a time stamp, you can get the next time stamp which satisfies the cron expression.

In another project, I decided to use cron expression syntax to encode scheduling information. Thus this standalone library to parse and apply time stamps to cron expressions.

The time-matching algorithm in this implementation is efficient, it avoids as much as possible to guess the next matching time stamp, a common technique seen in a number of implementations out there.

Implementation
--------------
The reference documentation for this implementation is found at
<https://en.wikipedia.org/wiki/Cron#CRON_expression>, which I copy/pasted here (laziness!) with modifications where this implementation differs:

    Field name     Mandatory?   Allowed values    Allowed special characters
    ----------     ----------   --------------    --------------------------
    Seconds        No           0-59              * / , -
    Minutes        Yes          0-59              * / , -
    Hours          Yes          0-23              * / , -
    Day of month   Yes          1-31              * / , - L W
    Month          Yes          1-12 or JAN-DEC   * / , -
    Day of week    Yes          0-6 or SUN-SAT    * / , - L #
    Year           No           1970–2099         * / , -

#### Asterisk ( * )
The asterisk indicates that the cron expression matches for all values of the field. E.g., using an asterisk in the 4th field (month) indicates every month. 

#### Slash ( / )
Slashes describe increments of ranges. For example `3-59/15` in the minute field indicate the third minute of the hour and every 15 minutes thereafter. The form `*/...` is equivalent to the form "first-last/...", that is, an increment over the largest possible range of the field.

#### Comma ( , )
Commas are used to separate items of a list. For example, using `MON,WED,FRI` in the 5th field (day of week) means Mondays, Wednesdays and Fridays.

#### Hyphen ( - )
Hyphens define ranges. For example, 2000-2010 indicates every year between 2000 and 2010 AD, inclusive.

#### L
`L` stands for "last". When used in the day-of-week field, it allows you to specify constructs such as "the last Friday" (`5L`) of a given month. In the day-of-month field, it specifies the last day of the month.

#### W
The `W` character is allowed for the day-of-month field. This character is used to specify the weekday (Monday-Friday) nearest the given day. As an example, if you were to specify `15W` as the value for the day-of-month field, the meaning is: "the nearest weekday to the 15th of the month." So, if the 15th is a Saturday, the trigger fires on Friday the 14th. If the 15th is a Sunday, the trigger fires on Monday the 16th. If the 15th is a Tuesday, then it fires on Tuesday the 15th. However if you specify `1W` as the value for day-of-month, and the 1st is a Saturday, the trigger fires on Monday the 3rd, as it does not 'jump' over the boundary of a month's days. The `W` character can be specified only when the day-of-month is a single day, not a range or list of days.

#### Hash ( # )
`#` is allowed for the day-of-week field, and must be followed by a number between one and five. It allows you to specify constructs such as "the second Friday" of a given month.

Predefined cron expressions
---------------------------
(Copied from <https://en.wikipedia.org/wiki/Cron#Predefined_scheduling_definitions>, with text modified according to this implementation) 

    Entry       Description                                                             Equivalent to
    @annually   Run once a year at midnight in the morning of January 1                 0 0 0 1 1 * *
    @yearly     Run once a year at midnight in the morning of January 1                 0 0 0 1 1 * *
    @monthly    Run once a month at midnight in the morning of the first of the month   0 0 0 1 * * *
    @weekly     Run once a week at midnight in the morning of Sunday                    0 0 0 * * 0 *
    @daily      Run once a day at midnight                                              0 0 0 * * * *
    @hourly     Run once an hour at the beginning of the hour                           0 0 * * * * *
    @reboot     Not supported

Other details
-------------
* If only six fields are present, a `0` second field is prepended, that is, `* * * * * 2013` internally become `0 * * * * * 2013`.
* If only five fields are present, a `0` second field is prepended and a wildcard year field is appended, that is, `* * * * Mon` internally become `0 * * * * Mon *`.
* Domain for day-of-week field is [0-7] instead of [0-6], 7 being Sunday (like 0). This to comply with http://linux.die.net/man/5/crontab#.
* As of now, the behavior of the code is undetermined if a malformed cron expression is supplied

Install
-------
    go get github.com/gorhill/cronexpression

Usage
-----
Import the library:

    import "github.com/gorhill/cronexpression"
    import "time"

Simplest way:

    nextTime := cronexpression.NextTime("0 0 29 2 *", time.Now())

Assuming `time.Now()` is "2013-08-29 09:28:00", then `nextTime` will be "2016-02-29 00:00:00".

If you need to reuse many times the same cron expression in your code, it is more efficient
to create a `CronExpression` object once and keep a copy of it for reuse:

    cronexpr := cronexpression.NewCronExpression("0 0 29 2 *")
    nextTime := cronexpr.NextTime(time.Now())
    ...

Use `cronexpression.NoMatch()` to find out whether a valid time was returned. For example,

    cronexpression.NoMatch(cronexpression.NextTime("* * * * * 1980", time.Now()))

will return `true`, whereas

    cronexpression.NoMatch(cronexpression.NextTime("* * * * * 2050", time.Now()))

will return `false` (as of 2013-08-29...)

API
---

#### func NextTime

    func NextTime(cronLine string, fromTime time.Time) time.Time

Given a time stamp `fromTime`, return the closest following time stamp which matches the cron expression string `cronLine`. The `time.Location` of the returned time stamp is the same as `fromTime`.

#### func NextTimeN

    func NextTimeN(cronLine string, fromTime time.Time, n int) []time.Time

Given a time stamp `fromTime`, return a slice of `n` closest following time stamps which match the cron expression string `cronLine`. The time stamps in the returned slice are in chronological ascending order. The `time.Location` of the returned time stamps is the same as `fromTime`.

Example:

    cronexpression.NextTimeN("0 0 0 29 2 ? *", time.Now(), 5)

will result in the following time stamps being returned (as of 2013-08-30):

    2016-02-29 00:00:00
    2020-02-29 00:00:00
    2024-02-29 00:00:00
    2028-02-29 00:00:00
    2032-02-29 00:00:00

#### func NewCronExpression

    func NewCronExpression(cronLine string) *CronExpression

Return a new `CronExpression` pointer which will interpret the cron expression string `cronLine`.

#### func (*CronExpression) NextTime

    func (cronexpr *CronExpression) NextTime(fromTime time.Time) time.Time

Given a time stamp `fromTime`, return the closest following time stamp which matches the cron expression `cronexpr`. The `time.Location` of the returned time stamp is the same as `fromTime`.

#### func (*CronExpression) NextTimeN

    func (cronexpr *CronExpression) NextTimeN(fromTime time.Time, n int) []time.Time

Given a time stamp `fromTime`, return a slice of `n` closest following time stamps which match the cron expression `cronexpr`. The time stamps in the returned slice are in chronological ascending order. The `time.Location` of the returned time stamps is the same as `fromTime`.

#### func NoMatch

    func NoMatch(t time.Time) bool

Returns `true` if time stamp `t` is not a valid time stamp from `CronExpression` point of view. An invalid time stamp is returned by this library whenever no matching time stamp is found given a specific cron expression.

