package crontab

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var parseCrontabTestCases = []struct {
	crontab  string
	expected *Crontab
}{
	// Success cases
	{
		"",
		&Crontab{
			Context: &Context{
				Shell:   "/bin/sh",
				Environ: map[string]string{},
			},
			Jobs: []*Job{},
		},
	},

	{
		"\n",
		&Crontab{
			Context: &Context{
				Shell:   "/bin/sh",
				Environ: map[string]string{},
			},
			Jobs: []*Job{},
		},
	},

	{
		"FOO=bar\n",
		&Crontab{
			Context: &Context{
				Shell:   "/bin/sh",
				Environ: map[string]string{"FOO": "bar"},
			},
			Jobs: []*Job{},
		},
	},

	{
		"FOO=bar",
		&Crontab{
			Context: &Context{
				Shell:   "/bin/sh",
				Environ: map[string]string{"FOO": "bar"},
			},
			Jobs: []*Job{},
		},
	},

	{
		"FOO=\"bar\"",
		&Crontab{
			Context: &Context{
				Shell:   "/bin/sh",
				Environ: map[string]string{"FOO": "bar"},
			},
			Jobs: []*Job{},
		},
	},

	{
		"FOO='bar'",
		&Crontab{
			Context: &Context{
				Shell:   "/bin/sh",
				Environ: map[string]string{"FOO": "bar"},
			},
			Jobs: []*Job{},
		},
	},

	{
		"FOO='",
		&Crontab{
			Context: &Context{
				Shell:   "/bin/sh",
				Environ: map[string]string{"FOO": "'"},
			},
			Jobs: []*Job{},
		},
	},

	{
		"FOO=''",
		&Crontab{
			Context: &Context{
				Shell:   "/bin/sh",
				Environ: map[string]string{"FOO": ""},
			},
			Jobs: []*Job{},
		},
	},

	{
		"* * * * * foo some # qux",
		&Crontab{
			Context: &Context{
				Shell:   "/bin/sh",
				Environ: map[string]string{},
			},
			Jobs: []*Job{
				{
					CrontabLine: CrontabLine{
						Schedule: "* * * * *",
						Command:  "foo some # qux",
					},
				},
			},
		},
	},

	{
		"* * * * * foo\nSHELL=some\n1 1 1 1 1 bar\nKEY=VAL",
		&Crontab{
			Context: &Context{
				Shell: "some",
				Environ: map[string]string{
					"SHELL": "some",
					"KEY":   "VAL",
				},
			},
			Jobs: []*Job{
				{
					CrontabLine: CrontabLine{
						Schedule: "* * * * *",
						Command:  "foo",
					},
				},
				{
					CrontabLine: CrontabLine{
						Schedule: "1 1 1 1 1",
						Command:  "bar",
					},
				},
			},
		},
	},

	{
		"* * * * * * with year\n* * * * * * * with seconds\n@daily with shorthand",
		&Crontab{
			Context: &Context{
				Shell:   "/bin/sh",
				Environ: map[string]string{},
			},
			Jobs: []*Job{
				{
					CrontabLine: CrontabLine{
						Schedule: "* * * * * *",
						Command:  "with year",
					},
				},
				{
					CrontabLine: CrontabLine{
						Schedule: "* * * * * * *",
						Command:  "with seconds",
					},
				},
				{
					CrontabLine: CrontabLine{
						Schedule: "@daily",
						Command:  "with shorthand",
					},
				},
			},
		},
	},

	{
		"# * * * * * * commented\n\n\n  # some\n\t\n\t# more\n  \t  */2 * * * * will run",
		&Crontab{
			Context: &Context{
				Shell:   "/bin/sh",
				Environ: map[string]string{},
			},
			Jobs: []*Job{
				{
					CrontabLine: CrontabLine{
						Schedule: "*/2 * * * *",
						Command:  "will run",
					},
				},
			},
		},
	},

	{
		"* * * * *        \twith plenty of whitespace",
		&Crontab{
			Context: &Context{
				Shell:   "/bin/sh",
				Environ: map[string]string{},
			},
			Jobs: []*Job{
				{
					CrontabLine: CrontabLine{
						Schedule: "* * * * *",
						Command:  "with plenty of whitespace",
					},
				},
			},
		},
	},

	{
		"*\t*\t*\t*\t*\ttabs everywhere\n",
		&Crontab{
			Context: &Context{
				Shell:   "/bin/sh",
				Environ: map[string]string{},
			},
			Jobs: []*Job{
				{
					CrontabLine: CrontabLine{
						Schedule: "*\t*\t*\t*\t*",
						Command:  "tabs everywhere",
					},
				},
			},
		},
	},

	{
		"@hourly foo1 foo2 foo3 foo4 foo5 foo6",
		&Crontab{
			Context: &Context{
				Shell:   "/bin/sh",
				Environ: map[string]string{},
			},
			Jobs: []*Job{
				{
					CrontabLine: CrontabLine{
						Schedule: "@hourly",
						Command:  "foo1 foo2 foo3 foo4 foo5 foo6",
					},
				},
			},
		},
	},

	{
		"* * * * * echo \" whitespaces   preservation    \"",
		&Crontab{
			Context: &Context{
				Shell:   "/bin/sh",
				Environ: map[string]string{},
			},
			Jobs: []*Job{
				{
					CrontabLine: CrontabLine{
						Schedule: "* * * * *",
						Command:  "echo \" whitespaces   preservation    \"",
					},
				},
			},
		},
	},

	// Failure cases
	{"* foo \n", nil},
	{"* some * * *  more\n", nil},
	{"* some * * *  \n", nil},
	{"FOO\n", nil},
}

func TestParseCrontab(t *testing.T) {
	for _, tt := range parseCrontabTestCases {
		label := fmt.Sprintf("ParseCrontab(%q)", tt.crontab)

		reader := bytes.NewBufferString(tt.crontab)

		crontab, err := ParseCrontab(reader)

		if tt.expected == nil {
			assert.Nil(t, crontab, label)
			assert.NotNil(t, err, label)
		} else {
			if assert.NotNil(t, crontab, label) {
				assert.Equal(t, tt.expected.Context, crontab.Context, label)

				if assert.Equal(t, len(tt.expected.Jobs), len(crontab.Jobs), label) {
					for i, crontabJob := range crontab.Jobs {
						expectedJob := tt.expected.Jobs[i]
						assert.Equal(t, expectedJob.Command, crontabJob.Command, label)
						assert.Equal(t, expectedJob.Schedule, crontabJob.Schedule, label)
						assert.NotNil(t, crontabJob.Expression, label)
					}
				}
			}
		}
	}
}
