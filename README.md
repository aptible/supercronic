cronexpression for Go
=====================
Cron expression parser in Go language (golang).

Given a cron expression and a time stamp, you can get the next time stamp which satisfy the cron expression.

In another project, I decided to use Cron syntax to encode scheduling information. Thus this standalone library to parse and execute cron expressions.

Implementation
--------------
The reference documentation for this implementation is found at
https://en.wikipedia.org/wiki/Cron#CRON_expression, which I copy/pasted here (laziness) with modifications where this implementation differs:

    Field name     Mandatory?   Allowed values    Allowed special characters
    ----------     ----------   --------------    --------------------------
    Seconds        No           0-59              * / , -
    Minutes        Yes          0-59              * / , -
    Hours          Yes          0-23              * / , -
    Day of month   Yes          1-31              * / , - ? L W
    Month          Yes          1-12 or JAN-DEC   * / , -
    Day of week    Yes          0-6 or SUN-SAT    * / , - ? L #
    Year           No           1970–2099         * / , -

Asterisk ( * )
--------------
The asterisk indicates that the cron expression matches for all values of the field. E.g., using an asterisk in the 4th field (month) indicates every month. 

Slash ( / )
-----------
Slashes describe increments of ranges. For example `3-59/15` in the minute field indicate the third minute of the hour and every 15 minutes thereafter. The form `*/...` is equivalent to the form "first-last/...", that is, an increment over the largest possible range of the field.

Comma ( , )
-----------
Commas are used to separate items of a list. For example, using `MON,WED,FRI` in the 5th field (day of week) means Mondays, Wednesdays and Fridays.

Hyphen ( - )
------------
Hyphens define ranges. For example, 2000-2010 indicates every year between 2000 and 2010 AD, inclusive.

L
-
`L` stands for "last". When used in the day-of-week field, it allows you to specify constructs such as "the last Friday" (`5L`) of a given month. In the day-of-month field, it specifies the last day of the month.

W
-
The `W` character is allowed for the day-of-month field. This character is used to specify the weekday (Monday-Friday) nearest the given day. As an example, if you were to specify `15W` as the value for the day-of-month field, the meaning is: "the nearest weekday to the 15th of the month." So, if the 15th is a Saturday, the trigger fires on Friday the 14th. If the 15th is a Sunday, the trigger fires on Monday the 16th. If the 15th is a Tuesday, then it fires on Tuesday the 15th. However if you specify `1W` as the value for day-of-month, and the 1st is a Saturday, the trigger fires on Monday the 3rd, as it does not 'jump' over the boundary of a month's days. The `W` character can be specified only when the day-of-month is a single day, not a range or list of days.

Hash ( # )
----------
`#` is allowed for the day-of-week field, and must be followed by a number between one and five. It allows you to specify constructs such as "the second Friday" of a given month.

Question mark ( ? )
-------------------
Note: Question mark is a non-standard character and exists only in some cron implementations. It is used instead of `*` for leaving either day-of-month or day-of-week blank.

With the following differences:

* Supports optional second field (before minute field)
* If five fields are present, a wildcard year field is appended
* If six field are present, `0` is prepended as second field, that is, `* * * * * *` internally become `0 * * * * * *`.
* Domain for day-of-week field is [0-7] instead of [0-6], 7 being Sunday (like 0).
* `@reboot` is not supported
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

    nextTime := cronexpression.NextTimeFromCronString("0 0 29 2 *", time.Now())

Assuming `time.Now()` is "2013-08-29 09:28:00", then `nextTime` will be "2016-02-29 00:00:00".

If you need to reuse many times a cron expression in your code, it is more efficient
to create a `CronExpression` object once and keep a copy of it for reuse:

    cronexpr := cronexpression.NewCronExpression("0 0 29 2 *")
    nextTime := cronexpr.NextTime(time.Now())
    

