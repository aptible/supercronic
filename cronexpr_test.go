/*!
 * Copyright 2013 Raymond Hill
 *
 * Project: github.com/gorhill/cronexpr
 * File: cronexpr_test.go
 * Version: 1.0
 * License: GPL v3 see <https://www.gnu.org/licenses/gpl.html>
 *
 */

package cronexpr_test

/******************************************************************************/

import (
	"testing"
	"time"

	"github.com/gorhill/cronexpr"
)

/******************************************************************************/

type crontimes struct {
	from string
	next string
}

type crontest struct {
	expr   string
	layout string
	times  []crontimes
}

var crontests = []crontest{
	// Seconds
	{
		"* * * * * * *",
		"2006-01-02 15:04:05",
		[]crontimes{
			{"2013-01-01 00:00:00", "2013-01-01 00:00:01"},
			{"2013-01-01 00:00:59", "2013-01-01 00:01:00"},
			{"2013-01-01 00:59:59", "2013-01-01 01:00:00"},
			{"2013-01-01 23:59:59", "2013-01-02 00:00:00"},
			{"2013-02-28 23:59:59", "2013-03-01 00:00:00"},
			{"2016-02-28 23:59:59", "2016-02-29 00:00:00"},
			{"2012-12-31 23:59:59", "2013-01-01 00:00:00"},
		},
	},

	// Minutes
	{
		"* * * * *",
		"2006-01-02 15:04:05",
		[]crontimes{
			{"2013-01-01 00:00:00", "2013-01-01 00:01:00"},
			{"2013-01-01 00:00:59", "2013-01-01 00:01:00"},
			{"2013-01-01 00:59:00", "2013-01-01 01:00:00"},
			{"2013-01-01 23:59:00", "2013-01-02 00:00:00"},
			{"2013-02-28 23:59:00", "2013-03-01 00:00:00"},
			{"2016-02-28 23:59:00", "2016-02-29 00:00:00"},
			{"2012-12-31 23:59:00", "2013-01-01 00:00:00"},
		},
	},

	// Minutes with interval
	{
		"17-43/5 * * * *",
		"2006-01-02 15:04:05",
		[]crontimes{
			{"2013-01-01 00:00:00", "2013-01-01 00:17:00"},
			{"2013-01-01 00:16:59", "2013-01-01 00:17:00"},
			{"2013-01-01 00:30:00", "2013-01-01 00:32:00"},
			{"2013-01-01 00:50:00", "2013-01-01 01:17:00"},
			{"2013-01-01 23:50:00", "2013-01-02 00:17:00"},
			{"2013-02-28 23:50:00", "2013-03-01 00:17:00"},
			{"2016-02-28 23:50:00", "2016-02-29 00:17:00"},
			{"2012-12-31 23:50:00", "2013-01-01 00:17:00"},
		},
	},

	// Minutes interval, list
	{
		"15-30/4,55 * * * *",
		"2006-01-02 15:04:05",
		[]crontimes{
			{"2013-01-01 00:00:00", "2013-01-01 00:15:00"},
			{"2013-01-01 00:16:00", "2013-01-01 00:19:00"},
			{"2013-01-01 00:30:00", "2013-01-01 00:55:00"},
			{"2013-01-01 00:55:00", "2013-01-01 01:15:00"},
			{"2013-01-01 23:55:00", "2013-01-02 00:15:00"},
			{"2013-02-28 23:55:00", "2013-03-01 00:15:00"},
			{"2016-02-28 23:55:00", "2016-02-29 00:15:00"},
			{"2012-12-31 23:54:00", "2012-12-31 23:55:00"},
			{"2012-12-31 23:55:00", "2013-01-01 00:15:00"},
		},
	},

	// Days of week
	{
		"0 0 * * MON",
		"Mon 2006-01-02 15:04",
		[]crontimes{
			{"2013-01-01 00:00:00", "Mon 2013-01-07 00:00"},
			{"2013-01-28 00:00:00", "Mon 2013-02-04 00:00"},
			{"2013-12-30 00:30:00", "Mon 2014-01-06 00:00"},
		},
	},

	// Specific weekdays of week
	{
		"0 0 * * 6#5",
		"Mon 2006-01-02 15:04",
		[]crontimes{
			{"2013-09-02 00:00:00", "Sat 2013-11-30 00:00"},
		},
	},

	// TODO: more tests
}

func TestExpressions(t *testing.T) {
	for _, test := range crontests {
		for _, times := range test.times {
			from, _ := time.Parse("2006-01-02 15:04:05", times.from)
			next := cronexpr.MustParse(test.expr).Next(from)
			nextstr := next.Format(test.layout)
			if nextstr != times.next {
				t.Errorf(`("%s").Next("%s") = "%s", got "%s"`, test.expr, times.from, times.next, nextstr)
			}
		}
	}
}

func TestZero(t *testing.T) {
	from, _ := time.Parse("2006-01-02", "2013-08-31")
	next := cronexpr.MustParse("* * * * * 1980").Next(from)
	if next.IsZero() == false {
		t.Error(`("* * * * * 1980").Next("2013-08-31").IsZero() returned 'false', expected 'true'`)
	}

	next = cronexpr.MustParse("* * * * * 2050").Next(from)
	if next.IsZero() == true {
		t.Error(`("* * * * * 2050").Next("2013-08-31").IsZero() returned 'true', expected 'false'`)
	}

	next = cronexpr.MustParse("* * * * * 2099").Next(time.Time{})
	if next.IsZero() == false {
		t.Error(`("* * * * * 2014").Next(time.Time{}).IsZero() returned 'true', expected 'false'`)
	}
}

func TestNextN(t *testing.T) {
	expected := []string{
		"Sat, 30 Nov 2013 00:00:00",
		"Sat, 29 Mar 2014 00:00:00",
		"Sat, 31 May 2014 00:00:00",
		"Sat, 30 Aug 2014 00:00:00",
		"Sat, 29 Nov 2014 00:00:00",
	}
	from, _ := time.Parse("2006-01-02 15:04:05", "2013-09-02 08:44:30")
	result := cronexpr.MustParse("0 0 * * 6#5").NextN(from, uint(len(expected)))
	if len(result) != len(expected) {
		t.Errorf(`MustParse("0 0 * * 6#5").NextN("2013-09-02 08:44:30", 5):\n"`)
		t.Errorf(`  Expected %d returned time values but got %d instead`, len(expected), len(result))
	}
	for i, next := range result {
		nextStr := next.Format("Mon, 2 Jan 2006 15:04:15")
		if nextStr != expected[i] {
			t.Errorf(`MustParse("0 0 * * 6#5").NextN("2013-09-02 08:44:30", 5):\n"`)
			t.Errorf(`  result[%d]: expected "%s" but got "%s"`, i, expected[i], nextStr)
		}
	}
}
