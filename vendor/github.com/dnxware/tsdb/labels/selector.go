// Copyright 2017 The dnxware Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package labels

import (
	"fmt"
	"regexp"
)

// Selector holds constraints for matching against a label set.
type Selector []Matcher

// Matches returns whether the labels satisfy all matchers.
func (s Selector) Matches(labels Labels) bool {
	for _, m := range s {
		if v := labels.Get(m.Name()); !m.Matches(v) {
			return false
		}
	}
	return true
}

// Matcher specifies a constraint for the value of a label.
type Matcher interface {
	// Name returns the label name the matcher should apply to.
	Name() string
	// Matches checks whether a value fulfills the constraints.
	Matches(v string) bool
	// String returns a human readable matcher.
	String() string
}

// EqualMatcher matches on equality.
type EqualMatcher struct {
	name, value string
}

// Name implements Matcher interface.
func (m EqualMatcher) Name() string { return m.name }

// Matches implements Matcher interface.
func (m EqualMatcher) Matches(v string) bool { return v == m.value }

// String implements Matcher interface.
func (m EqualMatcher) String() string { return fmt.Sprintf("%s=%q", m.name, m.value) }

// Value returns the matched value.
func (m EqualMatcher) Value() string { return m.value }

// NewEqualMatcher returns a new matcher matching an exact label value.
func NewEqualMatcher(name, value string) Matcher {
	return &EqualMatcher{name: name, value: value}
}

type regexpMatcher struct {
	name string
	re   *regexp.Regexp
}

func (m regexpMatcher) Name() string          { return m.name }
func (m regexpMatcher) Matches(v string) bool { return m.re.MatchString(v) }
func (m regexpMatcher) String() string        { return fmt.Sprintf("%s=~%q", m.name, m.re.String()) }

// NewRegexpMatcher returns a new matcher verifying that a value matches
// the regular expression pattern.
func NewRegexpMatcher(name, pattern string) (Matcher, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &regexpMatcher{name: name, re: re}, nil
}

// NewMustRegexpMatcher returns a new matcher verifying that a value matches
// the regular expression pattern. Will panic if the pattern is not a valid
// regular expression.
func NewMustRegexpMatcher(name, pattern string) Matcher {
	re, err := regexp.Compile(pattern)
	if err != nil {
		panic(err)
	}
	return &regexpMatcher{name: name, re: re}

}

// NotMatcher inverts the matching result for a matcher.
type NotMatcher struct {
	Matcher
}

func (m NotMatcher) Matches(v string) bool { return !m.Matcher.Matches(v) }
func (m NotMatcher) String() string        { return fmt.Sprintf("not(%s)", m.Matcher.String()) }

// Not inverts the matcher's matching result.
func Not(m Matcher) Matcher {
	return &NotMatcher{m}
}
