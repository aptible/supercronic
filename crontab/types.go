package crontab

import (
	"github.com/gorhill/cronexpr"
)

type crontabLine struct {
	Expression *cronexpr.Expression
	Schedule string
	Command  string
}

type Job struct {
	crontabLine
	Position int
}

type Context struct {
	Shell string
	Environ map[string]string
}

type Crontab struct {
	Jobs []*Job
	Context *Context
}
